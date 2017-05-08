// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package extensions

import (
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
	})
	l.Debug("connecting to DB...")
	client, err := pg.NewClient("extensions.pg", config)
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
