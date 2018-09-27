// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package reporters_test

import (
	"github.com/golang/mock/gomock"
	"github.com/topfreegames/extensions/dogstatsd/mocks"
	"github.com/topfreegames/maestro/reporters"
	handlers "github.com/topfreegames/maestro/reporters/dogstatsd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DogStatsD", func() {
	var (
		r    *reporters.Reporters
		c    *mocks.MockClient
		opts map[string]string
	)

	toMapStringInterface := func(o map[string]string) map[string]interface{} {
		n := map[string]interface{}{}
		for k, v := range o {
			n[k] = v
		}
		return n
	}

	BeforeEach(func() {
		r = reporters.NewReporters()
		c = mocks.NewMockClient(mockCtrl)
		opts = map[string]string{"game": "pong"}
	})

	It(`MakeDogStatsD should create a new DogStatsD
	instance and add it to the singleton reporters.Reporters`, func() {
		_, prs := r.GetReporter("dogstatsd")
		Expect(prs).To(BeFalse())
		reporters.MakeDogStatsD(config, logger, r)
		_, prs = r.GetReporter("dogstatsd")
		Expect(prs).To(BeTrue())
	})

	It("GruIncrHandler should Incr event metric by 1", func() {
		c.EXPECT().Incr("gru.new", []string{"game:pong"}, float64(1))
		handlers.GruIncrHandler(c, "gru.new", opts)
	})

	It("GruStatusHandler should send Gauge of given status", func() {
		var emptyArr []string
		c.EXPECT().Gauge("gru.terminating", float64(42), emptyArr, float64(1))

		opts["status"] = "terminating"
		opts["gauge"] = "42"
		handlers.GruStatusHandler(c, "gru.status", opts)
	})

	It("Report(gru.new, opts) should Incr gru.new", func() {
		c.EXPECT().Incr("gru.new", gomock.Any(), float64(1))
		d := reporters.NewDogStatsDFromClient(c, "test")
		err := d.Report("gru.new", toMapStringInterface(opts))
		Expect(err).NotTo(HaveOccurred())
	})

	It("Report(gru.status, opts) should send Gauge of given status", func() {
		c.EXPECT().Gauge("gru.creating", float64(5),
			[]string{"maestro-region:test"}, float64(1))

		d := reporters.NewDogStatsDFromClient(c, "test")
		opts["status"] = "creating"
		opts["gauge"] = "5"
		err := d.Report("gru.status", toMapStringInterface(opts))
		Expect(err).NotTo(HaveOccurred())
	})
})
