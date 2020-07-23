// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"github.com/topfreegames/maestro/api/auth"
	"net/http"
	"net/http/httptest"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/topfreegames/extensions/middleware"
	. "github.com/topfreegames/maestro/api"
	. "github.com/topfreegames/maestro/testing"
)

var _ = Describe("AuthMiddleware", func() {
	var authMiddleware *AuthMiddleware
	var request *http.Request
	var recorder *httptest.ResponseRecorder
	var dummyMiddleware = &DummyMiddleware{}

	BeforeEach(func() {
		authMiddleware = NewAuthMiddleware(app, auth.ActionResolver("SomePermission"))
		authMiddleware.SetNext(dummyMiddleware)

		request, _ = http.NewRequest("GET", "/scheduler/{schedulerName}/any/route", nil)
		request = request.WithContext(middleware.SetLogger(request.Context(), logger))

		recorder = httptest.NewRecorder()
	})

	Describe("ServeHTTP", func() {
		It("should return ok if not enabled", func() {
			config, _ := GetDefaultConfig()
			config.Set("oauth.enabled", false)

			app.Config = config
			authMiddleware = NewAuthMiddleware(app, auth.ActionResolver("SomePermission"))
			authMiddleware.SetNext(dummyMiddleware)

			authMiddleware.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("should return ok if from basicauth", func() {
			request = request.WithContext(
				auth.NewContextWithBasicAuthOK(request.Context()))

			authMiddleware.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("should ok if admin", func() {
			request = request.WithContext(
				auth.NewContextWithEmail(request.Context(), "user@example.com"))

			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)

			authMiddleware.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("should ok if not admin but authorized to scheduler", func() {
			request = request.WithContext(
				auth.NewContextWithEmail(request.Context(), "scheduler_user@example.com"))
			request = mux.SetURLVars(request, map[string]string{"schedulerName": "scheduler-name"})

			yamlStr := `name: scheduler-name
authorizedUsers:
- scheduler_user@example.com`
			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)
			MockSelectScheduler(yamlStr, mockDb, nil)

			authMiddleware.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("should ok if not admin and not authorized to scheduler", func() {
			request = request.WithContext(
				auth.NewContextWithEmail(request.Context(), "not_a_scheduler_user@example.com"))
			request = mux.SetURLVars(request, map[string]string{"schedulerName": "scheduler-name"})

			yamlStr := `name: scheduler-name
authorizedUsers:
- scheduler_user@example.com`
			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)
			MockSelectScheduler(yamlStr, mockDb, nil)

			authMiddleware.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
		})
	})
})
