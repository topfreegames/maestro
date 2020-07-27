// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package auth_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/viper"
	loginMocks "github.com/topfreegames/maestro/login/mocks"
	mtesting "github.com/topfreegames/maestro/testing"

	"testing"

	"github.com/golang/mock/gomock"
	pgmocks "github.com/topfreegames/extensions/pg/mocks"
)

var (
	logger    *logrus.Logger
	hook      logrus.Hook
	mockCtrl  *gomock.Controller
	mockDb    *pgmocks.MockDB
	mockLogin *loginMocks.MockLogin
	config    *viper.Viper
)

func TestApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Api Auth Suite")
}

var _ = BeforeEach(func() {
	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel
	mockCtrl = gomock.NewController(GinkgoT())
	mockDb = pgmocks.NewMockDB(mockCtrl)
	mockLogin = loginMocks.NewMockLogin(mockCtrl)

	var err error
	config, err = mtesting.GetDefaultConfig()
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
