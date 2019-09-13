// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package worker_test

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
	"github.com/topfreegames/maestro/models"
	mtesting "github.com/topfreegames/maestro/testing"
)

var (
	clientset        *fake.Clientset
	metricsClientset *metricsFake.Clientset
	config           *viper.Viper
	hook             *test.Hook
	logger           *logrus.Logger
	mockCtrl         *gomock.Controller
	mockDb           *pgmocks.MockDB
	mockPipeline     *redismocks.MockPipeliner
	mockRedisClient  *redismocks.MockRedisClient
	mr               *models.MixedMetricsReporter
	redisClient      *redis.Client
)

func TestWorker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Worker Suite")
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
	mockDb = pgmocks.NewMockDB(mockCtrl)
	mockRedisClient = redismocks.NewMockRedisClient(mockCtrl)
	mockPipeline = redismocks.NewMockPipeliner(mockCtrl)
	config, err = mtesting.GetDefaultConfig()
	Expect(err).NotTo(HaveOccurred())
	mockRedisClient.EXPECT().Ping()
	redisClient, err = redis.NewClient("extensions.redis", config, mockRedisClient)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
