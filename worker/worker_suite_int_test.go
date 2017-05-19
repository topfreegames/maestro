// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package worker_test

import (
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	"github.com/topfreegames/maestro/api"
	"github.com/topfreegames/maestro/models"
	mtesting "github.com/topfreegames/maestro/testing"
	"github.com/topfreegames/maestro/worker"
	"k8s.io/client-go/kubernetes"
)

var (
	clientset kubernetes.Interface
	config    *viper.Viper
	hook      *test.Hook
	logger    *logrus.Logger
	mr        *models.MixedMetricsReporter
	app       *api.App
	appRedis  redisinterfaces.RedisClient
	w         *worker.Worker
)

func TestIntWorker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Worker Integration Suite")
}

var _ = BeforeSuite(func() {
	var err error
	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel

	fakeReporter := mtesting.FakeMetricsReporter{}
	mr = models.NewMixedMetricsReporter()
	mr.AddReporter(fakeReporter)

	minikubeConfig, err := mtesting.MinikubeConfig()
	Expect(err).NotTo(HaveOccurred())
	clientset, err = kubernetes.NewForConfig(minikubeConfig)
	Expect(err).NotTo(HaveOccurred())

	config, err = mtesting.GetDefaultConfig()
	Expect(err).NotTo(HaveOccurred())
	config.Set("worker.syncPeriod", 10)
	config.Set("pingTimeout", 600)

	app, err = api.NewApp("0.0.0.0", 9998, config, logger, false, "", nil, nil, clientset)
	Expect(err).NotTo(HaveOccurred())

	w, err = worker.NewWorker(config, logger, mr, false, "", app.DB, app.RedisClient, clientset)
	Expect(err).NotTo(HaveOccurred())
	go w.Start()
})

var _ = AfterEach(func() {
	_, err := app.DB.Exec("DELETE FROM schedulers")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	err := app.DB.Close()
	Expect(err).NotTo(HaveOccurred())

	tx := w.RedisClient.Client.TxPipeline()
	tx.Del(config.GetString("watcher.lockKey"))
	_, err = tx.Exec()
	Expect(err).NotTo(HaveOccurred())

	w.Run = false
})
