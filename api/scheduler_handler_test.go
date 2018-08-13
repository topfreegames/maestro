// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/topfreegames/maestro/testing"

	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	goredis "github.com/go-redis/redis"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	"github.com/topfreegames/extensions/pg"
	"github.com/topfreegames/maestro/api"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/login"
	"github.com/topfreegames/maestro/models"
	"k8s.io/api/core/v1"
)

var _ = Describe("Scheduler Handler", func() {
	var (
		request    *http.Request
		recorder   *httptest.ResponseRecorder
		payload    JSON
		yamlString string
		scheduler1 *models.Scheduler
		lockKeyNs  string
		opManager  *models.OperationManager

		timeoutSec       = 300
		timeoutDur       = time.Duration(timeoutSec) * time.Second
		portStart        = 5000
		portEnd          = 6000
		workerPortRange  = models.NewPortRange(portStart, portEnd).String()
		numberOfVersions = 1
	)

	yamlString = `{
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
  ],
  "forwarders": {
    "mockplugin": {
      "mockfwd": {
        "enabled": true,
        "medatada": {
          "send": "me"
        }
      }
    }
  }
}`

	BeforeEach(func() {
		// Record HTTP responses.
		recorder = httptest.NewRecorder()
		node := &v1.Node{}
		node.SetName("node-name")
		node.SetLabels(map[string]string{
			"game": "controller",
		})
		_, err := clientset.CoreV1().Nodes().Create(node)
		Expect(err).NotTo(HaveOccurred())

		lockKeyNs = fmt.Sprintf("%s-scheduler-name", lockKey)

		mockRedisClient.EXPECT().Ping().AnyTimes()
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

		Describe("GET /scheduler", func() {
			It("should list schedulers", func() {
				var configYaml models.ConfigYAML
				expectedNames := []string{"scheduler1", "scheduler2", "scheduler3"}
				err := json.Unmarshal([]byte(yamlString), &configYaml)
				Expect(err).NotTo(HaveOccurred())

				url := fmt.Sprintf("http://%s/scheduler", app.Address)
				request, err := http.NewRequest("GET", url, nil)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT name FROM schedulers").Do(
					func(schedulers *[]models.Scheduler, query string) {
						expectedSchedulers := make([]models.Scheduler, len(expectedNames))
						for idx, name := range expectedNames {
							expectedSchedulers[idx] = models.Scheduler{Name: name}
						}
						*schedulers = expectedSchedulers
					})
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"schedulers":["scheduler1","scheduler2","scheduler3"]}`))
			})

			It("should list none if no schedulers", func() {
				var configYaml models.ConfigYAML
				expectedNames := []string{}
				err := json.Unmarshal([]byte(yamlString), &configYaml)
				Expect(err).NotTo(HaveOccurred())

				url := fmt.Sprintf("http://%s/scheduler", app.Address)
				request, err := http.NewRequest("GET", url, nil)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT name FROM schedulers").Do(
					func(schedulers *[]models.Scheduler, query string) {
						expectedSchedulers := make([]models.Scheduler, len(expectedNames))
						for idx, name := range expectedNames {
							expectedSchedulers[idx] = models.Scheduler{Name: name}
						}
						*schedulers = expectedSchedulers
					})
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"schedulers":[]}`))
			})

			It("should return error if db fails to list schedulers", func() {
				var configYaml models.ConfigYAML
				err := json.Unmarshal([]byte(yamlString), &configYaml)
				Expect(err).NotTo(HaveOccurred())

				url := fmt.Sprintf("http://%s/scheduler", app.Address)
				request, err := http.NewRequest("GET", url, nil)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT name FROM schedulers").
					Return(nil, errors.New("db error"))

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				var obj map[string]interface{}
				err = json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-000"))
				Expect(obj["error"]).To(Equal("List scheduler failed"))
				Expect(strings.ToLower(obj["description"].(string))).To(Equal("db error"))
				Expect(obj["success"]).To(Equal(false))
			})
		})

		Describe("GET /scheduler?info", func() {
			var request *http.Request

			BeforeEach(func() {
				var err error
				url := fmt.Sprintf("http://%s/scheduler?info", app.Address)
				request, err = http.NewRequest("GET", url, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			expectationsGivenNames := func(names []string) {
				var configYaml models.ConfigYAML
				err := json.Unmarshal([]byte(yamlString), &configYaml)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT name FROM schedulers").Do(
					func(schedulers *[]models.Scheduler, query string) {
						expectedSchedulers := make([]models.Scheduler, len(names))
						for idx, name := range names {
							expectedSchedulers[idx] = models.Scheduler{Name: name}
						}
						*schedulers = expectedSchedulers
					})

				if len(names) == 0 {
					return
				}

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name IN (?)", gomock.Any()).
					Do(func(schedulers *[]models.Scheduler, query string, _ interface{}) {
						expectedSchedulers := make([]models.Scheduler, len(names))
						for idx, name := range names {
							expectedSchedulers[idx] = models.Scheduler{
								Name:  name,
								Game:  configYaml.Game,
								YAML:  yamlString,
								State: models.StateInSync,
							}
						}
						*schedulers = expectedSchedulers
					})
			}

			redisExpectations := func(f func()) {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				mockRedisClient.EXPECT().Ping().AnyTimes()
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				f()
				mockPipeline.EXPECT().Exec()
			}

			It("should list schedulers with infos", func() {
				expectedNames := []string{"scheduler1"}
				expectationsGivenNames(expectedNames)

				redisExpectations(func() {
					Creating := models.GetRoomStatusSetRedisKey(expectedNames[0], models.StatusCreating)
					Ready := models.GetRoomStatusSetRedisKey(expectedNames[0], models.StatusReady)
					Occupied := models.GetRoomStatusSetRedisKey(expectedNames[0], models.StatusOccupied)
					Terminating := models.GetRoomStatusSetRedisKey(expectedNames[0], models.StatusTerminating)
					mockPipeline.EXPECT().SCard(Creating).Return(redis.NewIntResult(int64(2), nil))
					mockPipeline.EXPECT().SCard(Ready).Return(redis.NewIntResult(int64(1), nil))
					mockPipeline.EXPECT().SCard(Occupied).Return(redis.NewIntResult(int64(1), nil))
					mockPipeline.EXPECT().SCard(Terminating).Return(redis.NewIntResult(int64(0), nil))
				})

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`[{"autoscalingDownTriggerUsage":50,"autoscalingMin":100,"autoscalingUpTriggerUsage":70,"game":"game-name","name":"scheduler1","roomsCreating":2,"roomsOccupied":1,"roomsReady":1,"roomsTerminating":0,"state":"in-sync"}]`))
			})

			It("should list empty array when there aren't schedulers", func() {
				expectedNames := []string{}
				expectationsGivenNames(expectedNames)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal("[]"))
			})
		})

		Describe("POST /scheduler", func() {
			url := "/scheduler"
			BeforeEach(func() {
				err := json.Unmarshal([]byte(yamlString), &payload)
				Expect(err).NotTo(HaveOccurred())
				reader := JSONFor(payload)
				request, _ = http.NewRequest("POST", url, reader)
			})

			Context("when all services are healthy", func() {
				It("returns a status code of 201 and success body", func() {
					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
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
					MockInsertScheduler(mockDb, nil)
					MockUpdateSchedulerStatus(mockDb, nil, nil)

					mockRedisClient.EXPECT().
						Get(models.GlobalPortsPoolKey).
						Return(goredis.NewStringResult(workerPortRange, nil)).
						Times(100)

					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(http.StatusCreated))
					Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				})

				It("creates multiple schedulers", func() {
					yamlString := `
---
name: scheduler-name-1
game: game-name
image: somens/someimage:v123
ports:
- containerPort: 5050
  protocol: UDP
  name: port1
shutdownTimeout: 180
autoscaling:
  min: 1
  up:
    delta: 10
    trigger:
      usage: 70
      time: 600
    cooldown: 300
  down:
    delta: 2
    trigger:
      usage: 50
      time: 900
    cooldown: 300
---
name: scheduler-name-2
game: game-name
image: somens/someimage:v123
ports:
- containerPort: 5050
  protocol: UDP
  name: port1
shutdownTimeout: 180
autoscaling:
  min: 1
  up:
    delta: 10
    trigger:
      usage: 70
      time: 600
    cooldown: 300
  down:
    delta: 2
    trigger:
      usage: 50
      time: 900
    cooldown: 300
`
					reader := strings.NewReader(yamlString)
					request, err := http.NewRequest("POST", url, reader)
					Expect(err).NotTo(HaveOccurred())

					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
					mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
						func(schedulerName string, statusInfo map[string]interface{}) {
							Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
							Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
						},
					).Times(2)
					mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey("scheduler-name-1"), gomock.Any())
					mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey("scheduler-name-2"), gomock.Any())
					mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey("scheduler-name-1", "creating"), gomock.Any())
					mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey("scheduler-name-2", "creating"), gomock.Any())
					mockPipeline.EXPECT().Exec().Times(2)

					MockInsertScheduler(mockDb, nil)
					MockUpdateSchedulerStatus(mockDb, nil, nil)
					mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
						Return(goredis.NewStringResult(workerPortRange, nil))

					MockInsertScheduler(mockDb, nil)
					MockUpdateSchedulerStatus(mockDb, nil, nil)
					mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
						Return(goredis.NewStringResult(workerPortRange, nil))

					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
					Expect(recorder.Code).To(Equal(http.StatusCreated))
				})

				It("should return 409 if namespace already exists", func() {
					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
					ns := models.NewNamespace("scheduler-name")
					err := ns.Create(clientset)
					Expect(err).NotTo(HaveOccurred())

					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(http.StatusConflict))
					var obj map[string]interface{}
					err = json.Unmarshal([]byte(recorder.Body.String()), &obj)
					Expect(err).NotTo(HaveOccurred())
					Expect(obj["code"]).To(Equal("MAE-000"))
					Expect(obj["error"]).To(Equal("Create scheduler failed"))
					Expect(strings.ToLower(obj["description"].(string))).To(Equal("namespace \"scheduler-name\" already exists"))
					Expect(obj["success"]).To(Equal(false))
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
					Expect(obj["description"]).To(ContainSubstring(`cannot unmarshal`))
					Expect(obj["success"]).To(Equal(false))
				})
			})

			Context("when postgres is down", func() {
				It("returns status code of 500 if database is unavailable", func() {
					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
					MockInsertScheduler(mockDb, errors.New("sql: database is closed"))
					mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", gomock.Any())

					app.Router.ServeHTTP(recorder, request)
					var obj map[string]interface{}
					err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
					Expect(err).NotTo(HaveOccurred())
					Expect(obj["code"]).To(Equal("MAE-000"))
					Expect(obj["error"]).To(Equal("Create scheduler failed"))
					Expect(obj["description"]).To(Equal("sql: database is closed"))
					Expect(obj["success"]).To(Equal(false))

					Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when there is no nodes with affinity label", func() {
				It("should return error code 422", func() {
					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
					MockInsertScheduler(mockDb, errors.New("node without label error"))

					mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", gomock.Any())

					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(http.StatusUnprocessableEntity))
					var obj map[string]interface{}
					err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
					Expect(err).NotTo(HaveOccurred())
					Expect(obj["code"]).To(Equal("MAE-000"))
					Expect(obj["error"]).To(Equal("Create scheduler failed"))
					Expect(obj["description"]).To(Equal("node without label error"))
					Expect(obj["success"]).To(Equal(false))
				})
			})

			Context("with eventforwarders", func() {
				BeforeEach(func() {
					app.Forwarders = []*eventforwarder.Info{
						&eventforwarder.Info{
							Plugin:    "mockplugin",
							Name:      "mockfwd",
							Forwarder: mockEventForwarder1,
						},
					}
				})

				It("forwards scheduler event", func() {
					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
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
						"SELECT * FROM schedulers WHERE name = ?",
						"scheduler-name",
					).Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlString
						scheduler.Game = "game-name"
					})

					mockEventForwarder1.EXPECT().Forward(gomock.Any(), "schedulerEvent", gomock.Any(), gomock.Any())

					MockInsertScheduler(mockDb, nil)
					MockUpdateSchedulerStatus(mockDb, nil, nil)
					mockRedisClient.EXPECT().Get(models.GlobalPortsPoolKey).
						Return(goredis.NewStringResult(workerPortRange, nil)).Times(100)

					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(201))
					Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
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
					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
					mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", "schedulerName").Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlString
					})
					mockDb.EXPECT().Exec("DELETE FROM schedulers WHERE name = ?", "schedulerName")

					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(200))
					Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				})

				It("should return 404 if scheduler is not found", func() {
					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
					mockDb.EXPECT().
						Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", "schedulerName").
						Do(func(scheduler *models.Scheduler, query string, modifier string) {
							scheduler.YAML = ""
						})

					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(404))
					var obj map[string]interface{}
					err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
					Expect(err).NotTo(HaveOccurred())
					Expect(obj["code"]).To(Equal("MAE-004"))
					Expect(obj["error"]).To(Equal("ValidationFailedError"))
					Expect(obj["description"]).To(Equal("scheduler \"schedulerName\" not found"))
					Expect(obj["success"]).To(Equal(false))
				})
			})

			Context("when postgres is down", func() {
				It("returns status code of 500 if database is unavailable", func() {
					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
					mockDb.EXPECT().Query(
						gomock.Any(),
						"SELECT * FROM schedulers WHERE name = ?",
						"schedulerName",
					).Return(pg.NewTestResult(errors.New("sql: database is closed"), 0), errors.New("sql: database is closed"))

					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
					var obj map[string]interface{}
					err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
					Expect(err).NotTo(HaveOccurred())
					Expect(obj["code"]).To(Equal("MAE-000"))
					Expect(obj["error"]).To(Equal("delete scheduler failed"))
					Expect(obj["description"]).To(Equal("sql: database is closed"))
					Expect(obj["success"]).To(Equal(false))
				})
			})
		})

		Describe("PUT /scheduler/{schedulerName}", func() {
			var configYaml models.ConfigYAML
			var url string
			newJSONString := `{
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
					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
					MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb,
						logger, app.RoomManager, mmr, yamlString, timeoutSec, nil, workerPortRange, portStart, portEnd)

					err := yaml.Unmarshal([]byte(yamlString), &configYaml)
					Expect(err).NotTo(HaveOccurred())
					scheduler1 := models.NewScheduler(configYaml.Name, configYaml.Game, yamlString)

					opManager = models.NewOperationManager(configYaml.Name, mockRedisClient, logger)
					MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

					pods, err := clientset.CoreV1().Pods(configYaml.Name).List(metav1.ListOptions{})
					Expect(err).NotTo(HaveOccurred())
					Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

					// Update scheduler
					var configYaml2 models.ConfigYAML
					err = yaml.Unmarshal([]byte(newJSONString), &configYaml2)
					Expect(err).NotTo(HaveOccurred())

					reader := strings.NewReader(newJSONString)
					url = fmt.Sprintf("/scheduler/%s", configYaml.Name)
					request, err = http.NewRequest("PUT", url, reader)
					Expect(err).NotTo(HaveOccurred())

					// Select current scheduler yaml
					MockSelectScheduler(yamlString, mockDb, nil)

					// Get redis lock
					MockRedisLock(mockRedisClient, lockKeyNs, lockTimeoutMs, true, nil)

					// Set new operation manager description
					MockAnySetDescription(opManager, mockRedisClient, "running", nil)

					// Remove old rooms
					MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml)

					// Create new roome
					// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
					MockCreateRooms(mockRedisClient, mockPipeline, &configYaml)
					MockGetPortsFromPool(&configYaml, mockRedisClient, nil, workerPortRange, portStart, portEnd)

					// Update new config on schedulers table
					MockUpdateSchedulersTable(mockDb, nil)

					// Add new version into versions table
					scheduler1.NextMajorVersion()
					MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

					// Count to delete old versions if necessary
					MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

					// Retrieve redis lock
					MockReturnRedisLock(mockRedisClient, lockKeyNs, nil)

					recorder = httptest.NewRecorder()
					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
					Expect(recorder.Code).To(Equal(http.StatusOK))
				})

				It("should return 200 and update up time and not delete pods", func() {
					// Create scheduler
					yamlString1 := `
name: scheduler-name
game: game
image: image:v1
autoscaling:
  min: 1
  up:
    delta: 10
    trigger:
      usage: 70
      time: 600
    cooldown: 300
  down:
    delta: 2
    trigger:
      usage: 50
      time: 900
    cooldown: 300
`
					reader := strings.NewReader(yamlString1)
					url := "/scheduler"
					request, err := http.NewRequest("POST", url, reader)
					Expect(err).NotTo(HaveOccurred())

					var configYaml1 models.ConfigYAML
					err = yaml.Unmarshal([]byte(yamlString1), &configYaml1)
					Expect(err).NotTo(HaveOccurred())

					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
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

					MockInsertScheduler(mockDb, nil)
					MockUpdateSchedulerStatus(mockDb, nil, nil)

					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
					Expect(recorder.Code).To(Equal(http.StatusCreated))

					pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
					Expect(err).NotTo(HaveOccurred())
					Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

					// Update scheduler
					yamlString2 := `
name: scheduler-name
game: game
image: image:v1
autoscaling:
  min: 1
  up:
    delta: 10
    trigger:
      usage: 70
      time: 300
    cooldown: 300
  down:
    delta: 2
    trigger:
      usage: 50
      time: 900
    cooldown: 300
`
					var configYaml2 models.ConfigYAML
					err = yaml.Unmarshal([]byte(yamlString2), &configYaml2)
					Expect(err).NotTo(HaveOccurred())

					reader = strings.NewReader(yamlString2)
					url = fmt.Sprintf("/scheduler/%s", configYaml2.Name)
					request, err = http.NewRequest("PUT", url, reader)
					Expect(err).NotTo(HaveOccurred())

					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
					opManager = models.NewOperationManager(configYaml2.Name, mockRedisClient, logger)
					MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

					mockRedisClient.EXPECT().Ping().AnyTimes()

					mockDb.EXPECT().
						Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
						Do(func(scheduler *models.Scheduler, query string, modifier string) {
							*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString1)
						})

					lockKeyNs := fmt.Sprintf("%s-%s", lockKey, configYaml1.Name)

					mockRedisClient.EXPECT().
						SetNX(lockKeyNs, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
						Return(redis.NewBoolResult(true, nil))

					// Set new operation manager description
					MockAnySetDescription(opManager, mockRedisClient, "running", nil)

					scheduler1 := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)
					// Update new config on schedulers table
					MockUpdateSchedulersTable(mockDb, nil)
					// Add new version into versions table
					scheduler1.NextMinorVersion()
					MockInsertIntoVersionsTable(scheduler1, mockDb, nil)
					// Count to delete old versions if necessary
					MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

					mockRedisClient.EXPECT().
						Eval(gomock.Any(), []string{lockKeyNs}, gomock.Any()).
						Return(redis.NewCmdResult(nil, nil))

					recorder = httptest.NewRecorder()
					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
					Expect(recorder.Code).To(Equal(http.StatusOK))
				})

				It("should return 400 if updating nonexisting scheduler", func() {
					var configYaml1 models.ConfigYAML
					err := yaml.Unmarshal([]byte(yamlString), &configYaml1)
					Expect(err).NotTo(HaveOccurred())

					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
					opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
					MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

					reader := strings.NewReader(newJSONString)
					url := fmt.Sprintf("/scheduler/%s", configYaml1.Name)
					request, err = http.NewRequest("PUT", url, reader)
					Expect(err).NotTo(HaveOccurred())

					mockRedisClient.EXPECT().Ping().AnyTimes()

					lockKeyNs := fmt.Sprintf("%s-%s", lockKey, configYaml1.Name)
					mockRedisClient.EXPECT().
						SetNX(lockKeyNs, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
						Return(redis.NewBoolResult(true, nil))

					// Set new operation manager description
					MockAnySetDescription(opManager, mockRedisClient, "running", nil)

					mockDb.EXPECT().
						Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name)
					mockRedisClient.EXPECT().
						Eval(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(redis.NewCmdResult(nil, nil))

					recorder = httptest.NewRecorder()
					app.Router.ServeHTTP(recorder, request)
					var obj map[string]interface{}
					err = json.Unmarshal([]byte(recorder.Body.String()), &obj)
					Expect(err).NotTo(HaveOccurred())
					Expect(obj["code"]).To(Equal("MAE-004"))
					Expect(obj["error"]).To(Equal("ValidationFailedError"))
					Expect(obj["description"]).To(Equal("scheduler scheduler-name not found, create it first"))
					Expect(obj["success"]).To(Equal(false))

					Expect(recorder.Code).To(Equal(http.StatusNotFound))
				})

				It("should return 400 if names doesn't match", func() {
					var configYaml1 models.ConfigYAML
					err := yaml.Unmarshal([]byte(yamlString), &configYaml1)
					Expect(err).NotTo(HaveOccurred())

					reader := strings.NewReader(newJSONString)
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

				It("should return 422 if there is no nodes with affinity label", func() {
					var configYaml1 models.ConfigYAML
					err := yaml.Unmarshal([]byte(yamlString), &configYaml1)
					Expect(err).NotTo(HaveOccurred())

					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
					opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
					MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

					reader := strings.NewReader(newJSONString)
					url := fmt.Sprintf("/scheduler/%s", configYaml1.Name)
					request, err = http.NewRequest("PUT", url, reader)
					Expect(err).NotTo(HaveOccurred())

					mockRedisClient.EXPECT().Ping().AnyTimes()

					mockRedisClient.EXPECT().
						SetNX(gomock.Any(), gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
						Return(redis.NewBoolResult(true, nil))

					// Set new operation manager description
					MockAnySetDescription(opManager, mockRedisClient, "running", nil)

					mockRedisClient.EXPECT().
						Eval(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(redis.NewCmdResult(nil, nil))

					mockDb.EXPECT().
						Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
						//HACK!!! DB won't return this error, Kubernetes will
						Return(nil, errors.New("node without label error"))

					recorder = httptest.NewRecorder()
					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(http.StatusUnprocessableEntity))
					var obj map[string]interface{}
					err = json.Unmarshal([]byte(recorder.Body.String()), &obj)
					Expect(err).NotTo(HaveOccurred())
					Expect(obj["description"]).To(Equal("node without label error"))
					Expect(obj["success"]).To(Equal(false))
				})

				It("should asynchronously update scheduler", func() {
					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
					MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb,
						logger, app.RoomManager, mmr, yamlString, timeoutSec, nil, workerPortRange, portStart, portEnd)

					err := yaml.Unmarshal([]byte(yamlString), &configYaml)
					Expect(err).NotTo(HaveOccurred())
					scheduler1 := models.NewScheduler(configYaml.Name, configYaml.Game, yamlString)

					opManager = models.NewOperationManager(configYaml.Name, mockRedisClient, logger)
					MockGetCurrentOperationKey(opManager, mockRedisClient, nil)

					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
					mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
						Expect(m).To(HaveKeyWithValue("operation", "UpdateSchedulerConfig"))
					})
					mockPipeline.EXPECT().Expire(gomock.Any(), timeoutDur)
					mockPipeline.EXPECT().Set(opManager.BuildCurrOpKey(), gomock.Any(), timeoutDur)
					mockPipeline.EXPECT().Exec()

					mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(map[string]string{
						"not": "empty",
					}, nil)).AnyTimes()

					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
					mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
						Expect(m).To(HaveKeyWithValue("status", http.StatusOK))
						Expect(m).To(HaveKeyWithValue("operation", "UpdateSchedulerConfig"))
						Expect(m).To(HaveKeyWithValue("success", true))
					})
					mockPipeline.EXPECT().Expire(gomock.Any(), 10*time.Minute)
					mockPipeline.EXPECT().Del(opManager.BuildCurrOpKey())
					mockPipeline.EXPECT().Exec().Do(func() {
						opManager.StopLoop()
					})

					pods, err := clientset.CoreV1().Pods(configYaml.Name).List(metav1.ListOptions{})
					Expect(err).NotTo(HaveOccurred())
					Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

					// Update scheduler
					var configYaml2 models.ConfigYAML
					err = yaml.Unmarshal([]byte(newJSONString), &configYaml2)
					Expect(err).NotTo(HaveOccurred())

					reader := strings.NewReader(newJSONString)
					url = fmt.Sprintf("/scheduler/%s?async=true", configYaml.Name)
					request, err = http.NewRequest("PUT", url, reader)
					Expect(err).NotTo(HaveOccurred())

					// Select current scheduler yaml
					MockSelectScheduler(yamlString, mockDb, nil)

					// Get redis lock
					MockRedisLock(mockRedisClient, lockKeyNs, lockTimeoutMs, true, nil)

					// Set new operation manager description
					MockAnySetDescription(opManager, mockRedisClient, "running", nil)

					// Remove old rooms
					MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml)

					// Create new roome
					// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
					MockCreateRooms(mockRedisClient, mockPipeline, &configYaml)
					MockGetPortsFromPool(&configYaml, mockRedisClient, nil, workerPortRange, portStart, portEnd)

					// Update new config on schedulers table
					MockUpdateSchedulersTable(mockDb, nil)

					// Add new version into versions table
					scheduler1.NextMajorVersion()
					MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

					// Count to delete old versions if necessary
					MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

					// Retrieve redis lock
					MockReturnRedisLock(mockRedisClient, lockKeyNs, nil)

					recorder = httptest.NewRecorder()
					app.Router.ServeHTTP(recorder, request)
					var response map[string]interface{}
					json.Unmarshal(recorder.Body.Bytes(), &response)
					Expect(response).To(HaveKeyWithValue("success", true))
					Expect(response).To(HaveKey("operationKey"))

					Eventually(opManager.IsStopped, 1*time.Minute).Should(BeTrue())
				})

				It("should asynchronously update scheduler and show error when occurred", func() {
					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
					MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb,
						logger, app.RoomManager, mmr, yamlString, timeoutSec, nil, workerPortRange, portStart, portEnd)

					err := yaml.Unmarshal([]byte(yamlString), &configYaml)
					Expect(err).NotTo(HaveOccurred())

					opManager = models.NewOperationManager(configYaml.Name, mockRedisClient, logger)
					MockGetCurrentOperationKey(opManager, mockRedisClient, nil)

					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
					mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
						Expect(m).To(HaveKeyWithValue("operation", "UpdateSchedulerConfig"))
					})
					mockPipeline.EXPECT().Expire(gomock.Any(), timeoutDur)
					mockPipeline.EXPECT().Set(opManager.BuildCurrOpKey(), gomock.Any(), timeoutDur)
					mockPipeline.EXPECT().Exec()

					mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(map[string]string{
						"not": "empty",
					}, nil)).AnyTimes()

					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
					mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
						Expect(m).To(HaveKeyWithValue("status", http.StatusInternalServerError))
						Expect(m).To(HaveKeyWithValue("operation", "UpdateSchedulerConfig"))
						Expect(m).To(HaveKeyWithValue("success", false))
						Expect(m).To(HaveKeyWithValue("description", "update scheduler failed"))
						Expect(m).To(HaveKeyWithValue("error", "error to update scheduler on schedulers table: err on db"))
					})
					mockPipeline.EXPECT().Expire(gomock.Any(), 10*time.Minute)
					mockPipeline.EXPECT().Del(opManager.BuildCurrOpKey())
					mockPipeline.EXPECT().Exec().Do(func() {
						opManager.StopLoop()
					})

					pods, err := clientset.CoreV1().Pods(configYaml.Name).List(metav1.ListOptions{})
					Expect(err).NotTo(HaveOccurred())
					Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

					// Update scheduler
					var configYaml2 models.ConfigYAML
					err = yaml.Unmarshal([]byte(newJSONString), &configYaml2)
					Expect(err).NotTo(HaveOccurred())

					reader := strings.NewReader(newJSONString)
					url = fmt.Sprintf("/scheduler/%s?async=true", configYaml.Name)
					request, err = http.NewRequest("PUT", url, reader)
					Expect(err).NotTo(HaveOccurred())

					// Select current scheduler yaml
					MockSelectScheduler(yamlString, mockDb, nil)

					// Get redis lock
					MockRedisLock(mockRedisClient, lockKeyNs, lockTimeoutMs, true, nil)

					// Set new operation manager description
					MockAnySetDescription(opManager, mockRedisClient, "running", nil)

					// Remove old rooms
					MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml)

					// Create new roome
					// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
					MockCreateRooms(mockRedisClient, mockPipeline, &configYaml)
					MockGetPortsFromPool(&configYaml, mockRedisClient, nil, workerPortRange, portStart, portEnd)

					// Update new config on schedulers table
					MockUpdateSchedulersTable(mockDb, errors.New("err on db"))

					// // Add new version into versions table
					// scheduler1.NextMajorVersion()
					// MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

					// // Count to delete old versions if necessary
					// MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

					// Retrieve redis lock
					MockReturnRedisLock(mockRedisClient, lockKeyNs, nil)

					recorder = httptest.NewRecorder()
					app.Router.ServeHTTP(recorder, request)
					var response map[string]interface{}
					json.Unmarshal(recorder.Body.Bytes(), &response)
					Expect(response).To(HaveKeyWithValue("success", true))
					Expect(response).To(HaveKey("operationKey"))

					Eventually(opManager.IsStopped, 1*time.Minute).Should(BeTrue())
				})
			})

			Context("when postgres is down", func() {
				It("returns status code of 500 if database is unavailable", func() {
					var configYaml1 models.ConfigYAML
					err := yaml.Unmarshal([]byte(yamlString), &configYaml1)
					Expect(err).NotTo(HaveOccurred())

					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
					opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
					MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

					reader := strings.NewReader(newJSONString)
					url := fmt.Sprintf("/scheduler/%s", configYaml1.Name)
					request, err = http.NewRequest("PUT", url, reader)
					Expect(err).NotTo(HaveOccurred())

					mockRedisClient.EXPECT().Ping().AnyTimes()
					mockRedisClient.EXPECT().
						SetNX(gomock.Any(), gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
						Return(redis.NewBoolResult(true, nil))

					// Set new operation manager description
					MockAnySetDescription(opManager, mockRedisClient, "running", nil)

					mockDb.EXPECT().Query(
						gomock.Any(),
						"SELECT * FROM schedulers WHERE name = ?",
						configYaml1.Name,
					).Return(pg.NewTestResult(errors.New("sql: database is closed"), 0), errors.New("sql: database is closed"))
					lockKeyNs := fmt.Sprintf("%s-%s", lockKey, configYaml1.Name)
					mockRedisClient.EXPECT().
						Eval(gomock.Any(), []string{lockKeyNs}, gomock.Any()).
						Return(redis.NewCmdResult(nil, nil))

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

			Context("with eventforwarders", func() {
				BeforeEach(func() {
					app.Forwarders = []*eventforwarder.Info{
						&eventforwarder.Info{
							Plugin:    "mockplugin",
							Name:      "mockfwd",
							Forwarder: mockEventForwarder1,
						},
						&eventforwarder.Info{
							Plugin:    "mockplugin",
							Name:      "anothermockfwd",
							Forwarder: mockEventForwarder2,
						},
					}
				})

				It("forwards scheduler event", func() {
					// Create scheduler
					yamlString1 := `
name: scheduler-name
game: game
image: image:v1
autoscaling:
  min: 1
  up:
    delta: 10
    trigger:
      usage: 70
      time: 600
    cooldown: 300
  down:
    delta: 2
    trigger:
      usage: 50
      time: 900
    cooldown: 300
`
					reader := strings.NewReader(yamlString1)
					url := "/scheduler"
					request, err := http.NewRequest("POST", url, reader)
					Expect(err).NotTo(HaveOccurred())

					var configYaml1 models.ConfigYAML
					err = yaml.Unmarshal([]byte(yamlString1), &configYaml1)
					Expect(err).NotTo(HaveOccurred())

					mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
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

					MockInsertScheduler(mockDb, nil)
					MockUpdateSchedulerStatus(mockDb, nil, nil)

					mockDb.EXPECT().Query(
						gomock.Any(),
						"SELECT * FROM schedulers WHERE name = ?",
						"scheduler-name",
					).Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlString1
						scheduler.Game = "game"
					})

					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(http.StatusCreated))
					Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

					pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
					Expect(err).NotTo(HaveOccurred())
					Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

					// Update scheduler
					yamlString2 := `
name: scheduler-name
game: game
image: image:v1
autoscaling:
  min: 1
  up:
    delta: 10
    trigger:
      usage: 70
      time: 300
    cooldown: 300
  down:
    delta: 2
    trigger:
      usage: 50
      time: 900
    cooldown: 300
forwarders:
  mockplugin:
    mockfwd:
      enabled: true
      metadata:
        data: "to be forwarded"
        intField: 123
    anothermockfwd:
      enabled: true
      metadata:
        data: "newData"
        newInt: 987
`
					var configYaml2 models.ConfigYAML
					err = yaml.Unmarshal([]byte(yamlString2), &configYaml2)
					Expect(err).NotTo(HaveOccurred())

					reader = strings.NewReader(yamlString2)
					url = fmt.Sprintf("/scheduler/%s", configYaml2.Name)
					request, err = http.NewRequest("PUT", url, reader)
					Expect(err).NotTo(HaveOccurred())

					opManager = models.NewOperationManager(configYaml2.Name, mockRedisClient, logger)
					MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

					mockRedisClient.EXPECT().Ping().AnyTimes()

					mockDb.EXPECT().
						Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml2.Name).
						Do(func(scheduler *models.Scheduler, query string, modifier string) {
							*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString1)
						})

					lockKeyNs := fmt.Sprintf("%s-%s", lockKey, configYaml1.Name)

					mockRedisClient.EXPECT().
						SetNX(lockKeyNs, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
						Return(redis.NewBoolResult(true, nil))

					// Set new operation manager description
					MockAnySetDescription(opManager, mockRedisClient, "running", nil)

					scheduler1 := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)
					// Update new config on schedulers table
					MockUpdateSchedulersTable(mockDb, nil)
					// Add new version into versions table
					scheduler1.NextMinorVersion()
					MockInsertIntoVersionsTable(scheduler1, mockDb, nil)
					// Count to delete old versions if necessary
					MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

					mockRedisClient.EXPECT().
						Eval(gomock.Any(), []string{lockKeyNs}, gomock.Any()).
						Return(redis.NewCmdResult(nil, nil))

					mockDb.EXPECT().Query(
						gomock.Any(),
						"SELECT * FROM schedulers WHERE name = ?",
						"scheduler-name",
					).Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlString2
						scheduler.Game = "game"
					})

					mockEventForwarder1.EXPECT().Forward(gomock.Any(), "schedulerEvent", gomock.Any(), gomock.Any())
					mockEventForwarder2.EXPECT().Forward(gomock.Any(), "schedulerEvent", gomock.Any(), gomock.Any())

					recorder = httptest.NewRecorder()
					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
					Expect(recorder.Code).To(Equal(http.StatusOK))
				})
			})
		})

		Describe("GET /scheduler/{schedulerName}", func() {
			It("should return infos about scheduler and room", func() {
				var configYaml models.ConfigYAML
				err := json.Unmarshal([]byte(yamlString), &configYaml)
				Expect(err).NotTo(HaveOccurred())

				schedulerName := configYaml.Name

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				for i := 0; i < configYaml.AutoScaling.Min; i++ {
					room := models.NewRoom(fmt.Sprintf("room-%d", i), schedulerName)

					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
					mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any())
					mockPipeline.EXPECT().SAdd(gomock.Any(), gomock.Any())
					mockPipeline.EXPECT().ZAdd(gomock.Any(), gomock.Any())
					mockPipeline.EXPECT().Exec()

					room.Create(mockRedisClient, mockDb, mmr, &configYaml)
				}
				url := fmt.Sprintf("http://%s/scheduler/scheduler-name", app.Address)
				request, err := http.NewRequest("GET", url, nil)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml.Name, configYaml.Game, yamlString)
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
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
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
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
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

		Describe("GET /scheduler/{schedulerName}?config", func() {
			yamlStr := `
name: scheduler-name
game: game-name
`
			schedulerName := "scheduler-name"

			It("should return yaml config", func() {
				url := fmt.Sprintf("http://%s/scheduler/%s?config", app.Address, schedulerName)
				request, err := http.NewRequest("GET", url, nil)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT yaml FROM schedulers WHERE name = ?", schedulerName).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
					})

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))

				resp := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).To(HaveKeyWithValue("yaml", `name: scheduler-name
game: game-name
shutdownTimeout: 0
autoscaling: null
affinity: ""
toleration: ""
occupiedTimeout: 0
forwarders: {}
authorizedUsers: []
portRange: null
image: ""
imagePullPolicy: ""
ports: []
limits: null
requests: null
env: []
cmd: []
`))
			})

			It("should return yaml config for two container pods", func() {
				yamlStr := `
name: scheduler-name
game: game-name
containers:
- image: image/image
  name: container1
  ports:
  - containerPort: 8080
    protocol: TCP
    name: tcp
`
				url := fmt.Sprintf("http://%s/scheduler/%s?config", app.Address, schedulerName)
				request, err := http.NewRequest("GET", url, nil)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT yaml FROM schedulers WHERE name = ?", schedulerName).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
					})

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))

				resp := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).To(HaveKeyWithValue("yaml", `name: scheduler-name
game: game-name
shutdownTimeout: 0
autoscaling: null
affinity: ""
toleration: ""
occupiedTimeout: 0
forwarders: {}
authorizedUsers: []
containers:
- name: container1
  image: image/image
  imagePullPolicy: ""
  ports:
  - containerPort: 8080
    protocol: TCP
    name: tcp
  limits: null
  requests: null
  env: []
  cmd: []
portRange: null
`))
			})

			It("should return 500 if db fails", func() {
				url := fmt.Sprintf("http://%s/scheduler/%s?config", app.Address, schedulerName)
				request, err := http.NewRequest("GET", url, nil)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT yaml FROM schedulers WHERE name = ?", schedulerName).
					Return(nil, errors.New("db error"))

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))

				resp := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).To(HaveKeyWithValue("code", "MAE-000"))
				Expect(resp).To(HaveKeyWithValue("description", "db error"))
				Expect(resp).To(HaveKeyWithValue("error", "config scheduler failed"))
				Expect(resp).To(HaveKeyWithValue("success", false))
			})

			It("should return 404 if scheduler doesn't exists", func() {
				url := fmt.Sprintf("http://%s/scheduler/%s?config", app.Address, schedulerName)
				request, err := http.NewRequest("GET", url, nil)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT yaml FROM schedulers WHERE name = ?", schedulerName).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = ""
					})

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))

				resp := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &resp)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).To(HaveKeyWithValue("code", "MAE-000"))
				Expect(resp).To(HaveKeyWithValue("description", "config scheduler not found"))
				Expect(resp).To(HaveKeyWithValue("error", "get config error"))
				Expect(resp).To(HaveKeyWithValue("success", false))
			})
		})

		Describe("POST /scheduler/{schedulerName}", func() {
			var app *api.App
			schedulerName := "scheduler-name"
			yamlStr := `
name: scheduler-name
game: game-name
`

			BeforeEach(func() {
				config, err := GetDefaultConfig()
				Expect(err).NotTo(HaveOccurred())
				config.Set("basicauth.tryOauthIfUnset", true)
				app, err = api.NewApp("0.0.0.0", 9998, config, logger, false, false, "", mockDb, mockCtxWrapper, mockRedisClient, mockRedisTraceWrapper, clientset)
				Expect(err).NotTo(HaveOccurred())
				app.Login = mockLogin
			})

			It("should manually scale up", func() {
				body := map[string]interface{}{"scaleup": 1}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))

				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", schedulerName).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
					})
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				)
				mockPipeline.EXPECT().
					ZAdd(models.GetRoomPingRedisKey(schedulerName), gomock.Any())
				mockPipeline.EXPECT().
					SAdd(models.GetRoomStatusSetRedisKey(schedulerName, "creating"), gomock.Any())
				mockPipeline.EXPECT().Exec()

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("should return error if empty body on scale up", func() {
				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, nil)
				Expect(err).NotTo(HaveOccurred())

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))

				response := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response["error"]).To(Equal("ValidationFailedError"))
				Expect(response["description"]).To(Equal("empty body sent"))
				Expect(response["code"]).To(Equal("MAE-000"))
				Expect(response["success"]).To(Equal(false))
			})

			It("should manually scale down", func() {
				body := map[string]interface{}{"scaledown": 1}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))

				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", schedulerName).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
					})
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().
					SPop(models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady)).
					Return(redis.NewStringCmd("room-id"))
				mockPipeline.EXPECT().Exec()

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				room := models.NewRoom("room-id", schedulerName)
				for _, status := range allStatus {
					mockPipeline.EXPECT().
						SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), gomock.Any())
					mockPipeline.EXPECT().
						ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), gomock.Any())
				}
				mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(room.SchedulerName), gomock.Any())
				mockPipeline.EXPECT().Del(gomock.Any())
				mockPipeline.EXPECT().Exec()

				port := 5000
				pod := &v1.Pod{}
				pod.Spec.Containers = []v1.Container{
					{Ports: []v1.ContainerPort{
						{HostPort: int32(port), Name: "TCP"},
					}},
				}
				_, err = clientset.CoreV1().Pods(schedulerName).Create(pod)
				Expect(err).NotTo(HaveOccurred())

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			})

			It("should manually choose the number of replicas and scale up", func() {
				replicas := 5
				body := map[string]interface{}{"replicas": replicas}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))

				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", schedulerName).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
					})

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(replicas)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				).Times(replicas)
				mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey("scheduler-name"), gomock.Any()).Times(replicas)
				mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey("scheduler-name", "creating"), gomock.Any()).Times(replicas)
				mockPipeline.EXPECT().Exec().Times(replicas)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

				pods, err := clientset.CoreV1().Pods(schedulerName).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(replicas))
			})

			It("should return error if replicas is negative", func() {
				replicas := -1
				body := map[string]interface{}{"replicas": replicas}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))

				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusUnprocessableEntity))
				response := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response["error"]).To(Equal("ValidationFailedError"))
				Expect(response["description"]).To(ContainSubstring("yaml: unmarshal errors:"))
				Expect(response["code"]).To(Equal("MAE-004"))
				Expect(response["success"]).To(Equal(false))
			})

			It("should manually choose the number of replicas and scale down", func() {
				//scale up
				replicasBefore := 5
				body := map[string]interface{}{"replicas": replicasBefore}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))

				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", schedulerName).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
					})

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(replicasBefore)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				).Times(replicasBefore)
				mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey("scheduler-name"), gomock.Any()).Times(replicasBefore)
				mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey("scheduler-name", "creating"), gomock.Any()).Times(replicasBefore)
				mockPipeline.EXPECT().Exec().Times(replicasBefore)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

				pods, err := clientset.CoreV1().Pods(schedulerName).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(replicasBefore))

				//scale down
				replicasAfter := 2
				body = map[string]interface{}{"replicas": replicasAfter}
				bts, _ = json.Marshal(body)
				reader = strings.NewReader(string(bts))

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", schedulerName).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
					})

				names, err := controller.GetPodNames(replicasBefore-replicasAfter, schedulerName, clientset)
				Expect(err).NotTo(HaveOccurred())

				readyKey := models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				for _, name := range names {
					mockPipeline.EXPECT().SPop(readyKey).Return(redis.NewStringResult(name, nil))

				}
				mockPipeline.EXPECT().Exec()

				for _, name := range names {
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
					room := models.NewRoom(name, schedulerName)
					for _, status := range allStatus {
						mockPipeline.EXPECT().
							SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
						mockPipeline.EXPECT().
							ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
					}
					mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(schedulerName), room.ID)
					mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
					mockPipeline.EXPECT().Exec()
				}

				url = fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err = http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

				pods, err = clientset.CoreV1().Pods(schedulerName).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(replicasAfter))
			})

			It("should set to 0 replicas", func() {
				//scale up
				replicasBefore := 5
				body := map[string]interface{}{"replicas": replicasBefore}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))

				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", schedulerName).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
					})

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(replicasBefore)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
					func(schedulerName string, statusInfo map[string]interface{}) {
						Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
						Expect(statusInfo["lastPing"]).To(BeNumerically("~", time.Now().Unix(), 1))
					},
				).Times(replicasBefore)
				mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey("scheduler-name"), gomock.Any()).Times(replicasBefore)
				mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey("scheduler-name", "creating"), gomock.Any()).Times(replicasBefore)
				mockPipeline.EXPECT().Exec().Times(replicasBefore)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

				pods, err := clientset.CoreV1().Pods(schedulerName).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(replicasBefore))

				//scale down
				replicasAfter := 0
				body = map[string]interface{}{"replicas": replicasAfter}
				bts, _ = json.Marshal(body)
				reader = strings.NewReader(string(bts))

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", schedulerName).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
					})

				names, err := controller.GetPodNames(replicasBefore-replicasAfter, schedulerName, clientset)
				Expect(err).NotTo(HaveOccurred())

				readyKey := models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				for _, name := range names {
					mockPipeline.EXPECT().SPop(readyKey).Return(redis.NewStringResult(name, nil))

				}
				mockPipeline.EXPECT().Exec()

				for _, name := range names {
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
					room := models.NewRoom(name, schedulerName)
					for _, status := range allStatus {
						mockPipeline.EXPECT().
							SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
						mockPipeline.EXPECT().
							ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
					}
					mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(schedulerName), room.ID)
					mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
					mockPipeline.EXPECT().Exec()
				}

				url = fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err = http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

				pods, err = clientset.CoreV1().Pods(schedulerName).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(replicasAfter))
			})

			It("should return 400 if scaleup and scaledown are both specified", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				body := map[string]interface{}{"scaledown": 1, "scaleup": 1}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))

				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))

				response := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response["error"]).To(Equal("scale scheduler failed"))
				Expect(response["description"]).To(Equal("invalid scale parameter: can't handle more than one parameter"))
				Expect(response["code"]).To(Equal("MAE-000"))
				Expect(response["success"]).To(Equal(false))
			})

			It("should return 422 if scaleup is negative", func() {
				body := map[string]interface{}{"scaleup": -1}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))

				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusUnprocessableEntity))

				response := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response["error"]).To(Equal("ValidationFailedError"))
				Expect(response["description"]).To(ContainSubstring("yaml: unmarshal errors:"))
				Expect(response["code"]).To(Equal("MAE-004"))
				Expect(response["success"]).To(Equal(false))
			})

			It("should return 422 if scaleup is not a number", func() {
				body := map[string]interface{}{"scaleup": "qwe"}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))

				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusUnprocessableEntity))

				response := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response["error"]).To(Equal("ValidationFailedError"))
				Expect(response["description"]).To(ContainSubstring("yaml: unmarshal errors:"))
				Expect(response["code"]).To(Equal("MAE-004"))
				Expect(response["success"]).To(Equal(false))
			})

			It("should return 422 if scaledown is negative", func() {
				body := map[string]interface{}{"scaledown": -1}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))

				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusUnprocessableEntity))

				response := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response["error"]).To(Equal("ValidationFailedError"))
				Expect(response["description"]).To(ContainSubstring("yaml: unmarshal errors:"))
				Expect(response["code"]).To(Equal("MAE-004"))
				Expect(response["success"]).To(Equal(false))
			})

			It("should return 422 if scaledown is not a number", func() {
				body := map[string]interface{}{"scaledown": "qwe"}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))

				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusUnprocessableEntity))

				response := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response["error"]).To(Equal("ValidationFailedError"))
				Expect(response["description"]).To(ContainSubstring("yaml: unmarshal errors:"))
				Expect(response["code"]).To(Equal("MAE-004"))
				Expect(response["success"]).To(Equal(false))
			})

			It("should return 500 if DB fails", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				body := map[string]interface{}{"scaledown": 1}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))

				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", schedulerName).
					Return(nil, errors.New("database error"))

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))

				response := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response["error"]).To(Equal("DatabaseError"))
				Expect(response["description"]).To(Equal("database error"))
				Expect(response["code"]).To(Equal("MAE-001"))
				Expect(response["success"]).To(Equal(false))
			})

			It("should return 404 if scheduler does not exist", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				body := map[string]interface{}{"scaledown": 1}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))

				url := fmt.Sprintf("http://%s/scheduler/%s", app.Address, schedulerName)
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", schedulerName)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))

				response := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response["error"]).To(Equal("scale scheduler failed"))
				Expect(response["description"]).To(Equal("scheduler 'scheduler-name' not found"))
				Expect(response["code"]).To(Equal("MAE-000"))
				Expect(response["success"]).To(Equal(false))
			})
		})

		Describe("PUT /scheduler/{schedulerName}/image", func() {
			var configYaml1 models.ConfigYAML
			var user, pass string
			var err error

			BeforeEach(func() {
				err = yaml.Unmarshal([]byte(yamlString), &configYaml1)
				Expect(err).NotTo(HaveOccurred())

				MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb,
					logger, app.RoomManager, mmr, yamlString, timeoutSec, nil, workerPortRange, portStart, portEnd)

				scheduler1 = models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)

				user = app.Config.GetString("basicauth.username")
				pass = app.Config.GetString("basicauth.password")
			})

			It("should update image", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
				opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
				MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

				var configYaml models.ConfigYAML
				err = yaml.Unmarshal([]byte(yamlString), &configYaml)

				newImageName := "new-image"
				pods, err := clientset.CoreV1().Pods(configYaml.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

				// Update scheduler
				body := map[string]interface{}{"image": newImageName}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image", configYaml.Name)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				// Select current scheduler yaml
				MockSelectScheduler(yamlString, mockDb, nil)

				// Get redis lock
				MockRedisLock(mockRedisClient, lockKeyNs, lockTimeoutMs, true, nil)

				// Set new operation manager description
				MockAnySetDescription(opManager, mockRedisClient, "running", nil)

				// Remove old rooms
				MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml)

				// Create new roome
				// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
				MockCreateRooms(mockRedisClient, mockPipeline, &configYaml)
				MockGetPortsFromPool(&configYaml, mockRedisClient, nil, workerPortRange, portStart, portEnd)

				// Update new config on schedulers table
				MockUpdateSchedulersTable(mockDb, nil)

				// Add new version into versions table
				scheduler1.NextMajorVersion()
				MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

				// Count to delete old versions if necessary
				MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

				// Retrieve redis lock
				MockReturnRedisLock(mockRedisClient, lockKeyNs, nil)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("should update image with max surge of 100%", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
				opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
				MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

				var configYaml models.ConfigYAML
				err = yaml.Unmarshal([]byte(yamlString), &configYaml)

				newImageName := "new-image"
				pods, err := clientset.CoreV1().Pods(configYaml.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

				// Update scheduler
				body := map[string]interface{}{"image": newImageName}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image?maxsurge=100", configYaml.Name)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				// Select current scheduler yaml
				MockSelectScheduler(yamlString, mockDb, nil)

				// Get redis lock
				MockRedisLock(mockRedisClient, lockKeyNs, lockTimeoutMs, true, nil)

				// Set new operation manager description
				MockAnySetDescription(opManager, mockRedisClient, "running", nil)

				// Remove old rooms
				MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml)

				// Create new roome
				// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
				MockCreateRooms(mockRedisClient, mockPipeline, &configYaml)
				MockGetPortsFromPool(&configYaml, mockRedisClient, nil, workerPortRange, portStart, portEnd)

				// Update new config on schedulers table
				MockUpdateSchedulersTable(mockDb, nil)

				// Add new version into versions table
				scheduler1.NextMajorVersion()
				MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

				// Count to delete old versions if necessary
				MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

				// Retrieve redis lock
				MockReturnRedisLock(mockRedisClient, lockKeyNs, nil)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("should fail with invalid max surge parameter", func() {
				newImageName := "new-image"

				body := map[string]interface{}{"image": newImageName}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image?maxsurge=invalid", configYaml1.Name)
				request, err := http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
				body = make(map[string]interface{})

				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-000"))
				Expect(body["description"]).To(Equal("strconv.Atoi: parsing \"invalid\": invalid syntax"))
				Expect(body["error"]).To(Equal("invalid maxsurge parameter"))
				Expect(body["success"]).To(BeFalse())
			})

			It("should fail with negative max surge parameter", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
				MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

				newImageName := "new-image"

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)
					})

				body := map[string]interface{}{"image": newImageName}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image?maxsurge=-1", configYaml1.Name)
				request, err := http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				body = make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-000"))
				Expect(body["description"]).To(Equal("invalid parameter: maxsurge must be greater than 0"))
				Expect(body["error"]).To(Equal("failed to update scheduler image"))
				Expect(body["success"]).To(BeFalse())
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			})

			It("should fail with zero max surge parameter", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
				MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

				newImageName := "new-image"

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)
					})

				body := map[string]interface{}{"image": newImageName}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image?maxsurge=0", configYaml1.Name)
				request, err := http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				body = make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-000"))
				Expect(body["description"]).To(Equal("invalid parameter: maxsurge must be greater than 0"))
				Expect(body["error"]).To(Equal("failed to update scheduler image"))
				Expect(body["success"]).To(BeFalse())
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			})

			It("should return 500 if DB fails", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
				MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

				newImageName := "new-image"
				pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

				// Update scheduler
				body := map[string]interface{}{"image": newImageName}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image", configYaml1.Name)
				request, err := http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
					Return(pg.NewTestResult(errors.New("some error in db"), 0), errors.New("some error in db"))

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))

				body = make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-001"))
				Expect(body["description"]).To(Equal("some error in db"))
				Expect(body["error"]).To(Equal("DatabaseError"))
				Expect(body["success"]).To(BeFalse())
			})

			It("should return 404 if scheduler does not exist", func() {
				newSchedulerName := "new-scheduler"
				newImageName := "new-image"

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				opManager = models.NewOperationManager(newSchedulerName, mockRedisClient, logger)
				MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

				pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

				// Update scheduler
				body := map[string]interface{}{"image": newImageName}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image", newSchedulerName)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", newSchedulerName).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, "", "")
					})

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))

				body = make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-004"))
				Expect(body["description"]).To(Equal("scheduler new-scheduler not found, create it first"))
				Expect(body["error"]).To(Equal("ValidationFailedError"))
				Expect(body["success"]).To(BeFalse())
			})

			It("should return 422 if image is not sent in body", func() {
				newSchedulerName := "new-scheduler"
				pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

				// Update scheduler
				body := map[string]interface{}{}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image", newSchedulerName)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusUnprocessableEntity))

				body = make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-004"))
				Expect(body["description"]).To(Equal("Image: non zero value required;"))
				Expect(body["error"]).To(Equal("ValidationFailedError"))
				Expect(body["success"]).To(BeFalse())
			})

			It("should return 400 body is not sent", func() {
				newSchedulerName := "new-scheduler"
				pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

				// Update scheduler
				url := fmt.Sprintf("/scheduler/%s/image", newSchedulerName)
				request, err = http.NewRequest("PUT", url, nil)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))

				body := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-000"))
				Expect(body["description"]).To(Equal("image name not sent on body"))
				Expect(body["error"]).To(Equal("image name not sent on body"))
				Expect(body["success"]).To(BeFalse())
			})

			It("should not set image if basicauth is wrong", func() {
				newSchedulerName := "new-scheduler"
				pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

				// Update scheduler
				body := map[string]interface{}{"image": "new-image"}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image", newSchedulerName)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth("wrong user", "wrong pass")

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusUnauthorized))

				body = make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-000"))
				Expect(body["description"]).To(Equal("invalid basic auth"))
				Expect(body["error"]).To(Equal("authentication failed"))
				Expect(body["success"]).To(BeFalse())
			})

			It("should not set image if basicauth is not sent and tryOauthIfUnset is false", func() {
				newSchedulerName := "new-scheduler"
				pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

				// Update scheduler
				body := map[string]interface{}{"image": "new-image"}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image", newSchedulerName)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusUnauthorized))

				body = make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-000"))
				Expect(body["description"]).To(Equal("no basic auth sent"))
				Expect(body["error"]).To(Equal("authentication failed"))
				Expect(body["success"]).To(BeFalse())
			})

			It("should set image if basicauth is not sent and tryOauthIfUnset is true", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
				opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
				MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

				var configYaml models.ConfigYAML
				err = yaml.Unmarshal([]byte(yamlString), &configYaml)

				config, err := GetDefaultConfig()
				Expect(err).NotTo(HaveOccurred())
				config.Set("basicauth.tryOauthIfUnset", true)

				app, err := api.NewApp("0.0.0.0", 9998, config, logger, false, false, "", mockDb, mockCtxWrapper, mockRedisClient, mockRedisTraceWrapper, clientset)
				Expect(err).NotTo(HaveOccurred())
				app.Login = mockLogin
				newImageName := "new-image"

				pods, err := clientset.CoreV1().Pods(configYaml.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

				// Update scheduler
				body := map[string]interface{}{"image": newImageName}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image", configYaml.Name)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				// Select current scheduler yaml
				MockSelectScheduler(yamlString, mockDb, nil)

				// Get redis lock
				MockRedisLock(mockRedisClient, lockKeyNs, lockTimeoutMs, true, nil)

				// Set new operation manager description
				MockAnySetDescription(opManager, mockRedisClient, "running", nil)

				// Remove old rooms
				MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml)

				// Create new roome
				// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
				MockCreateRooms(mockRedisClient, mockPipeline, &configYaml)
				MockGetPortsFromPool(&configYaml, mockRedisClient, nil, workerPortRange, portStart, portEnd)

				// Update new config on schedulers table
				MockUpdateSchedulersTable(mockDb, nil)

				// Add new version into versions table
				scheduler1.NextMajorVersion()
				MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

				// Count to delete old versions if necessary
				MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

				// Retrieve redis lock
				MockReturnRedisLock(mockRedisClient, lockKeyNs, nil)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("should set image on scheduler with two containers per pod", func() {

				jsonString := `{
  "name": "scheduler-name-2",
  "game": "game-name",
	"autoscaling": {
    "min": 1
	},
	"containers": [{
		"name": "container1",
		"image": "image1"
	}, {
		"name": "container2",
		"image": "image2"
	}]
}`

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
				MockCreateScheduler(clientset, mockRedisClient, mockPipeline, mockDb,
					logger, app.RoomManager, mmr, jsonString, timeoutSec, nil, workerPortRange, portStart, portEnd)

				err := json.Unmarshal([]byte(jsonString), &configYaml1)
				Expect(err).NotTo(HaveOccurred())
				scheduler1 = models.NewScheduler(configYaml1.Name, configYaml1.Game, jsonString)

				app, err := api.NewApp("0.0.0.0", 9998, config, logger, false, false, "", mockDb, mockCtxWrapper, mockRedisClient, mockRedisTraceWrapper, clientset)
				Expect(err).NotTo(HaveOccurred())
				app.Login = mockLogin

				newImageName := "new-image"
				containerName := "container1"
				lockKeyNs := fmt.Sprintf("%s-%s", lockKey, configYaml1.Name)

				pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

				// Update scheduler
				body := map[string]interface{}{
					"image":     newImageName,
					"container": containerName,
				}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image", configYaml1.Name)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
				MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

				// Select current scheduler yaml
				MockSelectScheduler(jsonString, mockDb, nil)

				// Get redis lock
				MockRedisLock(mockRedisClient, lockKeyNs, lockTimeoutMs, true, nil)

				// Set new operation manager description
				MockAnySetDescription(opManager, mockRedisClient, "running", nil)

				// Remove old rooms
				MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml1)

				// Create new roome
				// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
				MockCreateRooms(mockRedisClient, mockPipeline, &configYaml1)

				// Update new config on schedulers table
				MockUpdateSchedulersTable(mockDb, nil)

				// Add new version into versions table
				scheduler1.NextMajorVersion()
				MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

				// Count to delete old versions if necessary
				MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

				// Retrieve redis lock
				MockReturnRedisLock(mockRedisClient, lockKeyNs, nil)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("should update image asynchronously", func() {
				var configYaml models.ConfigYAML
				err = yaml.Unmarshal([]byte(yamlString), &configYaml)

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
				opManager = models.NewOperationManager(configYaml.Name, mockRedisClient, logger)
				MockGetCurrentOperationKey(opManager, mockRedisClient, nil)

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
					Expect(m).To(HaveKeyWithValue("operation", "UpdateSchedulerImage"))
				})
				mockPipeline.EXPECT().Expire(gomock.Any(), timeoutDur)
				mockPipeline.EXPECT().Set(opManager.BuildCurrOpKey(), gomock.Any(), timeoutDur)
				mockPipeline.EXPECT().Exec()

				mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(map[string]string{
					"not": "empty",
				}, nil)).AnyTimes()

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
					Expect(m).To(HaveKeyWithValue("status", http.StatusOK))
					Expect(m).To(HaveKeyWithValue("operation", "UpdateSchedulerImage"))
					Expect(m).To(HaveKeyWithValue("success", true))
				})
				mockPipeline.EXPECT().Expire(gomock.Any(), 10*time.Minute)
				mockPipeline.EXPECT().Del(opManager.BuildCurrOpKey())
				mockPipeline.EXPECT().Exec().Do(func() {
					opManager.StopLoop()
				})

				newImageName := "new-image"
				pods, err := clientset.CoreV1().Pods(configYaml.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

				// Update scheduler
				body := map[string]interface{}{"image": newImageName}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image?async=true", configYaml.Name)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				// Select current scheduler yaml
				MockSelectScheduler(yamlString, mockDb, nil)

				// Get redis lock
				MockRedisLock(mockRedisClient, lockKeyNs, lockTimeoutMs, true, nil)

				// Set new operation manager description
				MockAnySetDescription(opManager, mockRedisClient, "running", nil)

				// Remove old rooms
				MockRemoveRoomsFromRedis(mockRedisClient, mockPipeline, pods, &configYaml)

				// Create new roome
				// It will use the same number of rooms as config1, and ScaleUp to new min in Watcher at AutoScale
				MockCreateRooms(mockRedisClient, mockPipeline, &configYaml)
				MockGetPortsFromPool(&configYaml, mockRedisClient, nil, workerPortRange, portStart, portEnd)

				// Update new config on schedulers table
				MockUpdateSchedulersTable(mockDb, nil)

				// Add new version into versions table
				scheduler1.NextMajorVersion()
				MockInsertIntoVersionsTable(scheduler1, mockDb, nil)

				// Count to delete old versions if necessary
				MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

				// Retrieve redis lock
				MockReturnRedisLock(mockRedisClient, lockKeyNs, nil)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				var response map[string]interface{}
				json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(response).To(HaveKeyWithValue("success", true))
				Expect(response).To(HaveKey("operationKey"))
				Expect(recorder.Code).To(Equal(http.StatusOK))

				Eventually(opManager.IsStopped, 1*time.Minute).Should(BeTrue())
			})

			It("should update image asynchronously and show error when occurred", func() {
				var configYaml models.ConfigYAML
				err = yaml.Unmarshal([]byte(yamlString), &configYaml)

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
				opManager = models.NewOperationManager(configYaml.Name, mockRedisClient, logger)

				MockGetCurrentOperationKey(opManager, mockRedisClient, nil)
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
					Expect(m).To(HaveKeyWithValue("operation", "UpdateSchedulerImage"))
				})
				mockPipeline.EXPECT().Expire(gomock.Any(), timeoutDur)
				mockPipeline.EXPECT().Set(opManager.BuildCurrOpKey(), gomock.Any(), timeoutDur)
				mockPipeline.EXPECT().Exec()

				mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(map[string]string{
					"not": "empty",
				}, nil)).AnyTimes()

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
					Expect(m).To(HaveKeyWithValue("status", http.StatusInternalServerError))
					Expect(m).To(HaveKeyWithValue("operation", "UpdateSchedulerImage"))
					Expect(m).To(HaveKeyWithValue("success", false))
					Expect(m).To(HaveKeyWithValue("description", "failed to update scheduler image"))
					Expect(m).To(HaveKeyWithValue("error", "redis error"))
				})
				mockPipeline.EXPECT().Expire(gomock.Any(), 10*time.Minute)
				mockPipeline.EXPECT().Del(opManager.BuildCurrOpKey())
				mockPipeline.EXPECT().Exec().Do(func() {
					opManager.StopLoop()
				})

				newImageName := "new-image"
				pods, err := clientset.CoreV1().Pods(configYaml.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml.AutoScaling.Min))

				// Update scheduler
				body := map[string]interface{}{"image": newImageName}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image?async=true", configYaml.Name)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				// Select current scheduler yaml
				MockSelectScheduler(yamlString, mockDb, nil)

				// Get redis lock
				MockRedisLock(mockRedisClient, lockKeyNs, lockTimeoutMs, true, errors.New("redis error"))

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				var response map[string]interface{}
				json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(response).To(HaveKeyWithValue("success", true))
				Expect(response).To(HaveKey("operationKey"))
				Expect(recorder.Code).To(Equal(http.StatusOK))

				Eventually(opManager.IsStopped, 1*time.Minute).Should(BeTrue())
			})
		})

		Describe("PUT /scheduler/{schedulerName}/min", func() {
			var configYaml1 models.ConfigYAML
			var user, pass string

			BeforeEach(func() {
				// Create scheduler
				reader := strings.NewReader(yamlString)
				url := "/scheduler"
				request, err := http.NewRequest("POST", url, reader)
				Expect(err).NotTo(HaveOccurred())

				err = yaml.Unmarshal([]byte(yamlString), &configYaml1)
				Expect(err).NotTo(HaveOccurred())

				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
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

				mockRedisClient.EXPECT().
					Get(models.GlobalPortsPoolKey).
					Return(goredis.NewStringResult(workerPortRange, nil)).
					Times(configYaml1.AutoScaling.Min)

				MockInsertScheduler(mockDb, nil)
				MockUpdateSchedulerStatus(mockDb, nil, nil)

				mockRedisClient.EXPECT().Ping().AnyTimes()

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusCreated))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))

				user = app.Config.GetString("basicauth.username")
				pass = app.Config.GetString("basicauth.password")
			})

			It("should update min", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
				opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
				MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

				scheduler1 := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)
				newMin := configYaml1.AutoScaling.Min + 1

				// Update scheduler
				body := map[string]interface{}{"min": newMin}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/min", configYaml1.Name)
				request, err := http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				schedulerLockKey := fmt.Sprintf("%s-%s", lockKey, configYaml1.Name)
				mockRedisClient.EXPECT().
					Eval(gomock.Any(), []string{schedulerLockKey}, gomock.Any()).
					Return(redis.NewCmdResult(nil, nil))
				mockRedisClient.EXPECT().
					SetNX(schedulerLockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
					Return(redis.NewBoolResult(true, nil))

				// Set new operation manager description
				MockAnySetDescription(opManager, mockRedisClient, "running", nil)

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)
					})

				// Update new config on schedulers table
				MockUpdateSchedulersTable(mockDb, nil)
				// Add new version into versions table
				scheduler1.NextMinorVersion()
				MockInsertIntoVersionsTable(scheduler1, mockDb, nil)
				// Count to delete old versions if necessary
				MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

				recorder = httptest.NewRecorder()

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			})

			It("should return 500 if DB fails", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
				MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

				newMin := configYaml1.AutoScaling.Min + 1

				// Update scheduler
				body := map[string]interface{}{"min": newMin}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/min", configYaml1.Name)
				request, err := http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
					Return(pg.NewTestResult(errors.New("some error in db"), 0), errors.New("some error in db"))

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)

				body = make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-001"))
				Expect(body["description"]).To(Equal("some error in db"))
				Expect(body["error"]).To(Equal("DatabaseError"))
				Expect(body["success"]).To(BeFalse())

				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			})

			It("should return 404 if scheduler does not exist", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient)
				newSchedulerName := "new-scheduler"
				newMin := configYaml1.AutoScaling.Min

				opManager = models.NewOperationManager(newSchedulerName, mockRedisClient, logger)
				MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

				// Update scheduler
				body := map[string]interface{}{"min": newMin}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/min", newSchedulerName)
				request, err := http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", newSchedulerName).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, "", "")
					})

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))

				body = make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-004"))
				Expect(body["description"]).To(Equal("scheduler new-scheduler not found, create it first"))
				Expect(body["error"]).To(Equal("ValidationFailedError"))
				Expect(body["success"]).To(BeFalse())
			})

			It("should return 422 if body is not sent", func() {
				newSchedulerName := "new-scheduler"

				// Update scheduler
				url := fmt.Sprintf("/scheduler/%s/min", newSchedulerName)
				request, err := http.NewRequest("PUT", url, nil)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusBadRequest))

				body := make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-000"))
				Expect(body["description"]).To(Equal("min not sent on body"))
				Expect(body["error"]).To(Equal("min not sent on body"))
				Expect(body["success"]).To(BeFalse())
			})

			It("should set min 0 if body is empty (default value is 0)", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
				opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
				MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

				scheduler1 := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)
				// Update scheduler
				body := map[string]interface{}{}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/min", configYaml1.Name)
				request, err := http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)
					})

				schedulerLockKey := fmt.Sprintf("%s-%s", lockKey, configYaml1.Name)
				mockRedisClient.EXPECT().
					Eval(gomock.Any(), []string{schedulerLockKey}, gomock.Any()).
					Return(redis.NewCmdResult(nil, nil))
				mockRedisClient.EXPECT().
					SetNX(schedulerLockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
					Return(redis.NewBoolResult(true, nil))

				// Set new operation manager description
				MockAnySetDescription(opManager, mockRedisClient, "running", nil)

				// Update new config on schedulers table
				MockUpdateSchedulersTable(mockDb, nil)
				// Add new version into versions table
				scheduler1.NextMinorVersion()
				MockInsertIntoVersionsTable(scheduler1, mockDb, nil)
				// Count to delete old versions if necessary
				MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			})

			It("should not set min if basicauth is wrong", func() {
				newSchedulerName := "new-scheduler"
				pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

				// Update scheduler
				body := map[string]interface{}{"min": configYaml1.AutoScaling.Min + 1}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image", newSchedulerName)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth("wrong user", "wrong pass")

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusUnauthorized))

				body = make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-000"))
				Expect(body["description"]).To(Equal("invalid basic auth"))
				Expect(body["error"]).To(Equal("authentication failed"))
				Expect(body["success"]).To(BeFalse())
			})

			It("should not set min if basicauth is not sent and tryOauthIfUnset is false", func() {
				newSchedulerName := "new-scheduler"
				pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

				// Update scheduler
				body := map[string]interface{}{"min": configYaml1.AutoScaling.Min + 1}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/image", newSchedulerName)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusUnauthorized))

				body = make(map[string]interface{})
				err = json.Unmarshal(recorder.Body.Bytes(), &body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body["code"]).To(Equal("MAE-000"))
				Expect(body["description"]).To(Equal("no basic auth sent"))
				Expect(body["error"]).To(Equal("authentication failed"))
				Expect(body["success"]).To(BeFalse())
			})

			It("should set min if basicauth is not sent and tryOauthIfUnset is true", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).Times(2)
				opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
				MockOperationManager(opManager, timeoutDur, mockRedisClient, mockPipeline)

				config, err := GetDefaultConfig()
				config.Set("basicauth.tryOauthIfUnset", true)
				app, err := api.NewApp("0.0.0.0", 9998, config, logger, false, false, "", mockDb, mockCtxWrapper, mockRedisClient, mockRedisTraceWrapper, clientset)
				Expect(err).NotTo(HaveOccurred())
				app.Login = mockLogin

				pods, err := clientset.CoreV1().Pods(configYaml1.Name).List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(configYaml1.AutoScaling.Min))

				schedulerLockKey := fmt.Sprintf("%s-%s", lockKey, configYaml1.Name)
				mockRedisClient.EXPECT().
					Eval(gomock.Any(), []string{schedulerLockKey}, gomock.Any()).
					Return(redis.NewCmdResult(nil, nil))
				mockRedisClient.EXPECT().
					SetNX(schedulerLockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
					Return(redis.NewBoolResult(true, nil))

				// Set new operation manager description
				MockAnySetDescription(opManager, mockRedisClient, "running", nil)

				// Update scheduler
				body := map[string]interface{}{"min": configYaml1.AutoScaling.Min + 1}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/min", configYaml1.Name)
				request, err = http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)
					})

				scheduler1 := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)
				// Update new config on schedulers table
				MockUpdateSchedulersTable(mockDb, nil)
				// Add new version into versions table
				scheduler1.NextMinorVersion()
				MockInsertIntoVersionsTable(scheduler1, mockDb, nil)
				// Count to delete old versions if necessary
				MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

				recorder = httptest.NewRecorder()
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			})

			It("should update min asynchronously", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
				opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
				MockGetCurrentOperationKey(opManager, mockRedisClient, nil)

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
					Expect(m).To(HaveKeyWithValue("operation", "UpdateSchedulerMin"))
				})
				mockPipeline.EXPECT().Expire(gomock.Any(), timeoutDur)
				mockPipeline.EXPECT().Set(opManager.BuildCurrOpKey(), gomock.Any(), timeoutDur)
				mockPipeline.EXPECT().Exec()

				mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(map[string]string{
					"not": "empty",
				}, nil)).AnyTimes()

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
					Expect(m).To(HaveKeyWithValue("status", http.StatusOK))
					Expect(m).To(HaveKeyWithValue("operation", "UpdateSchedulerMin"))
					Expect(m).To(HaveKeyWithValue("success", true))
				})
				mockPipeline.EXPECT().Expire(gomock.Any(), 10*time.Minute)
				mockPipeline.EXPECT().Del(opManager.BuildCurrOpKey())
				mockPipeline.EXPECT().Exec().Do(func() {
					opManager.StopLoop()
				})

				scheduler1 := models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)
				newMin := configYaml1.AutoScaling.Min + 1

				// Update scheduler
				body := map[string]interface{}{"min": newMin}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/min?async=true", configYaml1.Name)
				request, err := http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				schedulerLockKey := fmt.Sprintf("%s-%s", lockKey, configYaml1.Name)
				mockRedisClient.EXPECT().
					Eval(gomock.Any(), []string{schedulerLockKey}, gomock.Any()).
					Return(redis.NewCmdResult(nil, nil))
				mockRedisClient.EXPECT().
					SetNX(schedulerLockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
					Return(redis.NewBoolResult(true, nil))

				// Set new operation manager description
				MockAnySetDescription(opManager, mockRedisClient, "running", nil)

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)
					})

				// Update new config on schedulers table
				MockUpdateSchedulersTable(mockDb, nil)
				// Add new version into versions table
				scheduler1.NextMinorVersion()
				MockInsertIntoVersionsTable(scheduler1, mockDb, nil)
				// Count to delete old versions if necessary
				MockCountNumberOfVersions(scheduler1, numberOfVersions, mockDb, nil)

				recorder = httptest.NewRecorder()

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				var response map[string]interface{}
				json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(response).To(HaveKeyWithValue("success", true))
				Expect(response).To(HaveKey("operationKey"))

				Eventually(opManager.IsStopped, 1*time.Minute).Should(BeTrue())
			})

			It("should update min asynchronously and show error when occurred", func() {
				mockRedisTraceWrapper.EXPECT().WithContext(gomock.Any(), mockRedisClient).Return(mockRedisClient).AnyTimes()
				opManager = models.NewOperationManager(configYaml1.Name, mockRedisClient, logger)
				MockGetCurrentOperationKey(opManager, mockRedisClient, nil)

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
					Expect(m).To(HaveKeyWithValue("operation", "UpdateSchedulerMin"))
				})
				mockPipeline.EXPECT().Expire(gomock.Any(), timeoutDur)
				mockPipeline.EXPECT().Set(opManager.BuildCurrOpKey(), gomock.Any(), timeoutDur)
				mockPipeline.EXPECT().Exec()

				mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(goredis.NewStringStringMapResult(map[string]string{
					"not": "empty",
				}, nil)).AnyTimes()

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(func(_ string, m map[string]interface{}) {
					Expect(m).To(HaveKeyWithValue("status", http.StatusInternalServerError))
					Expect(m).To(HaveKeyWithValue("operation", "UpdateSchedulerMin"))
					Expect(m).To(HaveKeyWithValue("success", false))
					Expect(m).To(HaveKeyWithValue("error", "error to update scheduler on schedulers table: db failed"))
				})
				mockPipeline.EXPECT().Expire(gomock.Any(), 10*time.Minute)
				mockPipeline.EXPECT().Del(opManager.BuildCurrOpKey())
				mockPipeline.EXPECT().Exec().Do(func() {
					opManager.StopLoop()
				})

				newMin := configYaml1.AutoScaling.Min + 1

				// Update scheduler
				body := map[string]interface{}{"min": newMin}
				bts, _ := json.Marshal(body)
				reader := strings.NewReader(string(bts))
				url := fmt.Sprintf("/scheduler/%s/min?async=true", configYaml1.Name)
				request, err := http.NewRequest("PUT", url, reader)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(user, pass)

				schedulerLockKey := fmt.Sprintf("%s-%s", lockKey, configYaml1.Name)
				mockRedisClient.EXPECT().
					Eval(gomock.Any(), []string{schedulerLockKey}, gomock.Any()).
					Return(redis.NewCmdResult(nil, nil))
				mockRedisClient.EXPECT().
					SetNX(schedulerLockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
					Return(redis.NewBoolResult(true, nil))

				// Set new operation manager description
				MockAnySetDescription(opManager, mockRedisClient, "running", nil)

				mockDb.EXPECT().
					Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml1.Name).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						*scheduler = *models.NewScheduler(configYaml1.Name, configYaml1.Game, yamlString)
					})

				// Update new config on schedulers table
				MockUpdateSchedulersTable(mockDb, errors.New("db failed"))

				recorder = httptest.NewRecorder()

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusOK))
				var response map[string]interface{}
				json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(response).To(HaveKeyWithValue("success", true))
				Expect(response).To(HaveKey("operationKey"))

				Eventually(opManager.IsStopped, 1*time.Minute).Should(BeTrue())
			})
		})
	})

	Context("When authentication fails", func() {
		BeforeEach(func() {
			mockCtxWrapper.EXPECT().WithContext(gomock.Any(), app.DBClient.DB).Return(app.DBClient.DB).AnyTimes()
		})

		It("should return status code 500 if db fails", func() {
			mockDb.EXPECT().Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, gomock.Any()).
				Return(nil, errors.New("database error"))

			url := fmt.Sprintf("http://%s/scheduler/scheduler-name", app.Address)
			request, err := http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			body := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(HaveKeyWithValue("code", "MAE-001"))
			Expect(body).To(HaveKeyWithValue("description", "database error"))
			Expect(body).To(HaveKeyWithValue("error", "DatabaseError"))
			Expect(body).To(HaveKeyWithValue("success", false))
		})

		It("should return status code 500 if authentication fails with error", func() {
			mockDb.EXPECT().Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, gomock.Any()).
				Do(func(destToken *login.DestinationToken, query string, modifier string) {
					destToken.RefreshToken = "refresh-token"
				}).
				Return(nil, nil)

			mockLogin.EXPECT().Authenticate(gomock.Any(), app.DBClient.DB).
				Return("", 0, errors.New("authentication failed"))

			url := fmt.Sprintf("http://%s/scheduler/scheduler-name", app.Address)
			request, err := http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			body := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(HaveKeyWithValue("code", "MAE-000"))
			Expect(body).To(HaveKeyWithValue("description", "authentication failed"))
			Expect(body).To(HaveKeyWithValue("error", "Error fetching googleapis"))
			Expect(body).To(HaveKeyWithValue("success", false))
		})

		It("should return status code 401 if token is unauthorized", func() {
			mockDb.EXPECT().Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, gomock.Any()).
				Do(func(destToken *login.DestinationToken, query string, modifier string) {
					destToken.RefreshToken = "refresh-token"
				}).
				Return(nil, nil)

			mockLogin.EXPECT().Authenticate(gomock.Any(), app.DBClient.DB).
				Return("not authorized", http.StatusUnauthorized, nil)

			url := fmt.Sprintf("http://%s/scheduler/scheduler-name", app.Address)
			request, err := http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
			body := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(HaveKeyWithValue("code", "MAE-005"))
			Expect(body).To(HaveKeyWithValue("description", "not authorized"))
			Expect(body).To(HaveKeyWithValue("error", "invalid access token"))
		})

		It("should return status code 401 if authentication fails", func() {
			mockDb.EXPECT().Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, gomock.Any()).
				Do(func(destToken *login.DestinationToken, query string, modifier string) {
					destToken.RefreshToken = "refresh-token"
				}).
				Return(nil, nil)

			mockLogin.EXPECT().Authenticate(gomock.Any(), app.DBClient.DB).
				Return("not authorized", http.StatusBadRequest, nil)

			url := fmt.Sprintf("http://%s/scheduler/scheduler-name", app.Address)
			request, err := http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
			body := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(HaveKeyWithValue("code", "MAE-005"))
			Expect(body).To(HaveKeyWithValue("description", "not authorized"))
			Expect(body).To(HaveKeyWithValue("error", "Unauthorized access token"))
		})

		It("should return status code 401 if email is not authorized", func() {
			mockDb.EXPECT().Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, gomock.Any()).
				Do(func(destToken *login.DestinationToken, query string, modifier string) {
					destToken.RefreshToken = "refresh-token"
				}).
				Return(nil, nil)

			mockLogin.EXPECT().Authenticate(gomock.Any(), app.DBClient.DB).
				Return("user@notauthorized.com", http.StatusOK, nil)

			url := fmt.Sprintf("http://%s/scheduler/scheduler-name", app.Address)
			request, err := http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
			body := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(HaveKeyWithValue("code", "MAE-005"))
			Expect(body).To(HaveKeyWithValue("description", "the email on OAuth authorization is not from domain [example.com other.com]"))
			Expect(body).To(HaveKeyWithValue("error", "authorization access error"))
		})

		It("should return status code 401 if token is not found on DB", func() {
			mockDb.EXPECT().Query(gomock.Any(), `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = ?`, gomock.Any())

			url := fmt.Sprintf("http://%s/scheduler/scheduler-name", app.Address)
			request, err := http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
			body := make(map[string]interface{})
			err = json.Unmarshal(recorder.Body.Bytes(), &body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(HaveKeyWithValue("code", "MAE-005"))
			Expect(body).To(HaveKeyWithValue("description", "access token error"))
			Expect(body).To(HaveKeyWithValue("error", "access token was not found on db"))
		})
	})
})
