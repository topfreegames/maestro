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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes/fake"

	"testing"

	"github.com/topfreegames/extensions/pg/mocks"
	"github.com/topfreegames/maestro/api"
	mtesting "github.com/topfreegames/maestro/testing"
)

var (
	app       *api.App
	config    *viper.Viper
	db        *mocks.PGMock
	err       error
	hook      *test.Hook
	logger    *logrus.Logger
	clientset *fake.Clientset
)

func TestApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Api Suite")
}

var _ = BeforeEach(func() {
	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel
	clientset = fake.NewSimpleClientset()

	config, err = mtesting.GetDefaultConfig()
	db = mocks.NewPGMock(1, 1)
	app, err = api.NewApp("0.0.0.0", 9998, config, logger, false, "", db, clientset)
	Expect(err).NotTo(HaveOccurred())
})
