// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"github.com/go-redis/redis"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	clockmocks "github.com/topfreegames/extensions/clock/mocks"
	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
	eventforwardermock "github.com/topfreegames/maestro/eventforwarder/mock"
	mtesting "github.com/topfreegames/maestro/testing"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/api"
	"github.com/topfreegames/maestro/login/mocks"
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	app                   *api.App
	clientset             *fake.Clientset
	config                *viper.Viper
	hook                  *test.Hook
	logger                *logrus.Logger
	mockCtrl              *gomock.Controller
	mockDb                *pgmocks.MockDB
	mockCtxWrapper        *pgmocks.MockCtxWrapper
	mockPipeline          *redismocks.MockPipeliner
	mockRedisClient       *redismocks.MockRedisClient
	mockRedisTraceWrapper *redismocks.MockTraceWrapper
	mockClientset         *fake.Clientset
	mockEventForwarder1   *eventforwardermock.MockEventForwarder
	mockEventForwarder2   *eventforwardermock.MockEventForwarder
	mockEventForwarder3   *eventforwardermock.MockEventForwarder
	mockEventForwarder4   *eventforwardermock.MockEventForwarder
	mockEventForwarder5   *eventforwardermock.MockEventForwarder
	mockLogin             *mocks.MockLogin
	mockClock             *clockmocks.MockClock
	mmr                   *models.MixedMetricsReporter
	allStatus             = []string{
		models.StatusCreating,
		models.StatusReady,
		models.StatusOccupied,
		models.StatusTerminating,
		models.StatusTerminated,
	}
	lockTimeoutMs int
	lockKey       string
)

func TestApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Api Suite")
}

var _ = BeforeEach(func() {
	var err error
	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel

	clientset = fake.NewSimpleClientset()
	mockCtrl = gomock.NewController(GinkgoT())
	mockDb = pgmocks.NewMockDB(mockCtrl)
	mockCtxWrapper = pgmocks.NewMockCtxWrapper(mockCtrl)
	mockRedisClient = redismocks.NewMockRedisClient(mockCtrl)
	mockRedisTraceWrapper = redismocks.NewMockTraceWrapper(mockCtrl)
	mockEventForwarder1 = eventforwardermock.NewMockEventForwarder(mockCtrl)
	mockEventForwarder2 = eventforwardermock.NewMockEventForwarder(mockCtrl)
	mockEventForwarder3 = eventforwardermock.NewMockEventForwarder(mockCtrl)
	mockEventForwarder4 = eventforwardermock.NewMockEventForwarder(mockCtrl)
	mockEventForwarder5 = eventforwardermock.NewMockEventForwarder(mockCtrl)
	mockPipeline = redismocks.NewMockPipeliner(mockCtrl)
	mockClientset = fake.NewSimpleClientset()

	fakeReporter := mtesting.FakeMetricsReporter{}
	mmr = models.NewMixedMetricsReporter()
	mmr.AddReporter(fakeReporter)

	config, err = mtesting.GetDefaultConfig()

	lockTimeoutMs = config.GetInt("watcher.lockTimeoutMs")
	lockKey = config.GetString("watcher.lockKey")

	mockRedisClient.EXPECT().Ping().Return(redis.NewStatusResult("PONG", nil)).AnyTimes()
	app, err = api.NewApp("0.0.0.0", 9998, config, logger, false, false, "", mockDb, mockCtxWrapper, mockRedisClient, mockRedisTraceWrapper, clientset)
	Expect(err).NotTo(HaveOccurred())

	mockLogin = mocks.NewMockLogin(mockCtrl)
	app.Login = mockLogin
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
