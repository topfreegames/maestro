// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/fake"

	"testing"

	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
)

var (
	mockCtrl        *gomock.Controller
	mockDb          *pgmocks.MockDB
	mockRedisClient *redismocks.MockRedisClient
	mockClientset   *fake.Clientset
	mockPipeline    *redismocks.MockPipeliner
)

func TestModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Models Suite")
}

var _ = BeforeEach(func() {
	mockCtrl = gomock.NewController(GinkgoT())
	mockDb = pgmocks.NewMockDB(mockCtrl)
	mockRedisClient = redismocks.NewMockRedisClient(mockCtrl)
	mockClientset = fake.NewSimpleClientset()
	mockPipeline = redismocks.NewMockPipeliner(mockCtrl)
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
