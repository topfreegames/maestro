// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes/fake"

	"testing"

	clockmocks "github.com/topfreegames/extensions/clock/mocks"
	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
	"github.com/topfreegames/maestro/api"
	eventforwardermock "github.com/topfreegames/maestro/eventforwarder/mock"
	"github.com/topfreegames/maestro/login/mocks"
	"github.com/topfreegames/maestro/models"
	mtesting "github.com/topfreegames/maestro/testing"
)

var (
	app                 *api.App
	clientset           *fake.Clientset
	config              *viper.Viper
	hook                *test.Hook
	logger              *logrus.Logger
	mockCtrl            *gomock.Controller
	mockDb              *pgmocks.MockDB
	mockPipeline        *redismocks.MockPipeliner
	mockRedisClient     *redismocks.MockRedisClient
	mockEventForwarder1 *eventforwardermock.MockEventForwarder
	mockEventForwarder2 *eventforwardermock.MockEventForwarder
	mockLogin           *mocks.MockLogin
	mockClock           *clockmocks.MockClock
	allStatus           = []string{
		models.StatusCreating,
		models.StatusReady,
		models.StatusOccupied,
		models.StatusTerminating,
		models.StatusTerminated,
	}
	lockTimeoutMS int
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
	mockRedisClient = redismocks.NewMockRedisClient(mockCtrl)
	mockEventForwarder1 = eventforwardermock.NewMockEventForwarder(mockCtrl)
	mockEventForwarder2 = eventforwardermock.NewMockEventForwarder(mockCtrl)
	mockPipeline = redismocks.NewMockPipeliner(mockCtrl)

	config, err = mtesting.GetDefaultConfig()

	lockTimeoutMS = config.GetInt("watcher.lockTimeoutMs")
	lockKey = config.GetString("watcher.lockKey")

	app, err = api.NewApp("0.0.0.0", 9998, config, logger, false, "", mockDb, mockRedisClient, clientset)
	Expect(err).NotTo(HaveOccurred())

	mockLogin = mocks.NewMockLogin(mockCtrl)
	app.Login = mockLogin
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
