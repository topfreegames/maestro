// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package william_test

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/topfreegames/extensions/pg/mocks"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/william"
	"net/http"
)

var (
	listSchedulersPermission = william.Permission{
		Action:           "ListSchedulers",
		IncludeGame:      true,
		IncludeScheduler: false,
	}
	createSchedulerPermission = william.Permission{
		Action:           "CreateScheduler",
		IncludeGame:      false,
		IncludeScheduler: false,
	}
	updateSchedulerPermission = william.Permission{
		Action:           "UpdateScheduler",
		IncludeGame:      true,
		IncludeScheduler: true,
	}
	permissions = []william.Permission{
		listSchedulersPermission,
		createSchedulerPermission,
		updateSchedulerPermission,
	}
	schedulers = []models.Scheduler{
		{Name: "game1-green", Game: "game1"},
		{Name: "game1-blue", Game: "game1"},
	}
)

func mockSchedulers(mockDb *mocks.MockDB, schedulers []models.Scheduler) *gomock.Call {
	return mockDb.EXPECT().
		Query(gomock.Any(), "SELECT * FROM schedulers").
		Do(func(scheds *[]models.Scheduler, _ string) {
			*scheds = schedulers
		})
}

var _ = Describe("WillimAuth", func() {
	Describe("Permissions", func() {
		It("should return empty when initialized with no permissions", func() {
			auth := william.NewWilliamAuth(config, []william.Permission{})
			Expect(auth.Permissions(mockDb, "")).To(BeNil())
		})

		It("should return all actions when prefix is empty", func() {
			auth := william.NewWilliamAuth(config, permissions)
			mockSchedulers(mockDb, schedulers).Times(1)

			Expect(auth.Permissions(mockDb, "")).
				To(Equal([]william.IAMPermission{
					{Prefix: "*", Complete: false},
					{Prefix: "ListSchedulers", Complete: false},
					{Prefix: "CreateScheduler", Complete: false},
					{Prefix: "UpdateScheduler", Complete: false},
				}))
		})

		It("should return subset of actions when prefix matches", func() {
			auth := william.NewWilliamAuth(config, permissions)
			mockSchedulers(mockDb, schedulers).Times(1)

			Expect(auth.Permissions(mockDb, "List")).
				To(Equal([]william.IAMPermission{
					{Prefix: "ListSchedulers", Complete: false},
				}))
		})

		It("should return empty when prefix does not match", func() {
			auth := william.NewWilliamAuth(config, permissions)
			mockSchedulers(mockDb, schedulers).Times(1)

			Expect(auth.Permissions(mockDb, "abcd")).To(BeEmpty())
		})

		It(`should return {action}::{region} when prefix is {action}::`, func() {
			auth := william.NewWilliamAuth(config, permissions)
			mockSchedulers(mockDb, schedulers).Times(3)

			Expect(auth.Permissions(mockDb, "ListSchedulers::")).
				To(Equal([]william.IAMPermission{
					{Prefix: "ListSchedulers::*", Complete: true},
					{Prefix: "ListSchedulers::us", Complete: false},
				}))

			Expect(auth.Permissions(mockDb, "CreateScheduler::")).
				To(Equal([]william.IAMPermission{
					{Prefix: "CreateScheduler::*", Complete: true},
					{Prefix: "CreateScheduler::us", Complete: true},
				}))

			Expect(auth.Permissions(mockDb, "UpdateScheduler::")).
				To(Equal([]william.IAMPermission{
					{Prefix: "UpdateScheduler::*", Complete: true},
					{Prefix: "UpdateScheduler::us", Complete: false},
				}))
		})

		It(`should return {action}::{region}::{game} when permission supports it and prefix is {action}::{region}::`, func() {
			auth := william.NewWilliamAuth(config, permissions)
			mockSchedulers(mockDb, schedulers).Times(2)

			Expect(auth.Permissions(mockDb, "ListSchedulers::us::")).
				To(Equal([]william.IAMPermission{
					{Prefix: "ListSchedulers::us::*", Complete: true},
					{Prefix: "ListSchedulers::us::game1", Complete: true},
				}))

			Expect(auth.Permissions(mockDb, "UpdateScheduler::us::")).
				To(Equal([]william.IAMPermission{
					{Prefix: "UpdateScheduler::us::*", Complete: true},
					{Prefix: "UpdateScheduler::us::game1", Complete: false},
				}))
		})

		It(`should return {action}::{region}::{game}::{scheduler} when permission supports it and prefix is {action}::{region}::{game}::`, func() {
			auth := william.NewWilliamAuth(config, permissions)
			mockSchedulers(mockDb, schedulers).Times(1)

			Expect(auth.Permissions(mockDb, "UpdateScheduler::us::game1::")).
				To(Equal([]william.IAMPermission{
					{Prefix: "UpdateScheduler::us::game1::*", Complete: true},
					{Prefix: "UpdateScheduler::us::game1::game1-green", Complete: true},
					{Prefix: "UpdateScheduler::us::game1::game1-blue", Complete: true},
				}))
		})

		It(`should return empty when prefix has more parts than the permission supports`, func() {
			auth := william.NewWilliamAuth(config, permissions)
			mockSchedulers(mockDb, schedulers).Times(2)

			Expect(auth.Permissions(mockDb, "CreateScheduler::us::")).To(BeEmpty())
			Expect(auth.Permissions(mockDb, "ListSchedulers::us::game1::")).To(BeEmpty())
		})
	})

	Describe("Check", func() {
		var server *ghttp.Server
		var auth *william.WilliamAuth

		BeforeEach(func() {
			server = ghttp.NewServer()
			config.Set("william.url", server.URL())
			auth = william.NewWilliamAuth(config, permissions)
		})

		AfterEach(func() {
			server.Close()
			config.Set("william.url", "localhost")
		})

		It("should return no error when response status is 200", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/permissions/has", "permission=maestro::RL::CreateScheduler::us"),
					ghttp.VerifyHeader(http.Header{
						"Authorization": []string{"Bearer token"},
					}),
					ghttp.RespondWith(http.StatusOK, nil),
				),
			)

			Expect(auth.Check("token", "CreateScheduler", "")).ToNot(HaveOccurred())
		})

		It("should return AuthError when response status is 403", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/permissions/has", "permission=maestro::RL::UpdateScheduler::us::game1::game1-green"),
					ghttp.VerifyHeader(http.Header{
						"Authorization": []string{"Bearer token"},
					}),
					ghttp.RespondWith(http.StatusForbidden, nil),
				),
			)

			err := auth.Check("token", "UpdateScheduler", "game1::game1-green")
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&errors.AuthError{}))
		})

		It("should return AccessError when response status is different from 200 or 403", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/permissions/has", "permission=maestro::RL::UpdateScheduler::us::game1::game1-green"),
					ghttp.VerifyHeader(http.Header{
						"Authorization": []string{"Bearer token"},
					}),
					ghttp.RespondWith(http.StatusInternalServerError, nil),
				),
			)

			err := auth.Check("token", "UpdateScheduler", "game1::game1-green")
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&errors.AccessError{}))
		})
	})
})
