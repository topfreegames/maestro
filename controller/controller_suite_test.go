// maestro
// +build unit
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
	"time"

	clockmocks "github.com/topfreegames/extensions/clock/mocks"
	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
	mtesting "github.com/topfreegames/maestro/testing"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/viper"
	"github.com/topfreegames/extensions/redis"
	"github.com/topfreegames/maestro/mocks"
	"github.com/topfreegames/maestro/models"
	metricsFake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

var (
	hook                   *test.Hook
	logger                 *logrus.Logger
	metricsClientset       *metricsFake.Clientset
	mockCtrl               *gomock.Controller
	config                 *viper.Viper
	mockDb                 *pgmocks.MockDB
	mockPipeline           *redismocks.MockPipeliner
	mockRedisClient        *redismocks.MockRedisClient
	mockRedisTraceWrapper  *redismocks.MockTraceWrapper
	mockClock              *clockmocks.MockClock
	mockPortChooser        *mocks.MockPortChooser
	redisClient            *redis.Client
	mr                     *models.MixedMetricsReporter
	schedulerCache         *models.SchedulerCache
	err                    error
	allStatus              = []string{
		models.StatusCreating,
		models.StatusReady,
		models.StatusOccupied,
		models.StatusTerminating,
		models.StatusTerminated,
	}
	allMetrics = []string{
		string(models.CPUAutoScalingPolicyType),
		string(models.MemAutoScalingPolicyType),
	}
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	config, err = mtesting.GetDefaultConfig()
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel

	fakeReporter := mtesting.FakeMetricsReporter{}
	mr := models.NewMixedMetricsReporter()
	mr.AddReporter(fakeReporter)

	metricsClientset = metricsFake.NewSimpleClientset()
	mockCtrl = gomock.NewController(GinkgoT())

	mockDb = pgmocks.NewMockDB(mockCtrl)

	mockRedisClient = redismocks.NewMockRedisClient(mockCtrl)
	mockRedisTraceWrapper = redismocks.NewMockTraceWrapper(mockCtrl)
	mockPipeline = redismocks.NewMockPipeliner(mockCtrl)

	mockPortChooser = mocks.NewMockPortChooser(mockCtrl)
	models.ThePortChooser = mockPortChooser

	mockRedisClient.EXPECT().Ping()
	redisClient, err = redis.NewClient("extensions.redis", config, mockRedisClient, mockRedisTraceWrapper)
	Expect(err).NotTo(HaveOccurred())

	mockClock = clockmocks.NewMockClock(mockCtrl)

	schedulerCache = models.NewSchedulerCache(1*time.Minute, 10*time.Minute, logger)
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
