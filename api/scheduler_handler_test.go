// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"gopkg.in/pg.v5/types"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/models"
	. "github.com/topfreegames/maestro/testing"
)

var _ = Describe("Scheduler Handler", func() {
	var request *http.Request
	var recorder *httptest.ResponseRecorder
	var payload JSON
	var jsonString string

	BeforeEach(func() {
		// Record HTTP responses.
		recorder = httptest.NewRecorder()
	})

	Describe("POST /scheduler", func() {
		url := "/scheduler"
		BeforeEach(func() {
			jsonString = `{
		    "name": "scheduler-name",
		    "game": "game-name",
		    "image": "somens/someimage:v123",
		    "ports": [
		      {
		        "containerPort": 5050,
		        "protocol": "UDP",
		        "name": "port1"
		      },
		      {
		        "containerPort": 8888,
		        "protocol": "TCP",
		        "name": "port2"
		      }
		    ],
		    "limits": {
		      "memory": "128Mi",
		      "cpu": "1"
		    },
		    "shutdownTimeout": 180,
		    "autoscaling": {
		      "min": 100,
		      "up": {
		        "delta": 10,
		        "trigger": {
		          "usage": 70,
		          "time": 600
		        },
		        "cooldown": 300
		      },
		      "down": {
		        "delta": 2,
		        "trigger": {
		          "usage": 50,
		          "time": 900
		        },
		        "cooldown": 300
		      }
		    },
		    "env": [
		      {
		        "name": "EXAMPLE_ENV_VAR",
		        "value": "examplevalue"
		      },
		      {
		        "name": "ANOTHER_ENV_VAR",
		        "value": "anothervalue"
		      }
		    ],
		    "cmd": [
		      "./room-binary",
		      "-serverType",
		      "6a8e136b-2dc1-417e-bbe8-0f0a2d2df431"
		    ]
		  }`
			err := json.Unmarshal([]byte(jsonString), &payload)
			Expect(err).NotTo(HaveOccurred())
			reader := JSONFor(payload)
			request, _ = http.NewRequest("POST", url, reader)
		})

		Context("when all services are healthy", func() {
			It("returns a status code of 201 and success body", func() {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(100)
				mockPipeline.EXPECT().HMSet(gomock.Any(), map[string]interface{}{
					"status":   "creating",
					"lastPing": int64(0),
				}).Times(100)
				mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey("scheduler-name", "creating"), gomock.Any()).Times(100)
				mockPipeline.EXPECT().Exec().Times(100)
				mockDb.EXPECT().Query(
					gomock.Any(),
					"INSERT INTO schedulers (name, game, yaml, state, state_last_changed_at) VALUES (?name, ?game, ?yaml, ?state, ?state_last_changed_at) RETURNING id",
					gomock.Any(),
				)
				mockDb.EXPECT().Query(
					gomock.Any(),
					"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
					gomock.Any(),
				)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(201))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			})
		})

		Context("missing payload argument", func() {
			args := []string{"name", "game", "image", "autoscaling"}
			for _, arg := range args {
				It(fmt.Sprintf("returns status code of 422 if missing %s", arg), func() {
					delete(payload, arg)
					reader := JSONFor(payload)
					request, _ = http.NewRequest("POST", url, reader)

					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(422))
					var obj map[string]interface{}
					err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
					Expect(err).NotTo(HaveOccurred())
					Expect(obj["code"]).To(Equal("MAE-004"))
					Expect(obj["error"]).To(Equal("ValidationFailedError"))
					Expect(strings.ToLower(obj["description"].(string))).To(ContainSubstring(fmt.Sprintf("%s: non zero value required", arg)))
					Expect(obj["success"]).To(Equal(false))
				})
			}
		})

		Context("invalid payload argument", func() {
			It("returns status code of 422 if invalid ShutdownTimeout", func() {
				payload["shutdownTimeout"] = "not-an-int"
				reader := JSONFor(payload)
				request, _ = http.NewRequest("POST", url, reader)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(422))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(ContainSubstring("ConfigYAML.shutdownTimeout"))
				Expect(obj["success"]).To(Equal(false))
			})
		})

		Context("when postgres is down", func() {
			It("returns status code of 500 if database is unavailable", func() {
				mockDb.EXPECT().Query(
					gomock.Any(),
					"INSERT INTO schedulers (name, game, yaml, state, state_last_changed_at) VALUES (?name, ?game, ?yaml, ?state, ?state_last_changed_at) RETURNING id",
					gomock.Any(),
				).Return(&types.Result{}, errors.New("sql: database is closed"))
				mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", gomock.Any())

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-000"))
				Expect(obj["error"]).To(Equal("Create scheduler failed"))
				Expect(obj["description"]).To(Equal("sql: database is closed"))
				Expect(obj["success"]).To(Equal(false))
			})
		})
	})

	Describe("DELETE /scheduler/{schedulerName}", func() {
		url := "/scheduler/schedulerName"
		BeforeEach(func() {
			request, _ = http.NewRequest("DELETE", url, nil)
		})

		Context("when all services are healthy", func() {
			It("returns a status code of 200 and success body", func() {
				mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", "schedulerName").Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = jsonString
				})
				mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", "schedulerName")

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			})
		})

		Context("when postgres is down", func() {
			It("returns status code of 500 if database is unavailable", func() {
				mockDb.EXPECT().Query(
					gomock.Any(),
					"SELECT * FROM schedulers WHERE name = ?",
					"schedulerName",
				).Return(&types.Result{}, errors.New("sql: database is closed"))

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-000"))
				Expect(obj["error"]).To(Equal("Delete scheduler failed"))
				Expect(obj["description"]).To(Equal("sql: database is closed"))
				Expect(obj["success"]).To(Equal(false))
			})
		})
	})
})
