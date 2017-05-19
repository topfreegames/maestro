// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"

	"gopkg.in/pg.v5/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/metadata"
)

var _ = Describe("Healthcheck Handler", func() {
	var request *http.Request
	var recorder *httptest.ResponseRecorder

	BeforeEach(func() {
		// Record HTTP responses.
		recorder = httptest.NewRecorder()
	})

	Describe("GET /healthcheck", func() {
		BeforeEach(func() {
			request, _ = http.NewRequest("GET", "/healthcheck", nil)
		})

		Context("when all services are healthy", func() {
			It("returns a status code of 200", func() {
				mockDb.EXPECT().Exec("select 1")
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(200))
			})

			It("returns working string", func() {
				mockDb.EXPECT().Exec("select 1")
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Body.String()).To(Equal(`{"healthy": true}`))
			})

			It("returns the version as a header", func() {
				mockDb.EXPECT().Exec("select 1")
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Header().Get("X-Maestro-Version")).To(Equal(metadata.Version))
			})
		})

		Context("when postgres is down", func() {
			It("returns status code of 500 if database is unavailable", func() {
				mockDb.EXPECT().Exec("select 1").Return(&types.Result{}, errors.New("sql: database is closed"))
				app.Router.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-001"))
				Expect(obj["error"]).To(Equal("DatabaseError"))
				Expect(obj["description"]).To(Equal("sql: database is closed"))
				Expect(obj["success"]).To(Equal(false))
			})
		})
	})
})
