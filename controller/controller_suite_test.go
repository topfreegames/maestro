// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package controller_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
	"github.com/topfreegames/maestro/models"

	mtesting "github.com/topfreegames/maestro/testing"
)

var (
	db          *pgmocks.PGMock
	logger      *logrus.Logger
	hook        *test.Hook
	err         error
	mr          *models.MixedMetricsReporter
	redisClient *redismocks.RedisMock
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeEach(func() {
	db = pgmocks.NewPGMock(1, 1)
	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel
	fakeReporter := mtesting.FakeMetricsReporter{}
	mr := models.NewMixedMetricsReporter()
	mr.AddReporter(fakeReporter)
	redisClient = redismocks.NewRedisMock("PONG")
})

var _ = AfterEach(func() {
	defer db.Close()
})
