// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"testing"

	"github.com/topfreegames/extensions/redis"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/topfreegames/maestro/extensions"
	mtesting "github.com/topfreegames/maestro/testing"
	"k8s.io/client-go/kubernetes"
)

var (
	redisClient *redis.Client
	logger      *logrus.Logger
	hook        *test.Hook
	clientset   kubernetes.Interface
)

func TestIntModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Models Integration Suite")
}

var _ = BeforeSuite(func() {
	config, err := mtesting.GetDefaultConfig()
	Expect(err).NotTo(HaveOccurred())

	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel

	redisClient, err = extensions.GetRedisClient(logger, config)
	Expect(err).NotTo(HaveOccurred())

	kubeConfig, err := mtesting.MinikubeConfig()
	clientset, err = kubernetes.NewForConfig(kubeConfig)
})
