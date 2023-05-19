// maestro
//go:build unit
// +build unit

// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package watcher_test

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/viper"
	"github.com/topfreegames/extensions/redis"
	"k8s.io/client-go/kubernetes/fake"
	metricsFake "k8s.io/metrics/pkg/client/clientset/versioned/fake"

	"testing"

	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
	eventforwardermock "github.com/topfreegames/maestro/eventforwarder/mock"
	"github.com/topfreegames/maestro/mocks"
	"github.com/topfreegames/maestro/models"
	storagemock "github.com/topfreegames/maestro/storage/mock"
	mtesting "github.com/topfreegames/maestro/testing"
)

var (
	clientset             *fake.Clientset
	metricsClientset      *metricsFake.Clientset
	config                *viper.Viper
	hook                  *test.Hook
	logger                *logrus.Logger
	roomManager           models.RoomManager
	mockCtrl              *gomock.Controller
	mockDb                *pgmocks.MockDB
	mockPortChooser       *mocks.MockPortChooser
	mockPipeline          *redismocks.MockPipeliner
	mockRedisClient       *redismocks.MockRedisClient
	mockRedisTraceWrapper *redismocks.MockTraceWrapper
	mockEventForwarder    *eventforwardermock.MockEventForwarder
	mr                    *models.MixedMetricsReporter
	redisClient           *redis.Client
	eventsStorage         *storagemock.MockSchedulerEventStorage
	allStatus             = []string{
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

func TestWatcher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Watcher Suite")
}

var _ = BeforeEach(func() {
	var err error
	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel

	fakeReporter := mtesting.FakeMetricsReporter{}
	mr = models.NewMixedMetricsReporter()
	mr.AddReporter(fakeReporter)
	clientset = fake.NewSimpleClientset()
	metricsClientset = metricsFake.NewSimpleClientset()
	mockCtrl = gomock.NewController(GinkgoT())
	mockPortChooser = mocks.NewMockPortChooser(mockCtrl)
	mockDb = pgmocks.NewMockDB(mockCtrl)
	mockRedisClient = redismocks.NewMockRedisClient(mockCtrl)
	mockPipeline = redismocks.NewMockPipeliner(mockCtrl)
	mockEventForwarder = eventforwardermock.NewMockEventForwarder(mockCtrl)
	config, err = mtesting.GetDefaultConfig()
	Expect(err).NotTo(HaveOccurred())
	mockRedisClient.EXPECT().Ping()
	mockRedisTraceWrapper = redismocks.NewMockTraceWrapper(mockCtrl)
	redisClient, err = redis.NewClient("extensions.redis", config, mockRedisClient, mockRedisTraceWrapper)
	Expect(err).NotTo(HaveOccurred())
	roomManager = &models.GameRoom{}
	eventsStorage = storagemock.NewMockSchedulerEventStorage(mockCtrl)
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
