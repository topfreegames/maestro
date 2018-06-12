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

	yaml "gopkg.in/yaml.v2"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/topfreegames/maestro/api"
	"github.com/topfreegames/maestro/login"
	"github.com/topfreegames/maestro/models"
	. "github.com/topfreegames/maestro/testing"
)

var _ = Describe("SchedulerConfigHandler", func() {
	var request *http.Request
	var recorder *httptest.ResponseRecorder
	var url string
	var configYaml *models.ConfigYAML
	var yamlString = `name: scheduler-name`
	var user, pass string
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

		configYaml, _ = models.NewConfigYAML(yamlString)

		url = fmt.Sprintf("http://%s/scheduler/%s/config", app.Address, configYaml.Name)
		request, _ = http.NewRequest("GET", url, nil)
		user = app.Config.GetString("basicauth.username")
		pass = app.Config.GetString("basicauth.password")
		request.SetBasicAuth(user, pass)
	})

	Describe("GET /scheduler/{schedulerName}/config", func() {
		It("should return config yaml of current version", func() {
			MockSelectYaml(yamlString, mockDb, nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)
			var yamlResp map[string]interface{}
			yaml.Unmarshal([]byte(response["yaml"].(string)), &yamlResp)
			Expect(yamlResp["name"]).To(Equal(configYaml.Name))
		})

		It("should return config yaml of current version with oauth", func() {
			MockSelectYaml(yamlString, mockDb, nil)

			config, err := GetDefaultConfig()
			Expect(err).NotTo(HaveOccurred())
			config.Set("basicauth.tryOauthIfUnset", true)

			app, err := api.NewApp("0.0.0.0", 9998, config, logger, false, false, "", mockDb, mockRedisClient, clientset)
			Expect(err).NotTo(HaveOccurred())
			app.Login = mockLogin

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)
			var yamlResp map[string]interface{}
			yaml.Unmarshal([]byte(response["yaml"].(string)), &yamlResp)
			Expect(yamlResp["name"]).To(Equal(configYaml.Name))
		})

		It("should return config json of current version", func() {
			MockSelectYaml(yamlString, mockDb, nil)

			request.Header.Add("Accept", "application/json")
			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var schedulerConfig map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &schedulerConfig)
			yamlBytes, _ := json.Marshal(schedulerConfig)
			var yamlResp map[string]interface{}
			yaml.Unmarshal(yamlBytes, &yamlResp)
			Expect(yamlResp["name"]).To(Equal(configYaml.Name))
		})

		It("should return config yaml of specified version", func() {
			version := "v1.0"
			MockSelectYamlWithVersion(yamlString, version, mockDb, nil)

			url = fmt.Sprintf("http://%s/scheduler/%s/config?version=%s",
				app.Address, configYaml.Name, version)
			request, _ = http.NewRequest("GET", url, nil)
			request.SetBasicAuth(user, pass)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)
			var yamlResp map[string]interface{}
			yaml.Unmarshal([]byte(response["yaml"].(string)), &yamlResp)
			Expect(yamlResp["name"]).To(Equal(configYaml.Name))
		})

		It("should return error if scheduler not found", func() {
			version := "v1.0"

			mockDb.EXPECT().
				Query(gomock.Any(),
					"SELECT yaml FROM scheduler_versions WHERE name = ? AND version = ?",
					"scheduler-name", version).
				Do(func(scheduler *models.Scheduler, query, name, version string) {
					*scheduler = *models.NewScheduler(configYaml.Name, configYaml.Game, "")
				})

			url = fmt.Sprintf("http://%s/scheduler/%s/config?version=%s",
				app.Address, configYaml.Name, version)
			request, _ = http.NewRequest("GET", url, nil)
			request.SetBasicAuth(user, pass)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusNotFound))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response).To(HaveKeyWithValue("code", "MAE-000"))
			Expect(response).To(HaveKeyWithValue("description", "config scheduler not found"))
			Expect(response).To(HaveKeyWithValue("error", "get config error"))
			Expect(response).To(HaveKeyWithValue("success", false))
		})
	})
})
