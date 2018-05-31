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

	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/extensions/pg"
	"github.com/topfreegames/extensions/redis"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
)

// GetRedisClient returns a redis client
func GetRedisClient(logger logrus.FieldLogger, config *viper.Viper) (*redis.Client, error) {
	l := logger.WithFields(logrus.Fields{
		"operation": "configureRedisClient",
	})

	l.Debug("connecting to Redis...")
	client, err := redis.NewClient("extensions.redis", config)
	if err != nil {
		l.WithError(err).Error("connection to redis failed")
		return nil, err
	}
	l.Info("successfully connected to redis.")
	return client, nil
}

// GetDB returns a postgres client
func GetDB(logger logrus.FieldLogger, config *viper.Viper) (pginterfaces.DB, error) {
	l := logger.WithFields(logrus.Fields{
		"operation": "configureDatabase",
		"host":      config.GetString("extensions.pg.host"),
		"port":      config.GetString("extensions.pg.port"),
	})
	l.Debug("connecting to DB...")
	client, err := pg.NewClient("extensions.pg", config, nil, nil)
	if err != nil {
		l.WithError(err).Error("connection to database failed")
		return nil, err
	}
	l.Info("successfully connected to database")
	return client.DB, nil
}

// GetKubernetesClient returns a Kubernetes client
func GetKubernetesClient(logger logrus.FieldLogger, inCluster bool, kubeConfigPath string) (kubernetes.Interface, error) {
	var err error
	l := logger.WithFields(logrus.Fields{
		"operation": "configureKubernetesClient",
	})
	var config *rest.Config
	if inCluster {
		l.Debug("starting with incluster configuration")
		config, err = rest.InClusterConfig()
	} else {
		l.Debug("starting outside Kubernetes cluster")
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	}
	if err != nil {
		l.WithError(err).Error("start Kubernetes failed")
		return nil, err
	}
	l.Debug("connecting to Kubernetes...")
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		l.WithError(err).Error("connection to Kubernetes failed")
		return nil, err
	}
	l.Info("successfully connected to Kubernetes")
	return clientset, nil
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
