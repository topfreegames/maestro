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
	"time"

	goredis "github.com/go-redis/redis"
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
	var opManager *models.OperationManager
	var timeoutDur = time.Duration(300) * time.Second

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

			mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
			opManager = models.NewOperationManager(configYaml.Name, mockRedisClient, logger)
			MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

			version := "v1.0"
			// Select version from database
			MockSelectYamlWithVersion(yamlStringToRollbackTo, version, mockDb, nil)

			calls := NewCalls()

			configLockKey := models.GetSchedulerConfigLockKey(config.GetString("watcher.lockKey"), scheduler1.Name)

			// Get config lock
			MockRedisLock(mockRedisClient, configLockKey, lockTimeoutMs, true, nil)

			// Set new operation manager description
			MockAnySetDescription(opManager, mockRedisClient, models.OpManagerRunning, nil)

			// Get scheduler from DB
			MockSelectScheduler(yamlString, mockDb, nil)

			// Update scheduler
			MockUpdateSchedulersTable(mockDb, nil)

			// Add new version into versions table
			scheduler1.NextMinorVersion()
			MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

			// Count to delete old versions if necessary
			MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

			// Release configLock
			MockReturnRedisLock(mockRedisClient, configLockKey, nil)

			MockUpdateVersionsTable(mockDb, nil)

			calls.Finish()

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

	Describe("PUT /scheduler/{schedulerName}/rollback?async=true", func() {
		BeforeEach(func() {
			reader := JSONFor(JSON{
				"version": "v1.0",
			})
			url = fmt.Sprintf("http://%s/scheduler/%s/rollback?async=true", app.Address, configYaml.Name)
			request, _ = http.NewRequest("PUT", url, reader)
		})

		It("should asynchronously rollback to previous version", func() {
			mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
			yamlStringToRollbackTo := `name: scheduler-name
game: game
autoscaling:
  min: 10`

			yaml.Unmarshal([]byte(yamlStringToRollbackTo), &configYaml)
			scheduler1 := models.NewScheduler(configYaml.Name, configYaml.Game, yamlStringToRollbackTo)

			opManager = models.NewOperationManager(configYaml.Name, mockRedisClient, logger)
			MockGetCurrentOperationKey(opManager, mockRedisClient, nil)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
				Expect(m).To(HaveKeyWithValue("operation", "SchedulerRollback"))
			})
			mockPipeline.EXPECT().Expire(gomock.Any(), timeoutDur)
			mockPipeline.EXPECT().Set(opManager.BuildCurrOpKey(), gomock.Any(), timeoutDur)
			mockPipeline.EXPECT().Exec()

			mockRedisClient.EXPECT().HGetAll(gomock.Any()).
				Return(goredis.NewStringStringMapResult(map[string]string{
					"description": models.OpManagerWaitingLock,
			}, nil))

			operationFinished := false
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
				Expect(m).To(HaveKeyWithValue("status", http.StatusOK))
				Expect(m).To(HaveKeyWithValue("operation", "SchedulerRollback"))
				Expect(m).To(HaveKeyWithValue("success", true))
			})
			mockPipeline.EXPECT().Expire(gomock.Any(), 10*time.Minute)
			mockPipeline.EXPECT().Del(opManager.BuildCurrOpKey()).Do(func(_ interface{}) {
				operationFinished = true
			})
			mockPipeline.EXPECT().Exec()

			version := "v1.0"
			// Select version from database
			MockSelectYamlWithVersion(yamlStringToRollbackTo, version, mockDb, nil)

			configLockKey := models.GetSchedulerConfigLockKey(config.GetString("watcher.lockKey"), scheduler1.Name)

			// Get config lock
			MockRedisLock(mockRedisClient, configLockKey, lockTimeoutMs, true, nil)

			// Set new operation manager description
			MockAnySetDescription(opManager, mockRedisClient, models.OpManagerRunning, nil)

			// Get scheduler from DB
			MockSelectScheduler(yamlString, mockDb, nil)

			// Update scheduler
			MockUpdateSchedulersTable(mockDb, nil)

			// Add new version into versions table
			scheduler1.NextMinorVersion()
			MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

			// Count to delete old versions if necessary
			MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

			// Release configLock
			MockReturnRedisLock(mockRedisClient, configLockKey, nil)

			MockUpdateVersionsTable(mockDb, nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response).To(HaveKeyWithValue("success", true))
			Expect(response).To(HaveKey("operationKey"))
			Eventually(func() bool { return operationFinished }, time.Minute, time.Second).Should(BeTrue())
		})

		It("should save error on redis if failed", func() {
			mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
			mockRedisClient.EXPECT().Ping().AnyTimes()

			yamlStringToRollbackTo := `name: scheduler-name
game: game
autoscaling:
  min: 10`

			yaml.Unmarshal([]byte(yamlStringToRollbackTo), &configYaml)

			opManager = models.NewOperationManager(configYaml.Name, mockRedisClient, logger)
			MockGetCurrentOperationKey(opManager, mockRedisClient, nil)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
				Expect(m).To(HaveKeyWithValue("operation", "SchedulerRollback"))
			})
			mockPipeline.EXPECT().Expire(gomock.Any(), timeoutDur)
			mockPipeline.EXPECT().Set(opManager.BuildCurrOpKey(), gomock.Any(), timeoutDur)
			mockPipeline.EXPECT().Exec()

			mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(nil, nil)).AnyTimes()

			operationFinished := false
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
				Expect(m).To(HaveKeyWithValue("status", http.StatusBadRequest))
				Expect(m).To(HaveKeyWithValue("operation", "SchedulerRollback"))
				Expect(m).To(HaveKeyWithValue("success", false))
				Expect(m).To(HaveKeyWithValue("description", "invalid parameters"))
				Expect(m).To(HaveKeyWithValue("error", "invalid parameter: maxsurge must be greater than 0"))
			})
			mockPipeline.EXPECT().Expire(gomock.Any(), 10*time.Minute)
			mockPipeline.EXPECT().Del(opManager.BuildCurrOpKey())
			mockPipeline.EXPECT().Exec().Do(func() {
				operationFinished = true
			})

			version := "v1.0"
			// Select version from database
			MockSelectYamlWithVersion(yamlStringToRollbackTo, version, mockDb, nil)

			reader := JSONFor(JSON{
				"version": "v1.0",
			})
			url = fmt.Sprintf("http://%s/scheduler/%s/rollback?async=true&maxsurge=-50", app.Address, configYaml.Name)
			request, _ = http.NewRequest("PUT", url, reader)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(response).To(HaveKeyWithValue("success", true))
			Expect(response).To(HaveKey("operationKey"))
			Eventually(func() bool { return operationFinished }, time.Minute, time.Second).Should(BeTrue())
		})
	})
})
