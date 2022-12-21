// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package extensions

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/extensions/v9/pg"
	"github.com/topfreegames/extensions/v9/redis"
	kubernetesExtensions "github.com/topfreegames/go-extensions-k8s-client-go/kubernetes"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsClient "k8s.io/metrics/pkg/client/clientset/versioned"

	pginterfaces "github.com/topfreegames/extensions/v9/pg/interfaces"
)

// GetRedisClient returns a redis client
func GetRedisClient(logger logrus.FieldLogger, config *viper.Viper, ifaces ...interface{}) (*redis.Client, error) {
	l := logger.WithFields(logrus.Fields{
		"operation": "configureRedisClient",
	})

	l.Debug("connecting to Redis...")
	client, err := redis.NewClient("extensions.redis", config, ifaces...)
	if err != nil {
		l.WithError(err).Error("connection to redis failed")
		return nil, err
	}
	l.Info("successfully connected to redis.")
	return client, nil
}

// GetDB returns a postgres client
func GetDB(logger logrus.FieldLogger, config *viper.Viper, dbOrNil pginterfaces.DB, dbCtxOrNil pginterfaces.CtxWrapper) (*pg.Client, error) {
	l := logger.WithFields(logrus.Fields{
		"operation": "configureDatabase",
		"host":      config.GetString("extensions.pg.host"),
		"port":      config.GetString("extensions.pg.port"),
	})
	l.Debug("connecting to DB...")
	client, err := pg.NewClient("extensions.pg", config, dbOrNil, nil, dbCtxOrNil)
	if err != nil {
		l.WithError(err).Error("connection to database failed")
		return nil, err
	}
	if dbOrNil != nil {
		client.DB = dbOrNil
	}
	l.Info("successfully connected to database")
	return client, nil
}

// GetKubernetesClient returns a Kubernetes client
func GetKubernetesClient(logger logrus.FieldLogger, config *viper.Viper, inCluster bool, kubeConfigPath string) (kubernetes.Interface, metricsClient.Interface, error) {
	var err error
	l := logger.WithFields(logrus.Fields{
		"operation": "configureKubernetesClient",
	})
	var conf *rest.Config
	if inCluster {
		l.Debug("starting with incluster configuration")
		conf, err = rest.InClusterConfig()
	} else {
		l.Debug("starting outside Kubernetes cluster")
		conf, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	}
	conf.Timeout = config.GetDuration("extensions.kubernetesClient.timeout")
	conf.RateLimiter = nil
	conf.Burst = config.GetInt("extensions.kubernetesClient.burst")
	conf.QPS = float32(config.GetInt("extensions.kubernetesClient.qps"))
	if err != nil {
		l.WithError(err).Error("start Kubernetes failed")
		return nil, nil, err
	}
	l.Debug("connecting to Kubernetes...")
	clientset, err := kubernetesExtensions.NewForConfig(conf)
	if err != nil {
		l.WithError(err).Error("connection to Kubernetes failed")
		return nil, nil, err
	}
	metricsClientset, err := metricsClient.NewForConfig(conf)
	if err != nil {
		l.WithError(err).Error("connection to Kubernetes Metrics Server failed")
		return nil, nil, err
	}

	l.Info("successfully connected to Kubernetes")
	return clientset, metricsClientset, nil
}

// WaitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
// got from http://stackoverflow.com/a/32843750/3987733
func WaitTimeout(wg *sync.WaitGroup, timeout time.Duration, log logrus.FieldLogger) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		log.Info("waiting for WaitGroup")
		wg.Wait()
		log.Info("finished waiting for WaitGroup")
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

// GracefulShutdown waits for wg do complete then exits
func GracefulShutdown(logger logrus.FieldLogger, wg *sync.WaitGroup, timeout time.Duration) {
	if wg != nil {
		logger.Info("waiting for graceful shutdown...")
		e := WaitTimeout(wg, timeout, logger)
		if e {
			logger.Warn("exited because of graceful shutdown timeout")
		} else {
			logger.Info("exited gracefully")
		}
	}
}
