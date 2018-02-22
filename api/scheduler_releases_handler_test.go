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
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/topfreegames/maestro/login"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/testing"
)

var _ = Describe("SchedulerReleasesHandler", func() {
	var request *http.Request
	var recorder *httptest.ResponseRecorder
	var url string
	var yamlString = `name: scheduler-name`
	var configYaml *models.ConfigYAML
	var errDB = errors.New("db failed")

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
		configYaml, _ = models.NewConfigYAML(yamlString)
		url = fmt.Sprintf("http://%s/scheduler/%s/releases", app.Address, configYaml.Name)
		request, _ = http.NewRequest("GET", url, nil)
	})

	Describe("GET /scheduler/{schedulerName}/releases", func() {
		It("should return scheduler releases", func() {
			versions := []string{"1", "2", "3"}
			testing.MockSelectSchedulerVersions(yamlString, versions, mockDb, nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).ToNot(HaveOccurred())

			Expect(response["releases"]).ToNot(BeNil())
			releases := response["releases"].([]interface{})
			for i, release := range releases {
				releaseMap := release.(map[string]interface{})
				Expect(releaseMap["version"]).To(Equal(fmt.Sprintf("v%d", i+1)))
			}
		})

		It("should return error if db fails", func() {
			versions := []string{"1", "2", "3"}
			testing.MockSelectSchedulerVersions(yamlString, versions, mockDb, errDB)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).ToNot(HaveOccurred())

			Expect(response).ToNot(BeNil())
		})

		It("should return error if scheduler not found", func() {
			versions := []string{}
			testing.MockSelectSchedulerVersions(yamlString, versions, mockDb, nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).ToNot(HaveOccurred())

			Expect(response["releases"].([]interface{})).To(BeEmpty())
		})
	})
})
