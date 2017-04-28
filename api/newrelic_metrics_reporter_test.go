// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"net/http"
	"net/http/httptest"

	newrelic "github.com/newrelic/go-agent"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/api"
)

var _ = Describe("NewRelic Metrics Reporter", func() {
	var recorder *httptest.ResponseRecorder
	request, _ := http.NewRequest("GET", "/healthcheck", nil)

	BeforeEach(func() {
		recorder = httptest.NewRecorder()
	})

	Describe("StartSegment", func() {
		It("should return nil if transaction is nil", func() {
			nrMetricsReporter := &api.NewRelicMetricsReporter{}
			seg := nrMetricsReporter.StartSegment("name")
			Expect(seg).To(BeNil())
		})

		It("should return map string interface if transaction is not nil", func() {
			txn := app.NewRelic.StartTransaction("txn", recorder, request)
			defer txn.End()
			nrMetricsReporter := &api.NewRelicMetricsReporter{Transaction: txn}
			seg := nrMetricsReporter.StartSegment("name")
			Expect(seg["segment"]).NotTo(BeNil())
			var nrSegment newrelic.Segment
			Expect(seg["segment"]).To(BeAssignableToTypeOf(nrSegment))
		})
	})

	Describe("EndSegment", func() {
		It("should not fail if transaction is nil", func() {
			nrMetricsReporter := &api.NewRelicMetricsReporter{}
			Expect(func() { nrMetricsReporter.EndSegment(nil, "name") }).NotTo(Panic())
		})

		It("should not fail if transaction is not nil", func() {
			txn := app.NewRelic.StartTransaction("txn", recorder, request)
			nrMetricsReporter := &api.NewRelicMetricsReporter{Transaction: txn}
			seg := nrMetricsReporter.StartSegment("name")
			Expect(func() { nrMetricsReporter.EndSegment(seg, "name") }).NotTo(Panic())
		})
	})

	Describe("StartDatastoreSegment", func() {
		It("should return nil if transaction is nil", func() {
			nrMetricsReporter := &api.NewRelicMetricsReporter{}
			seg := nrMetricsReporter.StartDatastoreSegment("datastore", "table", "operation")
			Expect(seg).To(BeNil())
		})

		It("should return map string interface if transaction is not nil", func() {
			txn := app.NewRelic.StartTransaction("txn", recorder, request)
			defer txn.End()
			nrMetricsReporter := &api.NewRelicMetricsReporter{Transaction: txn}
			seg := nrMetricsReporter.StartDatastoreSegment("datastore", "table", "operation")
			Expect(seg["segment"]).NotTo(BeNil())
			var nrDatastoreSegment newrelic.DatastoreSegment
			Expect(seg["segment"]).To(BeAssignableToTypeOf(nrDatastoreSegment))
		})
	})

	Describe("EndDatastoreSegment", func() {
		It("should not fail if transaction is nil", func() {
			nrMetricsReporter := &api.NewRelicMetricsReporter{}
			Expect(func() { nrMetricsReporter.EndDatastoreSegment(nil) }).NotTo(Panic())
		})

		It("should not fail if transaction is not nil", func() {
			txn := app.NewRelic.StartTransaction("txn", recorder, request)
			nrMetricsReporter := &api.NewRelicMetricsReporter{Transaction: txn}
			seg := nrMetricsReporter.StartDatastoreSegment("datastore", "table", "operation")
			Expect(func() { nrMetricsReporter.EndDatastoreSegment(seg) }).NotTo(Panic())
		})
	})

	Describe("StartExternalSegment", func() {
		It("should return nil if transaction is nil", func() {
			nrMetricsReporter := &api.NewRelicMetricsReporter{}
			seg := nrMetricsReporter.StartExternalSegment("url")
			Expect(seg).To(BeNil())
		})

		It("should return map string interface if transaction is not nil", func() {
			txn := app.NewRelic.StartTransaction("txn", recorder, request)
			defer txn.End()
			nrMetricsReporter := &api.NewRelicMetricsReporter{Transaction: txn}
			seg := nrMetricsReporter.StartExternalSegment("url")
			Expect(seg["segment"]).NotTo(BeNil())
			var nrDatastoreSegment newrelic.ExternalSegment
			Expect(seg["segment"]).To(BeAssignableToTypeOf(nrDatastoreSegment))
		})
	})

	Describe("EndExternalSegment", func() {
		It("should not fail if transaction is nil", func() {
			nrMetricsReporter := &api.NewRelicMetricsReporter{}
			Expect(func() { nrMetricsReporter.EndExternalSegment(nil) }).NotTo(Panic())
		})

		It("should not fail if transaction is not nil", func() {
			txn := app.NewRelic.StartTransaction("txn", recorder, request)
			nrMetricsReporter := &api.NewRelicMetricsReporter{Transaction: txn}
			seg := nrMetricsReporter.StartExternalSegment("url")
			Expect(func() { nrMetricsReporter.EndExternalSegment(seg) }).NotTo(Panic())
		})
	})
})
