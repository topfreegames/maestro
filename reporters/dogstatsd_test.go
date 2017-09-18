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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DogStatsD", func() {
	It("Reporters.Report() must call Report on all children", func() {
		c := mocks.NewClientMock()
		d := reporters.NewDogStatsDFromClient(c)
		opts := map[string]string{"game": "pong"}
		Expect(c.Counts["gru.new"]).To(Equal(int64(0)))
		err := d.Report("gru.new", opts)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Counts["gru.new"]).To(Equal(int64(1)))
	})
})
