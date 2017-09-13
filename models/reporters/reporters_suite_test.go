package reporters_test

import (
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/topfreegames/maestro/models/reporters"
	"github.com/topfreegames/maestro/models/reporters/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestReporters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reporters Suite")
}

var (
	mockCtrl  *gomock.Controller
	singleton *reporters.Reporters
	mrs       []*mocks.MockReporter
)

var _ = BeforeSuite(func() {
	singleton = reporters.GetInstance()
	mockCtrl = gomock.NewController(GinkgoT())

	for i := 0; i < 3; i++ {
		mr := mocks.NewMockReporter(mockCtrl)
		mrs = append(mrs, mr)
		singleton.SetReporter(fmt.Sprintf("Reporter-%d", i), mr)
	}
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
