package reporters_test

import (
	"github.com/golang/mock/gomock"
	"github.com/topfreegames/maestro/models/reporters"
	"github.com/topfreegames/maestro/models/reporters/mock_reporters"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Reporters", func() {
	It("Reporters.Report() must call Report on all children", func() {
		mockCtrl := gomock.NewController(GinkgoT())
		defer mockCtrl.Finish()

		rs := []reporters.Reporter{}
		for i := 0; i < 3; i++ {
			mr := mock_reporters.NewMockReporter(mockCtrl)
			rs = append(rs, mr)
			mr.EXPECT().Report()
		}

		r := reporters.Reporters{Reporters: rs}
		r.Report()
	})
})
