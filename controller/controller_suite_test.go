// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package controller_test

import (
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"github.com/topfreegames/extensions/redis"

	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	clockmocks "github.com/topfreegames/extensions/clock/mocks"
	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
	"github.com/topfreegames/maestro/models"

	mtesting "github.com/topfreegames/maestro/testing"
)

var (
	hook            *test.Hook
	logger          *logrus.Logger
	mockCtrl        *gomock.Controller
	config          *viper.Viper
	mockDb          *pgmocks.MockDB
	mockPipeline    *redismocks.MockPipeliner
	mockRedisClient *redismocks.MockRedisClient
	mockClock       *clockmocks.MockClock
	redisClient     *redis.Client
	mr              *models.MixedMetricsReporter
	schedulerCache  *models.SchedulerCache
	allStatus       = []string{
		models.StatusCreating,
		models.StatusReady,
		models.StatusOccupied,
		models.StatusTerminating,
		models.StatusTerminated,
	}
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
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
	mr := models.NewMixedMetricsReporter()
	mr.AddReporter(fakeReporter)

	mockCtrl = gomock.NewController(GinkgoT())

	mockDb = pgmocks.NewMockDB(mockCtrl)

	mockRedisClient = redismocks.NewMockRedisClient(mockCtrl)
	mockPipeline = redismocks.NewMockPipeliner(mockCtrl)

	mockRedisClient.EXPECT().Ping()
	redisClient, err = redis.NewClient("extensions.redis", config, mockRedisClient)
	Expect(err).NotTo(HaveOccurred())

	mockClock = clockmocks.NewMockClock(mockCtrl)

	schedulerCache = models.NewSchedulerCache(1*time.Minute, 10*time.Minute, logger)
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
