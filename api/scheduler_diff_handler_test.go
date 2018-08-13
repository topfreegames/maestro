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
		mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB).AnyTimes()
		mockDb.EXPECT().Context().AnyTimes()
		mockDb.EXPECT().Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, gomock.Any()).
			Do(func(destToken *login.DestinationToken, query string, modifier string) {
				destToken.RefreshToken = "refresh-token"
			}).AnyTimes()
		mockLogin.EXPECT().
			Authenticate(gomock.Any(), app.DBClient.DB).
			Return("user@example.com", http.StatusOK, nil).
			AnyTimes()

		recorder = httptest.NewRecorder()
		url = fmt.Sprintf("http://%s/scheduler/scheduler-name/diff", app.Address)
	})

	Describe("GET /scheduler/{schedulerName}/releases", func() {
		It("should return schedulers diff between current and previous", func() {
			request, _ = http.NewRequest("GET", url, nil)

			MockSelectScheduler(yaml1, mockDb, nil)
			MockSelectPreviousSchedulerVersion(name, "v0.1", yaml2, mockDb, nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]string
			json.Unmarshal(recorder.Body.Bytes(), &response)

			Expect(response["version1"]).To(Equal("v1.0"))
			Expect(response["version2"]).To(Equal("v0.1"))
			Expect(response["diff"]).To(Equal("name: scheduler-name\ngame: \"\"\nshutdownTimeout: 0\nautoscaling: null\naffinity: \"\"\ntoleration: \"\"\noccupiedTimeout: 0\nforwarders: {}\nauthorizedUsers: []\nportRange: null\nimage: image\x1b[31m2\x1b[0m\x1b[32m1\x1b[0m\nimagePullPolicy: \"\"\nports: []\nlimits: null\nrequests: null\nenv: []\ncmd: []\ncontainers: []\n"))
		})

		It("should return schedulers diff between v2 and previous", func() {
			request, _ = http.NewRequest("GET", url, JSONFor(JSON{
				"version1": "v2.0",
			}))

			MockSelectYamlWithVersion(yaml1, "v2.0", mockDb, nil)
			MockSelectPreviousSchedulerVersion(name, "v1.0", yaml2, mockDb, nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]string
			json.Unmarshal(recorder.Body.Bytes(), &response)

			Expect(response["version1"]).To(Equal("v2.0"))
			Expect(response["version2"]).To(Equal("v1.0"))
			Expect(response["diff"]).To(Equal("name: scheduler-name\ngame: \"\"\nshutdownTimeout: 0\nautoscaling: null\naffinity: \"\"\ntoleration: \"\"\noccupiedTimeout: 0\nforwarders: {}\nauthorizedUsers: []\nportRange: null\nimage: image\x1b[31m2\x1b[0m\x1b[32m1\x1b[0m\nimagePullPolicy: \"\"\nports: []\nlimits: null\nrequests: null\nenv: []\ncmd: []\ncontainers: []\n"))
		})

		It("should return schedulers diff between v2 and v1", func() {
			request, _ = http.NewRequest("GET", url, JSONFor(JSON{
				"version1": "v2.0",
				"version2": "v1.0",
			}))

			MockSelectYamlWithVersion(yaml1, "v2.0", mockDb, nil)
			MockSelectYamlWithVersion(yaml2, "v1.0", mockDb, nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]string
			json.Unmarshal(recorder.Body.Bytes(), &response)

			Expect(response["version1"]).To(Equal("v2.0"))
			Expect(response["version2"]).To(Equal("v1.0"))
			Expect(response["diff"]).To(Equal("name: scheduler-name\ngame: \"\"\nshutdownTimeout: 0\nautoscaling: null\naffinity: \"\"\ntoleration: \"\"\noccupiedTimeout: 0\nforwarders: {}\nauthorizedUsers: []\nportRange: null\nimage: image\x1b[31m2\x1b[0m\x1b[32m1\x1b[0m\nimagePullPolicy: \"\"\nports: []\nlimits: null\nrequests: null\nenv: []\ncmd: []\ncontainers: []\n"))
		})

		It("should return error if version1 not found", func() {
			request, _ = http.NewRequest("GET", url, JSONFor(JSON{
				"version1": "v1.0",
				"version2": "v2.0",
			}))

			mockDb.EXPECT().
				Query(
					gomock.Any(),
					"SELECT yaml FROM scheduler_versions WHERE name = ? AND version = ?", name, "v1.0").
				Do(func(scheduler *models.Scheduler, query, name, version string) {
					*scheduler = *models.NewScheduler(name, "", "")
				})

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusNotFound))

			var response map[string]string
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response["code"]).To(Equal("MAE-000"))
			Expect(response["description"]).To(Equal("config scheduler not found: scheduler-name:v1.0"))
			Expect(response["error"]).To(Equal("schedulers diff failed"))
		})

		It("should return error if version2 not found", func() {
			request, _ = http.NewRequest("GET", url, JSONFor(JSON{
				"version1": "v1.0",
				"version2": "v2.0",
			}))

			MockSelectYamlWithVersion(yaml1, "v1.0", mockDb, nil)
			mockDb.EXPECT().
				Query(
					gomock.Any(),
					"SELECT yaml FROM scheduler_versions WHERE name = ? AND version = ?", name, "v2.0").
				Do(func(scheduler *models.Scheduler, query, name, version string) {
					*scheduler = *models.NewScheduler(name, "", "")
				})

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusNotFound))

			var response map[string]string
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response["code"]).To(Equal("MAE-000"))
			Expect(response["description"]).To(Equal("config scheduler not found: scheduler-name:v2.0"))
			Expect(response["error"]).To(Equal("schedulers diff failed"))
		})
	})
})
