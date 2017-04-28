// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/extensions/pg/mocks"
	. "github.com/topfreegames/maestro/testing"
)

var _ = Describe("Room Handler", func() {
	var request *http.Request
	var recorder *httptest.ResponseRecorder

	BeforeEach(func() {
		// Record HTTP responses.
		recorder = httptest.NewRecorder()
	})

	Describe("PUT /scheduler/{schedulerName}/rooms/{roomName}/ping", func() {
		url := "/scheduler/schedulerName/rooms/roomName/ping"
		BeforeEach(func() {
			reader := JSONFor(JSON{
				"timestamp": time.Now().Unix(),
			})
			request, _ = http.NewRequest("PUT", url, reader)
		})

		Context("when all services are healthy", func() {
			It("returns a status code of 200 and success body", func() {
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(db.Execs).To(HaveLen(1))
			})

			It("returns status code of 422 if missing timestamp", func() {
				reader := JSONFor(JSON{})
				request, _ = http.NewRequest("PUT", url, reader)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(422))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(ContainSubstring("Timestamp: non zero value required"))
				Expect(obj["success"]).To(Equal(false))
				Expect(db.Execs).To(HaveLen(0))
			})

			It("returns status code of 422 if invalid timestamp", func() {
				reader := JSONFor(JSON{
					"timestamp": "not-a-timestamp",
				})
				request, _ = http.NewRequest("PUT", url, reader)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(422))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(ContainSubstring("RoomPingPayload.timestamp"))
				Expect(obj["success"]).To(Equal(false))
				Expect(db.Execs).To(HaveLen(0))
			})
		})

		Context("when postgres is down", func() {
			It("returns status code of 500 if database is unavailable", func() {
				app.DB = mocks.NewPGMock(1, 1, fmt.Errorf("sql: database is closed"))
				app.Router.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-000"))
				Expect(obj["error"]).To(Equal("Ping failed"))
				Expect(obj["description"]).To(Equal("sql: database is closed"))
				Expect(obj["success"]).To(Equal(false))
				Expect(db.Execs).To(HaveLen(0))
			})
		})
	})

	Describe("PUT /scheduler/{schedulerName}/rooms/{roomName}/status", func() {
		url := "/scheduler/schedulerName/rooms/roomName/status"
		BeforeEach(func() {
			reader := JSONFor(JSON{
				"status":    "ready",
				"timestamp": time.Now().Unix(),
			})
			request, _ = http.NewRequest("PUT", url, reader)
		})

		Context("when all services are healthy", func() {
			It("returns a status code of 200 and success body", func() {
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(db.Execs).To(HaveLen(1))
			})

			It("returns status code of 422 if missing timestamp", func() {
				reader := JSONFor(JSON{
					"status": "ready",
				})
				request, _ = http.NewRequest("PUT", url, reader)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(422))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(ContainSubstring("Timestamp: non zero value required"))
				Expect(obj["success"]).To(Equal(false))
				Expect(db.Execs).To(HaveLen(0))
			})

			It("returns status code of 422 if invalid timestamp", func() {
				reader := JSONFor(JSON{
					"status":    "ready",
					"timestamp": "not-a-timestamp",
				})
				request, _ = http.NewRequest("PUT", url, reader)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(422))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(ContainSubstring("RoomStatusPayload.timestamp"))
				Expect(obj["success"]).To(Equal(false))
				Expect(db.Execs).To(HaveLen(0))
			})

			It("returns status code of 422 if missing status", func() {
				reader := JSONFor(JSON{
					"timestamp": time.Now().Unix(),
				})
				request, _ = http.NewRequest("PUT", url, reader)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(422))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(ContainSubstring("Status: non zero value required"))
				Expect(obj["success"]).To(Equal(false))
				Expect(db.Execs).To(HaveLen(0))
			})

			It("returns status code of 422 if invalid status", func() {
				reader := JSONFor(JSON{
					"status":    "invalid",
					"timestamp": time.Now().Unix(),
				})
				request, _ = http.NewRequest("PUT", url, reader)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(422))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(ContainSubstring("does not validate as matches"))
				Expect(obj["success"]).To(Equal(false))
				Expect(db.Execs).To(HaveLen(0))
			})
		})

		Context("when postgres is down", func() {
			It("returns status code of 500 if database is unavailable", func() {
				app.DB = mocks.NewPGMock(1, 1, fmt.Errorf("sql: database is closed"))
				app.Router.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-000"))
				Expect(obj["error"]).To(Equal("Status update failed"))
				Expect(obj["description"]).To(Equal("sql: database is closed"))
				Expect(obj["success"]).To(Equal(false))
				Expect(db.Execs).To(HaveLen(0))
			})
		})
	})
})
