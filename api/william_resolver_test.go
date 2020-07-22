// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"errors"
	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/extensions/pg"
	"github.com/topfreegames/maestro/models"
	"net/http"

	. "github.com/topfreegames/maestro/api"
	. "github.com/topfreegames/maestro/testing"
)

var _ = Describe("William Resolvers", func() {
	var request *http.Request

	BeforeEach(func() {
		request, _ = http.NewRequest("GET", "/scheduler/{schedulerName}/any/route", nil)
	})

	Describe("ActionResolver", func() {
		It("should always return action", func() {
			permission, resource, err := ActionResolver("ListSchedulers").ResolvePermission(nil, nil)
			Expect(permission).To(Equal("ListSchedulers"))
			Expect(resource).To(BeEmpty())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("SchedulerPathResolver", func() {
		It("should return correct resource if scheduler exists", func() {
			request = mux.SetURLVars(request, map[string]string{"schedulerName": "scheduler-name"})

			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)
			MockLoadScheduler("scheduler-name", mockDb).Do(func(s *models.Scheduler, query, modifier string) {
				s.Name = "scheduler-name"
				s.Game = "game"
			})

			permission, resource, err := SchedulerPathResolver("UpdateScheduler", "schedulerName").ResolvePermission(app, request)
			Expect(permission).To(Equal("UpdateScheduler"))
			Expect(resource).To(Equal("game::scheduler-name"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return error if var does not exist", func() {
			request = mux.SetURLVars(request, map[string]string{"schedulerName2": "scheduler-name"})

			_, _, err := SchedulerPathResolver("UpdateScheduler", "schedulerName").ResolvePermission(app, request)
			Expect(err).To(HaveOccurred())
		})

		It("should return error if fails to find scheduler", func() {
			request = mux.SetURLVars(request, map[string]string{"schedulerName": "scheduler-name"})

			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB)
			MockLoadScheduler("scheduler-name", mockDb).Do(func(s *models.Scheduler, query, modifier string) {
				s.Name = "scheduler-name"
				s.Game = "game"
			}).Return(pg.NewTestResult(nil, 1), errors.New("no rows"))

			_, _, err := SchedulerPathResolver("UpdateScheduler", "schedulerName").ResolvePermission(app, request)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("GameQueryResolver", func() {
		It("should return correct resource if game param is set", func() {
			query := request.URL.Query()
			query.Add("game", "game")
			request.URL.RawQuery = query.Encode()

			permission, resource, err := GameQueryResolver("ListSchedulers", "game").ResolvePermission(app, request)
			Expect(permission).To(Equal("ListSchedulers"))
			Expect(resource).To(Equal("game"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return wildcard resource if game param is not set", func() {
			permission, resource, err := GameQueryResolver("ListSchedulers", "game").ResolvePermission(app, request)
			Expect(permission).To(Equal("ListSchedulers"))
			Expect(resource).To(Equal("*"))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
