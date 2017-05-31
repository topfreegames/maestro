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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"gopkg.in/pg.v5/types"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

	BeforeEach(func() {
		// Record HTTP responses.
		recorder = httptest.NewRecorder()
	})

	Describe("POST /scheduler", func() {
		url := "/scheduler"
		BeforeEach(func() {
			err := json.Unmarshal([]byte(jsonString), &payload)
			Expect(err).NotTo(HaveOccurred())
			reader := JSONFor(payload)
			request, _ = http.NewRequest("POST", url, reader)
		})

		Context("when all services are healthy", func() {
			It("returns a status code of 201 and success body", func() {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(100)

				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				).Times(100)
				mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey("scheduler-name"), gomock.Any()).Times(100)
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

	Describe("PUT /scheduler/{schedulerName}", func() {
		newJsonString := `{
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
		      },
		      {
		        "name": "NEW_ENV_VAR",
		        "value": "newvalue"
		      }
		    ],
		    "cmd": [
		      "./room-binary",
		      "-serverType",
		      "6a8e136b-2dc1-417e-bbe8-0f0a2d2df431"
		    ]
		  }`

		Context("when all services are healthy", func() {
			It("returns a status code of 200 and success body", func() {
				// Create scheduler
				reader := strings.NewReader(jsonString)
				url := "/scheduler"
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				var configYaml1 models.ConfigYAML
				err = yaml.Unmarshal([]byte(jsonString), &configYaml1)
				Expect(err).NotTo(HaveOccurred())

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				).Times(configYaml1.AutoScaling.Min)
				mockPipeline.EXPECT().
					ZAdd(models.GetRoomPingRedisKey("scheduler-name"), gomock.Any()).
					Times(configYaml1.AutoScaling.Min)
				mockPipeline.EXPECT().
					SAdd(models.GetRoomStatusSetRedisKey("scheduler-name", "creating"), gomock.Any()).
					Times(configYaml1.AutoScaling.Min)
				mockPipeline.EXPECT().
					Exec().
					Times(configYaml1.AutoScaling.Min)
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
				Expect(recorder.Code).To(Equal(http.StatusCreated))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

				svcs, err := clientset.CoreV1().Services(configYaml1.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(svcs.Items).To(HaveLen(configYaml1.AutoScaling.Min))

				// Update scheduler
				var configYaml2 models.ConfigYAML
				err = yaml.Unmarshal([]byte(newJsonString), &configYaml2)
				Expect(err).NotTo(HaveOccurred())

				reader = strings.NewReader(newJsonString)
				url = fmt.Sprintf("/scheduler/%s", configYaml1.Name)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())

				mockRedisClient.EXPECT().Ping().AnyTimes()

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, jsonString)
					})

				mockRedisClient.EXPECT().
					SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMS)*time.Millisecond).
					Return(redis.NewBoolResult(true, nil))

				for _, svc := range svcs.Items {
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
					room := models.NewRoom(svc.GetName(), svc.GetNamespace())
					for _, status := range allStatus {
						mockPipeline.EXPECT().
							SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
					}
					mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(svc.GetNamespace()), room.ID)
					mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
					mockPipeline.EXPECT().Exec()
				}
				// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(configYaml1.AutoScaling.Min)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				).Times(configYaml1.AutoScaling.Min)
				mockPipeline.EXPECT().
					ZAdd(models.GetRoomPingRedisKey(configYaml1.Name), gomock.Any()).
					Times(configYaml1.AutoScaling.Min)
				mockPipeline.EXPECT().
					SAdd(models.GetRoomStatusSetRedisKey(configYaml1.Name, "creating"), gomock.Any()).
					Times(configYaml1.AutoScaling.Min)
				mockPipeline.EXPECT().Exec().Times(configYaml1.AutoScaling.Min)

				mockDb.EXPECT().
					Query(gomock.Any(), "UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id", gomock.Any())

				mockRedisClient.EXPECT().
					Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
					Return(redis.NewCmdResult(nil, nil))

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("should return 400 if updating nonexisting scheduler", func() {
				var configYaml1 models.ConfigYAML
				err := yaml.Unmarshal([]byte(jsonString), &configYaml1)
				Expect(err).NotTo(HaveOccurred())

				reader := strings.NewReader(newJsonString)
				url := fmt.Sprintf("/scheduler/%s", configYaml1.Name)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())

				mockRedisClient.EXPECT().Ping().AnyTimes()

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))
				var obj map[string]interface{}
				err = json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(Equal("scheduler scheduler-name not found, create it first"))
				Expect(obj["success"]).To(Equal(false))
			})

			It("should return 400 if names doesn't match", func() {

				var configYaml1 models.ConfigYAML
				err := yaml.Unmarshal([]byte(jsonString), &configYaml1)
				Expect(err).NotTo(HaveOccurred())

				reader := strings.NewReader(newJsonString)
				url := "/scheduler/some-other-name"
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
				var obj map[string]interface{}
				err = json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(Equal("url name some-other-name doesn't match payload name scheduler-name"))
				Expect(obj["success"]).To(Equal(false))
			})
		})

		Context("when postgres is down", func() {
			It("returns status code of 500 if database is unavailable", func() {
				var configYaml1 models.ConfigYAML
				err := yaml.Unmarshal([]byte(jsonString), &configYaml1)
				Expect(err).NotTo(HaveOccurred())

				reader := strings.NewReader(newJsonString)
				url := fmt.Sprintf("/scheduler/%s", configYaml1.Name)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())

				mockRedisClient.EXPECT().Ping().AnyTimes()
				mockDb.EXPECT().Query(
					gomock.Any(),
					"SELECT * FROM schedulers WHERE name = ?",
					configYaml1.Name,
				).Return(&types.Result{}, errors.New("sql: database is closed"))

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				var obj map[string]interface{}
				err = json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-001"))
				Expect(obj["error"]).To(Equal("DatabaseError"))
				Expect(obj["description"]).To(Equal("sql: database is closed"))
				Expect(obj["success"]).To(Equal(false))
			})
		})
	})

	Describe("GET /scheduler/{schedulerName}", func() {
		It("should return infos about scheduler and room", func() {
			var configYaml models.ConfigYAML
			err := json.Unmarshal([]byte(jsonString), &configYaml)
			Expect(err).NotTo(HaveOccurred())

			schedulerName := configYaml.Name

			for i := 0; i < configYaml.AutoScaling.Min; i++ {
				room := models.NewRoom(fmt.Sprintf("room-%d", i), schedulerName)

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any())
				mockPipeline.EXPECT().SAdd(gomock.Any(), gomock.Any())
				mockPipeline.EXPECT().ZAdd(gomock.Any(), gomock.Any())
				mockPipeline.EXPECT().Exec()

				room.Create(mockRedisClient)
			}
			url := fmt.Sprintf("http://%s/scheduler/scheduler-name", app.Address)
			request, err := http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					*scheduler = *models.NewScheduler(configYaml.Name, configYaml.Game, jsonString)
				})
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().
				SCard(models.GetRoomStatusSetRedisKey(schedulerName, models.StatusCreating)).
				Return(redis.NewIntResult(int64(0), nil))
			mockPipeline.EXPECT().
				SCard(models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady)).
				Return(redis.NewIntResult(int64(configYaml.AutoScaling.Min), nil))
			mockPipeline.EXPECT().
				SCard(models.GetRoomStatusSetRedisKey(schedulerName, models.StatusOccupied)).
				Return(redis.NewIntResult(int64(0), nil))
			mockPipeline.EXPECT().
				SCard(models.GetRoomStatusSetRedisKey(schedulerName, models.StatusTerminating)).
				Return(redis.NewIntResult(int64(0), nil))
			mockPipeline.EXPECT().Exec()

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusOK))

			resp := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(HaveKeyWithValue("game", configYaml.Game))
			Expect(resp).To(HaveKeyWithValue("state", models.StatusCreating))
			Expect(resp).To(HaveKeyWithValue("lastScaleOpAt", float64(0)))
			Expect(resp).To(HaveKeyWithValue("roomsAtCreating", float64(0)))
			Expect(resp).To(HaveKeyWithValue("roomsAtOccupied", float64(0)))
			Expect(resp).To(HaveKeyWithValue("roomsAtReady", float64(configYaml.AutoScaling.Min)))
			Expect(resp).To(HaveKeyWithValue("roomsAtTerminating", float64(0)))
			Expect(resp["stateLastChangedAt"]).To(BeNumerically("~", time.Now().Unix(), 3))
		})

		It("should return error if scheduler doesn't exist", func() {
			name := "other-scheduler-name"
			url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, name)
			request, err := http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", name).
				Do(func(scheduler *models.Scheduler, query string, modifier string) {
					scheduler.YAML = ""
				})

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusNotFound))
			var obj map[string]interface{}
			err = json.Unmarshal([]byte(recorder.Body.String()), &obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj["code"]).To(Equal("MAE-004"))
			Expect(obj["error"]).To(Equal("ValidationFailedError"))
			Expect(obj["description"]).To(Equal("scheduler \"other-scheduler-name\" not found"))
			Expect(obj["success"]).To(Equal(false))
		})

		It("should return error if db fails", func() {
			name := "other-scheduler-name"
			url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, name)
			request, err := http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())

			mockDb.EXPECT().
				Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", name).
				Return(nil, errors.New("some db error"))

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			var obj map[string]interface{}
			err = json.Unmarshal([]byte(recorder.Body.String()), &obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj["code"]).To(Equal("MAE-000"))
			Expect(obj["error"]).To(Equal("Status scheduler failed"))
			Expect(obj["description"]).To(Equal("some db error"))
			Expect(obj["success"]).To(Equal(false))
		})
	})
})
