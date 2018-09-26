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
	. "github.com/topfreegames/maestro/reporters/constants"
)

var _ = Describe("Reporters", func() {
	It("Reporters.Report() must call Report on all children", func() {
		opts := map[string]interface{}{"game": "pong"}

		for _, mr := range mrs {
			mr.EXPECT().Report(EventGruNew, opts)
		}

		singleton.Report(EventGruNew, opts)
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

		It("must delete an existing reporter from singleton with UnsetReporter", func() {
			r := reporters.NewReporters()
			reporters.MakeDogStatsD(config, logger, r)
			_, prs := r.GetReporter("dogstatsd")
			Expect(prs).To(Equal(true))
			r.UnsetReporter("dogstatsd")
			_, prs = r.GetReporter("dogstatsd")
			Expect(prs).To(Equal(false))
		})
	})
})
