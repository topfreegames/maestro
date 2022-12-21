// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"testing"

	pgmocks "github.com/topfreegames/extensions/v9/pg/mocks"
	redismocks "github.com/topfreegames/extensions/v9/redis/mocks"
	reportersmocks "github.com/topfreegames/maestro/reporters/mocks"
	mtesting "github.com/topfreegames/maestro/testing"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/topfreegames/maestro/mocks"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/reporters"
	"k8s.io/client-go/kubernetes/fake"
	metricsFake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

var (
	mockCtrl         *gomock.Controller
	mockDb           *pgmocks.MockDB
	metricsClientset *metricsFake.Clientset
	mockRedisClient  *redismocks.MockRedisClient
	config           *viper.Viper
	mockClientset    *fake.Clientset
	mockPipeline     *redismocks.MockPipeliner
	mockPortChooser  *mocks.MockPortChooser
	mmr              *models.MixedMetricsReporter
	mr               *reportersmocks.MockReporter
	singleton        *reporters.Reporters
	hook             *test.Hook
	logger           *logrus.Logger
)

func TestModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Models Suite")
}

var _ = BeforeEach(func() {
	mockCtrl = gomock.NewController(GinkgoT())
	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel
	mockDb = pgmocks.NewMockDB(mockCtrl)
	mockRedisClient = redismocks.NewMockRedisClient(mockCtrl)
	config, _ = mtesting.GetDefaultConfig()
	metricsClientset = metricsFake.NewSimpleClientset()
	mockClientset = fake.NewSimpleClientset()
	mockPipeline = redismocks.NewMockPipeliner(mockCtrl)

	mockPortChooser = mocks.NewMockPortChooser(mockCtrl)
	models.ThePortChooser = mockPortChooser

	mr = reportersmocks.NewMockReporter(mockCtrl)
	singleton = reporters.GetInstance()
	singleton.SetReporter("mockReporter", mr)
	fakeReporter := mtesting.FakeMetricsReporter{}
	mmr = models.NewMixedMetricsReporter()
	mmr.AddReporter(fakeReporter)
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
