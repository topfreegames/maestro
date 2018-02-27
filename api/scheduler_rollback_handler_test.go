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
	"github.com/topfreegames/maestro/login"
	"github.com/topfreegames/maestro/models"
	. "github.com/topfreegames/maestro/testing"
)

var _ = Describe("SchedulerRollbackHandler", func() {
	var request *http.Request
	var recorder *httptest.ResponseRecorder
	var url string
	var yamlString = `name: scheduler-name
game: game
autoscaling:
  min: 1`
	var configYaml *models.ConfigYAML
	var numberOfVersions int = 1

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

		reader := JSONFor(JSON{
			"version": "v1.0",
		})

		url = fmt.Sprintf("http://%s/scheduler/%s/rollback", app.Address, configYaml.Name)
		request, _ = http.NewRequest("PUT", url, reader)
	})

	Describe("PUT /scheduler/{schedulerName}/rollback", func() {
		It("should rollback to previous version", func() {
			yamlStringToRollbackTo := `name: scheduler-name
game: game
autoscaling:
  min: 10`

			yaml.Unmarshal([]byte(yamlStringToRollbackTo), &configYaml)
			scheduler1 := models.NewScheduler(configYaml.Name, configYaml.Game, yamlStringToRollbackTo)
			lockKeyNs := fmt.Sprintf("%s-scheduler-name", lockKey)

			version := "v1.0"
			// Select version from database
			MockSelectYamlWithVersion(yamlStringToRollbackTo, version, mockDb, nil)

			// Select current scheduler
			MockSelectScheduler(yamlString, mockDb, nil)

			// Get redis lock
			MockRedisLock(mockRedisClient, lockKeyNs, lockTimeoutMs, true, nil)

			// Update new config on schedulers table
			MockUpdateSchedulersTable(mockDb, nil)

			// Add new version into versions table
			scheduler1.NextMinorVersion()
			MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

			// Count to delete old versions if necessary
			MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

			// Retrieve redis lock
			MockReturnRedisLock(mockRedisClient, lockKeyNs, nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
		})

		It("should return error if scheduler not found", func() {
			version := "v1.0"
			// Select version from database
			mockDb.EXPECT().
				Query(gomock.Any(),
					"SELECT yaml FROM scheduler_versions WHERE name = ? AND version = ?",
					"scheduler-name", version).
				Do(func(scheduler *models.Scheduler, query, name, version string) {
					*scheduler = *models.NewScheduler(configYaml.Name, configYaml.Game, "")
				})

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusNotFound))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response).To(HaveKeyWithValue("code", "MAE-000"))
			Expect(response).To(HaveKeyWithValue("description", "config scheduler not found for version"))
			Expect(response).To(HaveKeyWithValue("error", "get config error"))
			Expect(response).To(HaveKeyWithValue("success", false))
		})

		It("should return error if invalid yaml", func() {
			version := "v1.0"
			// Select version from database
			mockDb.EXPECT().
				Query(gomock.Any(),
					"SELECT yaml FROM scheduler_versions WHERE name = ? AND version = ?",
					"scheduler-name", version).
				Do(func(scheduler *models.Scheduler, query, name, version string) {
					*scheduler = *models.NewScheduler(configYaml.Name, configYaml.Game, "  invalid{ yaml!.")
				})

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response).To(HaveKeyWithValue("code", "MAE-002"))
			Expect(response).To(HaveKeyWithValue("description", "yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `invalid...` into models.ConfigYAML"))
			Expect(response).To(HaveKeyWithValue("error", "parse yaml error"))
			Expect(response).To(HaveKeyWithValue("success", false))
		})
	})
})
