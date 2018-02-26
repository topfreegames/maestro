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
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/topfreegames/maestro/login"
	"github.com/topfreegames/maestro/models"
	. "github.com/topfreegames/maestro/testing"
)

var _ = Describe("SchedulerDiffHandler", func() {
	var request *http.Request
	var recorder *httptest.ResponseRecorder
	var url string
	var yaml1 = `name: scheduler-name
image: image1`
	var yaml2 = `name: scheduler-name
image: image2`
	var name = "scheduler-name"
	// var errDB = errors.New("db failed")

	BeforeEach(func() {
		mockDb.EXPECT().Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, gomock.Any()).
			Do(func(destToken *login.DestinationToken, query string, modifier string) {
				destToken.RefreshToken = "refresh-token"
			}).AnyTimes()
		mockLogin.EXPECT().
			Authenticate(gomock.Any(), app.DB).
			Return("user@example.com", http.StatusOK, nil).
			AnyTimes()

		recorder = httptest.NewRecorder()
		url = fmt.Sprintf("http://%s/scheduler/scheduler-name/diff", app.Address)
	})

	Describe("GET /scheduler/{schedulerName}/releases", func() {
		It("should return schedulers diff between current and previous", func() {
			request, _ = http.NewRequest("GET", url, nil)

			MockSelectScheduler(yaml1, mockDb, nil)
			MockSelectYamlWithVersion(yaml2, 0, mockDb, nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]string
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response["version1"]).To(Equal("v1"))
			Expect(response["version2"]).To(Equal("v0"))
			Expect(response["diff"]).To(Equal("name: scheduler-name\ngame: \"\"\nshutdownTimeout: 0\nautoscaling: null\naffinity: \"\"\ntoleration: \"\"\noccupiedTimeout: 0\nforwarders: {}\nimage: image\u001b[31m1\u001b[0m\u001b[32m2\u001b[0m\nports: []\nlimits: null\nrequests: null\nenv: []\ncmd: []\ncontainers: []\n"))
		})

		It("should return schedulers diff between v2 and previous", func() {
			request, _ = http.NewRequest("GET", url, JSONFor(JSON{
				"version1": "v2",
			}))

			MockSelectYamlWithVersion(yaml1, 2, mockDb, nil)
			MockSelectYamlWithVersion(yaml2, 1, mockDb, nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]string
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response["version1"]).To(Equal("v2"))
			Expect(response["version2"]).To(Equal("v1"))
			Expect(response["diff"]).To(Equal("name: scheduler-name\ngame: \"\"\nshutdownTimeout: 0\nautoscaling: null\naffinity: \"\"\ntoleration: \"\"\noccupiedTimeout: 0\nforwarders: {}\nimage: image\u001b[31m1\u001b[0m\u001b[32m2\u001b[0m\nports: []\nlimits: null\nrequests: null\nenv: []\ncmd: []\ncontainers: []\n"))
		})

		It("should return schedulers diff between v2 and v1", func() {
			request, _ = http.NewRequest("GET", url, JSONFor(JSON{
				"version1": "v2",
				"version2": "v1",
			}))

			MockSelectYamlWithVersion(yaml1, 2, mockDb, nil)
			MockSelectYamlWithVersion(yaml2, 1, mockDb, nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]string
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response["version1"]).To(Equal("v2"))
			Expect(response["version2"]).To(Equal("v1"))
			Expect(response["diff"]).To(Equal("name: scheduler-name\ngame: \"\"\nshutdownTimeout: 0\nautoscaling: null\naffinity: \"\"\ntoleration: \"\"\noccupiedTimeout: 0\nforwarders: {}\nimage: image\u001b[31m1\u001b[0m\u001b[32m2\u001b[0m\nports: []\nlimits: null\nrequests: null\nenv: []\ncmd: []\ncontainers: []\n"))
		})

		It("should return error if invalid version1", func() {
			request, _ = http.NewRequest("GET", url, JSONFor(JSON{
				"version1": "not a version",
				"version2": "v1",
			}))

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]string
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response["code"]).To(Equal("MAE-000"))
			Expect(response["description"]).To(Equal("version is not of format v<integer>, like v1, v2, v3, etc"))
			Expect(response["error"]).To(Equal("schedulers diff failed"))
		})

		It("should return error if invalid version2", func() {
			request, _ = http.NewRequest("GET", url, JSONFor(JSON{
				"version1": "v1",
				"version2": "not a version",
			}))

			MockSelectYamlWithVersion(yaml1, 1, mockDb, nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]string
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response["code"]).To(Equal("MAE-000"))
			Expect(response["description"]).To(Equal("version is not of format v<integer>, like v1, v2, v3, etc"))
			Expect(response["error"]).To(Equal("schedulers diff failed"))
		})

		It("should return error if version1 not found", func() {
			request, _ = http.NewRequest("GET", url, JSONFor(JSON{
				"version1": "v1",
				"version2": "v2",
			}))

			mockDb.EXPECT().
				Query(
					gomock.Any(),
					"SELECT yaml FROM scheduler_versions WHERE name = ? AND version = ?", name, 1).
				Do(func(scheduler *models.Scheduler, query string, name string, version int) {
					*scheduler = *models.NewScheduler(name, "", "")
				})

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusNotFound))

			var response map[string]string
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response["code"]).To(Equal("MAE-000"))
			Expect(response["description"]).To(Equal("config scheduler not found: scheduler-name:v1"))
			Expect(response["error"]).To(Equal("schedulers diff failed"))
		})

		It("should return error if version2 not found", func() {
			request, _ = http.NewRequest("GET", url, JSONFor(JSON{
				"version1": "v1",
				"version2": "v2",
			}))

			MockSelectYamlWithVersion(yaml1, 1, mockDb, nil)
			mockDb.EXPECT().
				Query(
					gomock.Any(),
					"SELECT yaml FROM scheduler_versions WHERE name = ? AND version = ?", name, 2).
				Do(func(scheduler *models.Scheduler, query string, name string, version int) {
					*scheduler = *models.NewScheduler(name, "", "")
				})

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusNotFound))

			var response map[string]string
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response["code"]).To(Equal("MAE-000"))
			Expect(response["description"]).To(Equal("config scheduler not found: scheduler-name:v2"))
			Expect(response["error"]).To(Equal("schedulers diff failed"))
		})
	})
})
