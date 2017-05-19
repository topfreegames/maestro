// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/testing"
)

func sendToChan(c chan bool) {
	c <- true
}

var _ = Describe("Mixed Metrics Reporter Model", func() {
	var testChan chan bool
	BeforeEach(func() {
		testChan = make(chan bool, 1)
	})

	Describe("Create New Mixed Metrics Reporter", func() {
		It("Should return a MixedMetrisReporterStruct with empty MetricsReporters", func() {
			mixedMetricsReporter := models.NewMixedMetricsReporter()
			Expect(mixedMetricsReporter.MetricsReporters).To(HaveLen(0))
		})
	})

	Describe("AddReporter", func() {
		It("Should return a MixedMetrisReporterStruct with empty MetricsReporters", func() {
			mr := testing.FakeMetricsReporter{}
			mixedMetricsReporter := models.NewMixedMetricsReporter()
			mixedMetricsReporter.AddReporter(mr)
			Expect(mixedMetricsReporter.MetricsReporters).To(HaveLen(1))
		})
	})

	Describe("WithSegment", func() {
		It("Should execute the given function if mr is nil", func() {
			var mixedMetricsReporter *models.MixedMetricsReporter
			f := func() error {
				sendToChan(testChan)
				return nil
			}
			err := mixedMetricsReporter.WithSegment("name", f)
			Expect(err).NotTo(HaveOccurred())
			Eventually(testChan).Should(Receive())
		})

		It("Should execute StartSegment and EndSegment and the given function if mr is not nil", func() {
			mr := testing.FakeMetricsReporter{}
			mixedMetricsReporter := models.NewMixedMetricsReporter()
			mixedMetricsReporter.AddReporter(mr)
			f := func() error {
				sendToChan(testChan)
				return nil
			}
			err := mixedMetricsReporter.WithSegment("name", f)
			Expect(err).NotTo(HaveOccurred())
			Eventually(testChan).Should(Receive())
		})
	})

	Describe("WithDatastoreSegment", func() {
		It("Should execute the given function if mr is nil", func() {
			var mixedMetricsReporter *models.MixedMetricsReporter
			f := func() error {
				sendToChan(testChan)
				return nil
			}
			err := mixedMetricsReporter.WithDatastoreSegment("table", "op", f)
			Expect(err).NotTo(HaveOccurred())
			Eventually(testChan).Should(Receive())
		})

		It("Should execute StartSegment and EndSegment and the given function if mr is not nil", func() {
			mr := testing.FakeMetricsReporter{}
			mixedMetricsReporter := models.NewMixedMetricsReporter()
			mixedMetricsReporter.AddReporter(mr)
			f := func() error {
				sendToChan(testChan)
				return nil
			}
			err := mixedMetricsReporter.WithDatastoreSegment("table", "op", f)
			Expect(err).NotTo(HaveOccurred())
			Eventually(testChan).Should(Receive())
		})
	})

	Describe("WithRedisSegment", func() {
		It("Should execute the given function if mr is nil", func() {
			var mixedMetricsReporter *models.MixedMetricsReporter
			f := func() error {
				sendToChan(testChan)
				return nil
			}
			err := mixedMetricsReporter.WithRedisSegment("op", f)
			Expect(err).NotTo(HaveOccurred())
			Eventually(testChan).Should(Receive())
		})

		It("Should execute StartSegment and EndSegment and the given function if mr is not nil", func() {
			mr := testing.FakeMetricsReporter{}
			mixedMetricsReporter := models.NewMixedMetricsReporter()
			mixedMetricsReporter.AddReporter(mr)
			f := func() error {
				sendToChan(testChan)
				return nil
			}
			err := mixedMetricsReporter.WithRedisSegment("op", f)
			Expect(err).NotTo(HaveOccurred())
			Eventually(testChan).Should(Receive())
		})
	})

	Describe("WithExternalSegment", func() {
		It("Should execute the given function if mr is nil", func() {
			var mixedMetricsReporter *models.MixedMetricsReporter
			f := func() error {
				sendToChan(testChan)
				return nil
			}
			err := mixedMetricsReporter.WithExternalSegment("url", f)
			Expect(err).NotTo(HaveOccurred())
			Eventually(testChan).Should(Receive())
		})

		It("Should execute StartSegment and EndSegment and the given function if mr is not nil", func() {
			mr := testing.FakeMetricsReporter{}
			mixedMetricsReporter := models.NewMixedMetricsReporter()
			mixedMetricsReporter.AddReporter(mr)
			f := func() error {
				sendToChan(testChan)
				return nil
			}
			err := mixedMetricsReporter.WithExternalSegment("url", f)
			Expect(err).NotTo(HaveOccurred())
			Eventually(testChan).Should(Receive())
		})
	})
})
