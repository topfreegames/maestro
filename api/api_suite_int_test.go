// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	"github.com/topfreegames/maestro/api"
	"github.com/topfreegames/maestro/login"
	"github.com/topfreegames/maestro/login/mocks"
	"github.com/topfreegames/maestro/models"

	mTest "github.com/topfreegames/maestro/testing"
	"k8s.io/client-go/kubernetes"
)

var (
	clientset kubernetes.Interface
	app       *api.App
	hook      *test.Hook
	logger    *logrus.Logger
	config    *viper.Viper
	mockDb    *pgmocks.MockDB
	mockLogin *mocks.MockLogin
	mockCtrl  *gomock.Controller
	mmr       *models.MixedMetricsReporter
	token     = "token"
)

func TestIntModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Api Integration Suite")
}

var _ = BeforeSuite(func() {
	var err error
	minikubeConfig, err := mTest.MinikubeConfig()
	Expect(err).NotTo(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(minikubeConfig)
	Expect(err).NotTo(HaveOccurred())

	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel

	config, err = mTest.GetDefaultConfig()
	Expect(err).NotTo(HaveOccurred())

	app, err = api.NewApp("0.0.0.0", 9998, config, logger, false, false, "", nil, nil, clientset)
	Expect(err).NotTo(HaveOccurred())

	user := &login.User{
		KeyAccessToken: token,
		AccessToken:    token,
		RefreshToken:   token,
		Expiry:         time.Now().Add(1 * time.Hour),
		TokenType:      "Bearer",
		Email:          "user@example.com",
	}
	query := `INSERT INTO users(key_access_token, access_token, refresh_token, expiry, token_type, email) 
	VALUES(?key_access_token, ?access_token, ?refresh_token, ?expiry, ?token_type, ?email)
	ON CONFLICT (key_access_token) DO NOTHING`
	_, err = app.DB.Query(user, query, user)
	Expect(err).NotTo(HaveOccurred())

	mockCtrl = gomock.NewController(GinkgoT())
	mockLogin = mocks.NewMockLogin(mockCtrl)
	app.Login = mockLogin

	err = models.InitAvailablePorts(app.RedisClient, models.FreePortsRedisKey(), 40000, 60000)
	Expect(err).NotTo(HaveOccurred())
})
