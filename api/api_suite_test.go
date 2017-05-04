// maestro
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

	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
	"github.com/topfreegames/maestro/api"
	mtesting "github.com/topfreegames/maestro/testing"
)

var (
	app             *api.App
	clientset       *fake.Clientset
	config          *viper.Viper
	hook            *test.Hook
	logger          *logrus.Logger
	mockCtrl        *gomock.Controller
	mockDb          *pgmocks.MockDB
	mockPipeline    *redismocks.MockPipeliner
	mockRedisClient *redismocks.MockRedisClient
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
	mockPipeline = redismocks.NewMockPipeliner(mockCtrl)

	config, err = mtesting.GetDefaultConfig()
	app, err = api.NewApp("0.0.0.0", 9998, config, logger, false, "", mockDb, mockRedisClient, clientset)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
