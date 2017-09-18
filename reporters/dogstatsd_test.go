// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package reporters_test

import (
	"github.com/topfreegames/extensions/dogstatsd/mocks"
	"github.com/topfreegames/maestro/reporters"
	handlers "github.com/topfreegames/maestro/reporters/dogstatsd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DogStatsD", func() {
	var (
		c    *mocks.ClientMock
		opts map[string]string
	)

	BeforeEach(func() {
		c = mocks.NewClientMock()
		opts = map[string]string{"game": "pong"}
	})

	It("GruIncrHandler should Incr event metric by 1", func() {
		Expect(c.Counts["gru.new"]).To(Equal(int64(0)))
		handlers.GruIncrHandler(c, "gru.new", opts)
		Expect(c.Counts["gru.new"]).To(Equal(int64(1)))
	})

	It("Report(gru.new, opts) should Incr gru.new", func() {
		d := reporters.NewDogStatsDFromClient(c)
		Expect(c.Counts["gru.new"]).To(Equal(int64(0)))
		err := d.Report("gru.new", opts)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Counts["gru.new"]).To(Equal(int64(1)))
	})
})
