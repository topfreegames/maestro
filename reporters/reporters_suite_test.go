package reporters_test

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	"github.com/golang/mock/gomock"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/reporters"
	"github.com/topfreegames/maestro/reporters/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	mtesting "github.com/topfreegames/maestro/testing"

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
	config    *viper.Viper
	logger    *logrus.Logger
)

var _ = BeforeSuite(func() {
	singleton = reporters.GetInstance()
	mockCtrl = gomock.NewController(GinkgoT())
	config, _ = mtesting.GetDefaultConfig()
	logger, _ = test.NewNullLogger()
	logger.Level = logrus.DebugLevel

	for i := 0; i < 3; i++ {
		mr := mocks.NewMockReporter(mockCtrl)
		mrs = append(mrs, mr)
		singleton.SetReporter(fmt.Sprintf("Reporter-%d", i), mr)
	}
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
