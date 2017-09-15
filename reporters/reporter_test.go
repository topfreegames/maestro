// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package reporters_test

import (
	"github.com/topfreegames/maestro/reporters"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reporters", func() {
	It("Reporters.Report() must call Report on all children", func() {
		for _, mr := range mrs {
			mr.EXPECT().Report("report")
		}
		reporters.MakeDogStatsD(config, logger)

		singleton.Report("report")
	})

	It("Reporters must be Singleton", func() {
		Expect(singleton).To(Equal(reporters.GetInstance()))
	})

	Describe("MakeReporters", func() {
		It("must create a reporter for every key in config.reporters", func() {
			reporters.MakeReporters(config, logger)
			singleton := reporters.GetInstance()
			_, prs := singleton.GetReporter("dogstatsd")
			Expect(prs).To(Equal(true))
		})
	})
})
