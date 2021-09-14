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
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/topfreegames/maestro/login"
	"github.com/topfreegames/maestro/models"
)

var _ = Describe("Scheduler Event Handler", func() {
	var (
		recorder   *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		recorder = httptest.NewRecorder()
	})

	Context("When authentication is ok", func() {
		BeforeEach(func() {
			mockDb.EXPECT().Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, gomock.Any()).
				Do(func(destToken *login.DestinationToken, query string, modifier string) {
					destToken.RefreshToken = "refresh-token"
				}).AnyTimes()
			mockLogin.EXPECT().Authenticate(gomock.Any(), app.DBClient.DB).Return("user@example.com", http.StatusOK, nil).AnyTimes()
			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB).AnyTimes()
		})

		Describe("GET /schedulers/:name/events", func() {
			It("should list scheduler events", func() {
				page := 1
				schedulerName := "scheduler-name-1"
				url := fmt.Sprintf("http://%s/scheduler/%s/events?page=%d", app.Address, schedulerName, page)
				request, err := http.NewRequest("GET", url, nil)
				Expect(err).NotTo(HaveOccurred())

				createdAtString := "2020-01-01T00:00:00.000Z"
				createdAt, err := time.Parse(time.RFC3339Nano, createdAtString)
				mockSchedulerEventStorage.EXPECT().LoadSchedulerEvents(schedulerName, page).Return([]*models.SchedulerEvent{
					{
						Name: "UPDATE_STARTED",
						SchedulerName: schedulerName,
						CreatedAt: createdAt,
						Metadata: map[string]interface{}{
							"reason": "update",
						},
					},
				}, nil)
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))

				resp := make([]map[string]interface{}, 1)
				err = json.Unmarshal(recorder.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp[0]).To(HaveKeyWithValue("name", "UPDATE_STARTED"))
				Expect(resp[0]).To(HaveKeyWithValue("schedulerName", "scheduler-name-1"))
				Expect(resp[0]).To(HaveKeyWithValue("createdAt", "2020-01-01T00:00:00Z"))
				Expect(resp[0]).To(HaveKeyWithValue("metadata", map[string]interface{}{
					"reason": "update",
				}))
			})

			It("should return bad request when page is not informed", func() {
				schedulerName := "scheduler-name-1"
				url := fmt.Sprintf("http://%s/scheduler/%s/events", app.Address, schedulerName)
				request, err := http.NewRequest("GET", url, nil)
				Expect(err).NotTo(HaveOccurred())

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			})

			It("should return bad request when page is not an integer", func() {
				page := "not_an_integer"
				schedulerName := "scheduler-name-1"
				url := fmt.Sprintf("http://%s/scheduler/%s/events?page=%s", app.Address, schedulerName, page)
				request, err := http.NewRequest("GET", url, nil)
				Expect(err).NotTo(HaveOccurred())

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			})

			It("should return internal server error when storage fails to retrieve data", func() {
				page := 1
				schedulerName := "scheduler-name-1"
				url := fmt.Sprintf("http://%s/scheduler/%s/events?page=%d", app.Address, schedulerName, page)
				request, err := http.NewRequest("GET", url, nil)
				Expect(err).NotTo(HaveOccurred())

				mockSchedulerEventStorage.EXPECT().LoadSchedulerEvents(schedulerName, page).Return(nil, errors.New("Failed to retrieve events"))

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})
})
