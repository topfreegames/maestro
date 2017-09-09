// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package worker_test

import (
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/api"
	"github.com/topfreegames/maestro/login"
	"github.com/topfreegames/maestro/login/mocks"
	"github.com/topfreegames/maestro/models"
	mtesting "github.com/topfreegames/maestro/testing"
	"github.com/topfreegames/maestro/worker"
	"k8s.io/client-go/kubernetes"
)

var (
	clientset      kubernetes.Interface
	config         *viper.Viper
	hook           *test.Hook
	logger         *logrus.Logger
	mr             *models.MixedMetricsReporter
	app            *api.App
	w              *worker.Worker
	mockLogin      *mocks.MockLogin
	mockCtrl       *gomock.Controller
	token          string = "token"
	startPortRange        = 40000
	endPortRange          = 40099
)

func TestIntWorker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Worker Integration Suite")
}

var _ = BeforeSuite(func() {
	var err error
	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel

	fakeReporter := mtesting.FakeMetricsReporter{}
	mr = models.NewMixedMetricsReporter()
	mr.AddReporter(fakeReporter)

	minikubeConfig, err := mtesting.MinikubeConfig()
	Expect(err).NotTo(HaveOccurred())
	clientset, err = kubernetes.NewForConfig(minikubeConfig)
	Expect(err).NotTo(HaveOccurred())

	config, err = mtesting.GetDefaultConfig()
	Expect(err).NotTo(HaveOccurred())
	config.Set("worker.syncPeriod", 10)
	config.Set("pingTimeout", 600)
	config.Set("occupiedTimeout", 1)

	app, err = api.NewApp("0.0.0.0", 9998, config, logger, false, "", nil, nil, clientset)
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

	w, err = worker.NewWorker(config, logger, mr, false, "", app.DB, app.RedisClient, clientset)
	Expect(err).NotTo(HaveOccurred())
	go w.Start(startPortRange, endPortRange, false)
})

var _ = AfterEach(func() {
	_, err := app.DB.Exec("DELETE FROM schedulers")
	Expect(err).NotTo(HaveOccurred())

	pipe := app.RedisClient.TxPipeline()
	cmd := pipe.FlushAll()
	_, err = pipe.Exec()
	Expect(err).NotTo(HaveOccurred())
	err = cmd.Err()
	Expect(err).NotTo(HaveOccurred())

	err = models.InitAvailablePorts(app.RedisClient, startPortRange, endPortRange)
})

var _ = AfterSuite(func() {
	err := app.DB.Close()
	Expect(err).NotTo(HaveOccurred())

	tx := w.RedisClient.Client.TxPipeline()
	tx.Del(config.GetString("watcher.lockKey"))
	_, err = tx.Exec()
	Expect(err).NotTo(HaveOccurred())

	w.Run = false
})
