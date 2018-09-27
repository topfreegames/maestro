// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2018 Top Free Games <backend@tfgco.com>

package reporters_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/topfreegames/maestro/reporters"
	"github.com/topfreegames/maestro/reporters/constants"
	handlers "github.com/topfreegames/maestro/reporters/http"
	httpMocks "github.com/topfreegames/maestro/reporters/http/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTTP", func() {
	var (
		r    *reporters.Reporters
		opts map[string]interface{}
	)

	BeforeEach(func() {
		r = reporters.NewReporters()
		opts = map[string]interface{}{"game": "pong"}
	})

	It(`MakeHTTP should create a new HTTP instance and add it to the singleton
	reporters.Reporters`, func() {
		_, prs := r.GetReporter("http")
		Expect(prs).To(BeFalse())
		reporters.MakeHTTP(config, logger, r)
		_, prs = r.GetReporter("http")
		Expect(prs).To(BeTrue())
	})

	Describe("AnyHandler", func() {
		It("Should call Client.Send", func() {
			c := httpMocks.NewMockClient(mockCtrl)
			c.EXPECT().Send(opts)
			handlers.AnyHandler(c, opts)
		})
	})

	Describe("Handlers", func() {
		It("Should handle EventSchedulerUpdate", func() {
			_, found := handlers.Find(constants.EventSchedulerUpdate)
			Expect(found).To(BeTrue())
		})
	})

	Describe("Report", func() {
		var ts *httptest.Server

		BeforeEach(func() {
			ts = httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var bodyBytes []byte
					Expect(r.Body).NotTo(BeNil())
					bodyBytes, _ = ioutil.ReadAll(r.Body)
					defer r.Body.Close()
					var bodyMap map[string]interface{}
					json.Unmarshal(bodyBytes, &bodyMap)
					Expect(
						bodyMap["metadata"].(map[string]interface{})["game"].(string),
					).To(Equal("pong"))
					Expect(
						bodyMap["metadata"].(map[string]interface{})["region"].(string),
					).To(Equal("test"))
					tags := bodyMap["tags"].([]interface{})
					Expect(len(tags)).To(Equal(3))
					Expect(tags[0].(string)).To(Equal("maestro"))
					Expect(tags[1].(string)).To(Equal("scheduler.update"))
					Expect(tags[2].(string)).To(Equal("test"))
					w.WriteHeader(http.StatusCreated)
				}),
			)
			client := reporters.NewHTTPClient(ts.URL)
			reporters.MakeHTTPWithClient(client, config, logger, r)
		})

		AfterEach(func() {
			ts.Close()
		})

		It("Should send the expected body", func() {
			err := r.Report(constants.EventSchedulerUpdate, opts)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
