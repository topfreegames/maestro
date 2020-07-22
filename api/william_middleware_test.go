// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/onsi/gomega/ghttp"
	"github.com/topfreegames/maestro/models"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/topfreegames/extensions/middleware"
	. "github.com/topfreegames/maestro/api"
	. "github.com/topfreegames/maestro/testing"
)

var _ = Describe("WilliamMiddleware", func() {
	var williamMiddleware *WilliamMiddleware
	var request *http.Request
	var recorder *httptest.ResponseRecorder
	var server *ghttp.Server
	var dummyMiddleware = &DummyMiddleware{}

	BeforeEach(func() {
		server = ghttp.NewServer()
		config.Set("william.enabled", true)

		var err error
		app, err = NewApp("0.0.0.0", 9998, config, logger, false, "", mockDb, mockCtxWrapper, mockRedisClient, mockRedisTraceWrapper, clientset, metricsClientset)
		Expect(err).NotTo(HaveOccurred())

		request, _ = http.NewRequest("GET", "/scheduler/{schedulerName}/any/route", nil)
		request = request.WithContext(middleware.SetLogger(request.Context(), logger))

		recorder = httptest.NewRecorder()
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("ServeHTTP", func() {
		It("should return ok if not enabled", func() {
			app.Config.Set("william.enabled", false)

			williamMiddleware = NewWilliamMiddleware(app, ActionResolver("ListSchedulers"))
			williamMiddleware.SetNext(dummyMiddleware)

			williamMiddleware.ServeHTTP(recorder, request)
			response := recorder.Result()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("should return unauthorized if no token is passed", func() {
			app.Config.Set("william.enabled", true)

			williamMiddleware = NewWilliamMiddleware(app, ActionResolver("ListSchedulers"))
			williamMiddleware.SetNext(dummyMiddleware)

			williamMiddleware.ServeHTTP(recorder, request)
			response := recorder.Result()
			Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should ok if token authorized to scheduler", func() {
			app.Config.Set("william.enabled", true)

			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/permissions/has", "permission=maestro::RL::UpdateScheduler::us::game::scheduler-name"),
				ghttp.VerifyHeader(http.Header{
					"Authorization": []string{"Bearer token"},
				}),
				ghttp.RespondWith(http.StatusOK, nil),
			))

			request = mux.SetURLVars(request, map[string]string{"schedulerName": "scheduler-name"})
			request.Header.Add("Authorization", "Bearer token")

			williamMiddleware = NewWilliamMiddleware(app, SchedulerPathResolver("UpdateScheduler", "schedulerName"))
			williamMiddleware.SetNext(dummyMiddleware)

			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)
			MockLoadScheduler("scheduler-name", mockDb).Do(func(s *models.Scheduler, query, modifier string) {
				s.Name = "scheduler-name"
				s.Game = "game"
			})

			williamMiddleware.ServeHTTP(recorder, request)
			response := recorder.Result()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("should return forbidden if not authorized to scheduler", func() {
			app.Config.Set("william.enabled", true)

			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/permissions/has", "permission=maestro::RL::UpdateScheduler::us::game::scheduler-name"),
				ghttp.VerifyHeader(http.Header{
					"Authorization": []string{"Bearer token"},
				}),
				ghttp.RespondWith(http.StatusForbidden, nil),
			))

			request = mux.SetURLVars(request, map[string]string{"schedulerName": "scheduler-name"})
			request.Header.Add("Authorization", "Bearer token")

			williamMiddleware = NewWilliamMiddleware(app, SchedulerPathResolver("UpdateScheduler", "schedulerName"))
			williamMiddleware.SetNext(dummyMiddleware)

			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)
			MockLoadScheduler("scheduler-name", mockDb).Do(func(s *models.Scheduler, query, modifier string) {
				s.Name = "scheduler-name"
				s.Game = "game"
			})

			williamMiddleware.ServeHTTP(recorder, request)
			response := recorder.Result()
			Expect(response.StatusCode).To(Equal(http.StatusForbidden))
		})
	})
})
