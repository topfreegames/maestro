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

	"github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/login"
	"github.com/topfreegames/maestro/models"
)

var _ = Describe("SchedulerLocksHandler", func() {
	var request *http.Request
	var recorder *httptest.ResponseRecorder
	var url string
	var yamlString = `name: scheduler-name`
	var configYaml *models.ConfigYAML
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
		configYaml, _ = models.NewConfigYAML(yamlString)
		url = fmt.Sprintf("http://%s/scheduler/%s/locks", app.Address, configYaml.Name)
		request, _ = http.NewRequest("GET", url, nil)
	})

	Describe("GET /scheduler/{schedulerName}/locks", func() {
		BeforeEach(func() {
			recorder = httptest.NewRecorder()
			configYaml, _ = models.NewConfigYAML(yamlString)
			url = fmt.Sprintf("http://%s/scheduler/%s/locks", app.Address, configYaml.Name)
			request, _ = http.NewRequest("GET", url, nil)
		})

		It("should return unlocked watcher lock", func() {
			mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().TTL("maestro-lock-key-scheduler-name").
				Return(redis.NewDurationResult(9*time.Second, nil))
			mockPipeline.EXPECT().Exists("maestro-lock-key-scheduler-name").
				Return(redis.NewIntResult(1, nil))
			mockPipeline.EXPECT().Exec()
			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))
			var locks []models.SchedulerLock
			json.Unmarshal(recorder.Body.Bytes(), &locks)
			Expect(locks).To(HaveLen(1))
			Expect(locks[0].Key).To(Equal("maestro-lock-key-scheduler-name"))
			Expect(locks[0].TTLInSec).To(Equal(int64(9)))
			Expect(locks[0].IsLocked).To(BeTrue())
		})

		It("should return locked watcher lock", func() {
			mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().TTL("maestro-lock-key-scheduler-name").
				Return(redis.NewDurationResult(time.Duration(0), redis.Nil))
			mockPipeline.EXPECT().Exists("maestro-lock-key-scheduler-name").
				Return(redis.NewIntResult(0, nil))
			mockPipeline.EXPECT().Exec()
			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))
			var locks []models.SchedulerLock
			json.Unmarshal(recorder.Body.Bytes(), &locks)
			Expect(locks).To(HaveLen(1))
			Expect(locks[0].Key).To(Equal("maestro-lock-key-scheduler-name"))
			Expect(locks[0].TTLInSec).To(Equal(int64(0)))
			Expect(locks[0].IsLocked).To(BeFalse())
		})
	})

	Describe("DELETE /scheduler/{schedulerName}/locks/{lockName}", func() {
		BeforeEach(func() {
			recorder = httptest.NewRecorder()
			configYaml, _ = models.NewConfigYAML(yamlString)
		})

		It("should remove lockName key in redis", func() {
			url = fmt.Sprintf(
				"http://%s/scheduler/%s/locks/%s", app.Address, configYaml.Name,
				models.GetSchedulerLockKey(app.Config.GetString("watcher.lockKey"), configYaml.Name),
			)
			request, _ = http.NewRequest("DELETE", url, nil)
			mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
			mockRedisClient.EXPECT().Del("maestro-lock-key-scheduler-name").Return(redis.NewIntResult(1, nil))
			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusNoContent))
		})

		It("should return 400 when trying to remove invalid lock key", func() {
			url = fmt.Sprintf(
				"http://%s/scheduler/%s/locks/%s", app.Address, configYaml.Name, "invalid_lock_key",
			)
			request, _ = http.NewRequest("DELETE", url, nil)
			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
		})
	})
})
