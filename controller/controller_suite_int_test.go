// maestro
//go:build integration
// +build integration

// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package controller_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
	mtesting "github.com/topfreegames/maestro/testing"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/viper"
	"github.com/topfreegames/extensions/redis"
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes"
)

var (
	hook            *test.Hook
	logger          *logrus.Logger
	mockCtrl        *gomock.Controller
	config          *viper.Viper
	mockDb          *pgmocks.MockDB
	redisClient     *redis.Client
	mr              *models.MixedMetricsReporter
	mockPipeline    *redismocks.MockPipeliner
	mockRedisClient *redismocks.MockRedisClient
	clientset       kubernetes.Interface
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Integration Suite")
}

var _ = BeforeSuite(func() {
	var err error
	config, err = mtesting.GetDefaultConfig()
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	var err error
	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel

	fakeReporter := mtesting.FakeMetricsReporter{}
	mr = models.NewMixedMetricsReporter()
	mr.AddReporter(fakeReporter)

	mockCtrl = gomock.NewController(GinkgoT())

	mockDb = pgmocks.NewMockDB(mockCtrl)

	mockRedisClient = redismocks.NewMockRedisClient(mockCtrl)
	mockPipeline = redismocks.NewMockPipeliner(mockCtrl)

	mockRedisClient.EXPECT().Ping()
	redisClient, err = redis.NewClient("extensions.redis", config, mockRedisClient)
	Expect(err).NotTo(HaveOccurred())

	minikubeConfig, err := mtesting.MinikubeConfig()
	Expect(err).NotTo(HaveOccurred())
	clientset, err = kubernetes.NewForConfig(minikubeConfig)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
