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
	"github.com/onsi/gomega/ghttp"
	"github.com/topfreegames/maestro/api/auth"
	"net/http"
	"net/http/httptest"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/topfreegames/extensions/v9/middleware"
	. "github.com/topfreegames/maestro/api"
	. "github.com/topfreegames/maestro/testing"
)

var _ = Describe("AuthMiddleware", func() {
	var server *ghttp.Server
	var authMiddleware *AuthMiddleware
	var request *http.Request
	var recorder *httptest.ResponseRecorder
	var dummyMiddleware = &DummyMiddleware{}

	BeforeEach(func() {
		server = ghttp.NewServer()
		config.Set("william.url", server.URL())
		config.Set("william.region", "us")
		config.Set("william.iamName", "maestro")
		config.Set("william.checkTimeout", "1s")

		var err error
		app, err = NewApp("0.0.0.0", 9998, config, logger, false, "", mockDb, mockCtxWrapper, mockRedisClient, mockRedisTraceWrapper, clientset, metricsClientset, mockSchedulerEventStorage)
		Expect(err).NotTo(HaveOccurred())

		authMiddleware = NewAuthMiddleware(app, auth.ActionResolver("SomePermission"))
		authMiddleware.SetNext(dummyMiddleware)

		request, _ = http.NewRequest("GET", "/scheduler/{schedulerName}/any/route", nil)
		request = request.WithContext(middleware.SetLogger(request.Context(), logger))

		recorder = httptest.NewRecorder()
	})

	AfterEach(func() {
		server.Close()
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

		It("should ok if authorized on william", func() {
			config.Set("william.enabled", "true")
			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)

			request = mux.SetURLVars(request, map[string]string{"schedulerName": "scheduler-name"})
			request.Header.Add("Authorization", "Bearer token")

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/permissions/has", "permission=maestro::RL::SomePermission::us"),
					ghttp.VerifyHeader(http.Header{
						"Authorization": []string{"Bearer token"},
					}),
					ghttp.RespondWith(http.StatusOK, nil),
				),
			)

			authMiddleware.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("should return unauthorized if not authorized on william", func() {
			config.Set("william.enabled", "true")
			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)

			request = mux.SetURLVars(request, map[string]string{"schedulerName": "scheduler-name"})
			request.Header.Add("Authorization", "Bearer token")

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/permissions/has", "permission=maestro::RL::SomePermission::us"),
					ghttp.VerifyHeader(http.Header{
						"Authorization": []string{"Bearer token"},
					}),
					ghttp.RespondWith(http.StatusForbidden, nil),
				),
			)

			authMiddleware.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusForbidden))

			var responseMap map[string]string
			err := json.Unmarshal(recorder.Body.Bytes(), &responseMap)
			Expect(err).ToNot(HaveOccurred())
			Expect(responseMap).To(HaveKeyWithValue("code", "MAE-006"))
		})
	})
})
