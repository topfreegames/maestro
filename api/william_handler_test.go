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
	"github.com/golang/mock/gomock"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/william"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/topfreegames/extensions/middleware"
	. "github.com/topfreegames/maestro/api"
)

var _ = Describe("WilliamHandler", func() {
	var request *http.Request
	var recorder *httptest.ResponseRecorder

	BeforeEach(func() {
		config.Set("william.enabled", true)

		var err error
		app, err = NewApp("0.0.0.0", 9998, config, logger, false, "", mockDb, mockCtxWrapper, mockRedisClient, mockRedisTraceWrapper, clientset, metricsClientset)
		Expect(err).NotTo(HaveOccurred())

		request, _ = http.NewRequest("GET", "/am", nil)
		request = request.WithContext(middleware.SetLogger(request.Context(), logger))

		recorder = httptest.NewRecorder()
	})

	Describe("ServeHTTP", func() {
		It("should return permissions", func() {
			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)
			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers").
				Do(func(scheds *[]models.Scheduler, _ string) {
					*scheds = []models.Scheduler{}
				})

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			var permissions []william.IAMPermission
			err := json.Unmarshal(recorder.Body.Bytes(), &permissions)
			Expect(err).ToNot(HaveOccurred())

			Expect(permissions).To(Equal([]william.IAMPermission{
				{Prefix: "ListSchedulers", Complete: false},
				{Prefix: "GetScheduler", Complete: false},
				{Prefix: "CreateScheduler", Complete: false},
				{Prefix: "UpdateScheduler", Complete: false},
				{Prefix: "ScaleScheduler", Complete: false},
				{Prefix: "DeleteScheduler", Complete: false},
			}))
		})
	})
})
