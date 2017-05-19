// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/api"

	mTest "github.com/topfreegames/maestro/testing"
	"k8s.io/client-go/kubernetes"
)

var (
	clientset kubernetes.Interface
	app       *api.App
	hook      *test.Hook
	logger    *logrus.Logger
	config    *viper.Viper
)

func TestIntModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Models Integration Suite")
}

var _ = BeforeSuite(func() {
	var err error
	minikubeConfig, err := mTest.MinikubeConfig()
	Expect(err).NotTo(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(minikubeConfig)
	Expect(err).NotTo(HaveOccurred())

	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel

	config, err = mTest.GetDefaultConfig()
	Expect(err).NotTo(HaveOccurred())

	app, err = api.NewApp("0.0.0.0", 9998, config, logger, false, "", nil, nil, clientset)
	Expect(err).NotTo(HaveOccurred())
})
