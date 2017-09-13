package reporters_test

import (
	"github.com/topfreegames/maestro/models/reporters"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reporters", func() {
	It("Reporters.Report() must call Report on all children", func() {
		for _, mr := range mrs {
			mr.EXPECT().Report("report")
		}

		singleton.Report("report")
	})

	It("Reporters must be Singleton", func() {
		Expect(singleton).To(Equal(reporters.GetInstance()))
	})
})
