// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"github.com/topfreegames/extensions/mocks"
)

var (
	db          *mocks.PGMock
	redisClient *mocks.RedisMock
	err         error
)

func TestModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Models Suite")
}

var _ = BeforeEach(func() {
	db = mocks.NewPGMock(1, 1)
	redisClient = mocks.NewRedisMock("PONG")
})

var _ = AfterEach(func() {
	defer db.Close()
})
