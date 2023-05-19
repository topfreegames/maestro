// maestro
//go:build unit
// +build unit

// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package william_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	mtesting "github.com/topfreegames/maestro/testing"

	"github.com/golang/mock/gomock"
	"github.com/spf13/viper"
)

var (
	config   *viper.Viper
	mockCtrl *gomock.Controller
	mockDb   *pgmocks.MockDB
)

func TestApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Api Suite")
}

var _ = BeforeEach(func() {
	mockCtrl = gomock.NewController(GinkgoT())
	mockDb = pgmocks.NewMockDB(mockCtrl)
	config, _ = mtesting.GetDefaultConfig()
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
