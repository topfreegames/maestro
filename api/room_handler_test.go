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

	"k8s.io/client-go/kubernetes"

	"github.com/go-redis/redis"
	goredis "github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/api"
	"github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/models"
	. "github.com/topfreegames/maestro/testing"
)

var _ = Describe("Room Handler", func() {
	var request *http.Request
	var recorder *httptest.ResponseRecorder

	namespace := "schedulerName"
	game := "game"
	yamlStr := `
name: schedulerName
game: game
image: image:v1
autoscaling:
  min: 100
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
forwarders:
  mockplugin:
    mockfwd:
      enabled: true
    anothermockfwd:
      enabled: true
    disabledmockfwd:
      enabled: false
`

	createNamespace := func(name string, clientset kubernetes.Interface) error {
		return models.NewNamespace(name).Create(clientset)
	}
	createPod := func(name, namespace string, clientset kubernetes.Interface) error {
		pod, err := models.NewPod(
			"game", "img", name, namespace, nil, nil, 0,
			[]*models.Port{&models.Port{ContainerPort: 1234, Name: "port1", Protocol: "UDP"}},
			nil, nil, mockClientset, mockRedisClient,
		)
		if err != nil {
			return err
		}
		_, err = pod.Create(clientset)
		return err
	}

	BeforeEach(func() { // Record HTTP responses.
		recorder = httptest.NewRecorder()
	})

	Describe("PUT /scheduler/{schedulerName}/rooms/{roomName}/ping", func() {
		url := "/scheduler/schedulerName/rooms/roomName/ping"
		rKey := "scheduler:schedulerName:rooms:roomName"
		pKey := "scheduler:schedulerName:ping"
		sKey := "scheduler:schedulerName:status:ready"
		lKey := "scheduler:schedulerName:last:status:occupied"
		roomName := "roomName"
		status := "ready"
		allStatusKeys := []string{
			"scheduler:schedulerName:status:creating",
			"scheduler:schedulerName:status:occupied",
			"scheduler:schedulerName:status:terminating",
			"scheduler:schedulerName:status:terminated",
		}

		Context("when all services are healthy", func() {
			It("returns a status code of 200 and success body", func() {
				reader := JSONFor(JSON{
					"timestamp": time.Now().Unix(),
					"status":    status,
				})
				request, _ = http.NewRequest("PUT", url, reader)

				mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", namespace).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
						scheduler.Game = game
					})

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
					"lastPing": time.Now().Unix(),
					"status":   status,
				})
				mockPipeline.EXPECT().ZAdd(pKey, gomock.Any())
				mockPipeline.EXPECT().ZRem(lKey, roomName)
				mockPipeline.EXPECT().SAdd(sKey, rKey)
				for _, key := range allStatusKeys {
					mockPipeline.EXPECT().SRem(key, rKey)
				}
				mockPipeline.EXPECT().Exec()

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			})

			It("uses cache in the second ping", func() {
				reader := JSONFor(JSON{
					"timestamp": time.Now().Unix(),
					"status":    status,
				})
				request, _ = http.NewRequest("PUT", url, reader)

				mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", namespace).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
						scheduler.Game = game
					})

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(2)
				mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
					"lastPing": time.Now().Unix(),
					"status":   status,
				}).Times(2)
				mockPipeline.EXPECT().ZAdd(pKey, gomock.Any()).Times(2)
				mockPipeline.EXPECT().ZRem(lKey, roomName).Times(2)
				mockPipeline.EXPECT().SAdd(sKey, rKey).Times(2)
				for _, key := range allStatusKeys {
					mockPipeline.EXPECT().SRem(key, rKey).Times(2)
				}
				mockPipeline.EXPECT().Exec().Times(2)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(recorder.Code).To(Equal(200))

				recorder = httptest.NewRecorder()
				reader = JSONFor(JSON{
					"timestamp": time.Now().Unix(),
					"status":    status,
				})
				request, _ = http.NewRequest("PUT", url, reader)
				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
				Expect(recorder.Code).To(Equal(200))
			})

			It("returns status code of 422 if missing timestamp", func() {
				reader := JSONFor(JSON{
					"status": status,
				})
				request, _ = http.NewRequest("PUT", url, reader)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(422))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(ContainSubstring("Timestamp: non zero value required"))
				Expect(obj["success"]).To(Equal(false))
			})

			It("returns status code of 422 if invalid timestamp", func() {
				reader := JSONFor(JSON{
					"timestamp": "not-a-timestamp",
					"status":    status,
				})
				request, _ = http.NewRequest("PUT", url, reader)

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

			It("returns status code of 422 if missing status", func() {
				reader := JSONFor(JSON{
					"timestamp": time.Now().Unix(),
				})
				request, _ = http.NewRequest("PUT", url, reader)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(422))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(ContainSubstring("Status: non zero value required"))
				Expect(obj["success"]).To(Equal(false))
			})

			It("returns status code of 422 if invalid status", func() {
				reader := JSONFor(JSON{
					"timestamp": time.Now().Unix(),
					"status":    "not-valid",
				})
				request, _ = http.NewRequest("PUT", url, reader)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(422))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(ContainSubstring("Status: not-valid does not validate as matches"))
				Expect(obj["success"]).To(Equal(false))
			})
		})

		Context("with eventforwarders", func() {
			var app *api.App
			game := "somegame"
			BeforeEach(func() {
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
					Return(goredis.NewStringResult("5000", nil))
				mockPipeline.EXPECT().Exec()

				createNamespace(namespace, clientset)
				err := createPod(roomName, namespace, clientset)
				Expect(err).NotTo(HaveOccurred())
				app, err = api.NewApp("0.0.0.0", 9998, config, logger, false, "", mockDb, mockRedisClient, clientset)
				Expect(err).NotTo(HaveOccurred())
				app.Forwarders = []*eventforwarder.Info{
					&eventforwarder.Info{
						Plugin:    "mockplugin",
						Name:      "mockfwd",
						Forwarder: mockEventForwarder1,
					},
				}
			})

			It("forwards room event", func() {
				reader := JSONFor(JSON{
					"timestamp": time.Now().Unix(),
					"status":    status,
				})
				request, _ = http.NewRequest("PUT", url, reader)

				mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", namespace).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
						scheduler.Game = game
					})

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
					"lastPing": time.Now().Unix(),
					"status":   status,
				})
				mockPipeline.EXPECT().ZAdd(pKey, gomock.Any())
				mockPipeline.EXPECT().ZRem(lKey, roomName)
				mockPipeline.EXPECT().SAdd(sKey, rKey)
				for _, key := range allStatusKeys {
					mockPipeline.EXPECT().SRem(key, rKey)
				}
				mockPipeline.EXPECT().Exec()

				mockEventForwarder1.EXPECT().Forward(fmt.Sprintf("ping%s", strings.Title(status)), gomock.Any())

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			})
		})

		Context("when redis is down", func() {
			It("returns status code of 500 if redis is unavailable", func() {
				reader := JSONFor(JSON{
					"timestamp": time.Now().Unix(),
					"status":    status,
				})
				request, _ = http.NewRequest("PUT", url, reader)

				mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", namespace).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
						scheduler.Game = game
					})

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
					"lastPing": time.Now().Unix(),
					"status":   status,
				})
				mockPipeline.EXPECT().ZAdd(pKey, gomock.Any())
				mockPipeline.EXPECT().ZRem(lKey, roomName)
				mockPipeline.EXPECT().SAdd(sKey, rKey)
				for _, key := range allStatusKeys {
					mockPipeline.EXPECT().SRem(key, rKey)
				}
				mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-000"))
				Expect(obj["error"]).To(Equal("Ping failed"))
				Expect(obj["description"]).To(Equal("some error in redis"))
				Expect(obj["success"]).To(Equal(false))
			})
		})
	})

	Describe("PUT /scheduler/{schedulerName}/rooms/{roomName}/status", func() {
		url := "/scheduler/schedulerName/rooms/roomName/status"
		rKey := "scheduler:schedulerName:rooms:roomName"
		pKey := "scheduler:schedulerName:ping"
		lKey := "scheduler:schedulerName:last:status:occupied"
		roomName := "roomName"
		status := "ready"
		newSKey := fmt.Sprintf("scheduler:schedulerName:status:%s", status)
		allStatusKeys := []string{
			"scheduler:schedulerName:status:creating",
			"scheduler:schedulerName:status:occupied",
			"scheduler:schedulerName:status:terminating",
			"scheduler:schedulerName:status:terminated",
		}

		//TODO ver se envia forward
		Context("when all services are healthy", func() {
			It("returns a status code of 200 and success body", func() {
				reader := JSONFor(JSON{
					"status":    status,
					"timestamp": time.Now().Unix(),
				})
				request, _ = http.NewRequest("PUT", url, reader)

				mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", namespace).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
						scheduler.Game = game
					})

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
					"lastPing": time.Now().Unix(),
					"status":   status,
				})
				mockPipeline.EXPECT().ZAdd(pKey, gomock.Any())
				mockPipeline.EXPECT().ZRem(lKey, roomName)
				mockPipeline.EXPECT().SAdd(newSKey, rKey)
				for _, key := range allStatusKeys {
					mockPipeline.EXPECT().SRem(key, rKey)
				}
				mockPipeline.EXPECT().Exec()

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
			})

			It("returns status code of 422 if missing timestamp", func() {
				reader := JSONFor(JSON{
					"status": status,
				})
				request, _ = http.NewRequest("PUT", url, reader)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(422))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(ContainSubstring("Timestamp: non zero value required"))
				Expect(obj["success"]).To(Equal(false))
			})

			It("returns status code of 422 if invalid timestamp", func() {
				reader := JSONFor(JSON{
					"status":    status,
					"timestamp": "not-a-timestamp",
				})
				request, _ = http.NewRequest("PUT", url, reader)

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

			It("returns status code of 422 if missing status", func() {
				reader := JSONFor(JSON{
					"timestamp": time.Now().Unix(),
				})
				request, _ = http.NewRequest("PUT", url, reader)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(422))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(ContainSubstring("Status: non zero value required"))
				Expect(obj["success"]).To(Equal(false))
			})

			It("returns status code of 422 if invalid status", func() {
				reader := JSONFor(JSON{
					"status":    "invalid-status",
					"timestamp": time.Now().Unix(),
				})
				request, _ = http.NewRequest("PUT", url, reader)

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(422))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-004"))
				Expect(obj["error"]).To(Equal("ValidationFailedError"))
				Expect(obj["description"]).To(ContainSubstring("does not validate as matches"))
				Expect(obj["success"]).To(Equal(false))
			})

			Context("with eventforwarders", func() {
				// TODO map status from api to something standard
				var app *api.App
				game := "somegame"
				BeforeEach(func() {
					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
					mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
						Return(goredis.NewStringResult("5000", nil))
					mockPipeline.EXPECT().Exec()

					createNamespace(namespace, clientset)
					err := createPod(roomName, namespace, clientset)
					Expect(err).NotTo(HaveOccurred())
					app, err = api.NewApp("0.0.0.0", 9998, config, logger, false, "", mockDb, mockRedisClient, clientset)
					Expect(err).NotTo(HaveOccurred())
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
						&eventforwarder.Info{
							Plugin:    "mockplugin",
							Name:      "disabledmockfwd",
							Forwarder: mockEventForwarder3,
						},
						&eventforwarder.Info{
							Plugin:    "mockplugin",
							Name:      "unexistentmockfwd",
							Forwarder: mockEventForwarder4,
						},
						&eventforwarder.Info{
							Plugin:    "unexistentmockplugin",
							Name:      "unexistentmockfwd",
							Forwarder: mockEventForwarder5,
						},
					}
				})

				It("should forward event to enabled eventforwarders", func() {
					reader := JSONFor(JSON{
						"status":    status,
						"timestamp": time.Now().Unix(),
					})
					request, _ = http.NewRequest("PUT", url, reader)

					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
					mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
						"lastPing": time.Now().Unix(),
						"status":   status,
					})
					mockPipeline.EXPECT().ZAdd(pKey, gomock.Any())
					mockPipeline.EXPECT().ZRem(lKey, roomName)
					mockPipeline.EXPECT().SAdd(newSKey, rKey)
					for _, key := range allStatusKeys {
						mockPipeline.EXPECT().SRem(key, rKey)
					}
					mockPipeline.EXPECT().Exec()
					mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", namespace).
						Do(func(scheduler *models.Scheduler, query string, modifier string) {
							scheduler.YAML = yamlStr
							scheduler.Game = game
						})
					mockEventForwarder1.EXPECT().Forward(status, gomock.Any())
					mockEventForwarder2.EXPECT().Forward(status, gomock.Any())

					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
					Expect(recorder.Code).To(Equal(200))
				})

				It("should forward event to eventforwarders with metadata", func() {
					reader := JSONFor(JSON{
						"status":    status,
						"timestamp": time.Now().Unix(),
						"metadata": map[string]string{
							"type": "sometype",
						},
					})
					request, _ = http.NewRequest("PUT", url, reader)

					mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
					mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
						"lastPing": time.Now().Unix(),
						"status":   status,
					})
					mockPipeline.EXPECT().ZAdd(pKey, gomock.Any())
					mockPipeline.EXPECT().ZRem(lKey, roomName)
					mockPipeline.EXPECT().SAdd(newSKey, rKey)
					for _, key := range allStatusKeys {
						mockPipeline.EXPECT().SRem(key, rKey)
					}
					mockPipeline.EXPECT().Exec()
					mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", namespace).
						Do(func(scheduler *models.Scheduler, query string, modifier string) {
							scheduler.YAML = yamlStr
							scheduler.Game = game
						})
					mockEventForwarder1.EXPECT().Forward(status, gomock.Any()).Do(
						func(status string, infos map[string]interface{}) {
							Expect(infos["game"]).To(Equal(game))
							Expect(infos["roomId"]).To(Equal(roomName))
							Expect(infos["metadata"]).To(BeEquivalentTo(map[string]interface{}{
								"type": "sometype",
							}))
						})
					mockEventForwarder2.EXPECT().Forward(status, gomock.Any()).Do(
						func(status string, infos map[string]interface{}) {
							Expect(infos["game"]).To(Equal(game))
							Expect(infos["roomId"]).To(Equal(roomName))
							Expect(infos["metadata"]).To(BeEquivalentTo(map[string]interface{}{
								"type": "sometype",
							}))
						})

					app.Router.ServeHTTP(recorder, request)
					Expect(recorder.Body.String()).To(Equal(`{"success": true}`))
					Expect(recorder.Code).To(Equal(200))
				})
			})
		})

		Context("when redis is down", func() {
			It("returns status code of 500 if redis is unavailable", func() {
				reader := JSONFor(JSON{
					"status":    status,
					"timestamp": time.Now().Unix(),
				})
				request, _ = http.NewRequest("PUT", url, reader)

				mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", namespace).
					Do(func(scheduler *models.Scheduler, query string, modifier string) {
						scheduler.YAML = yamlStr
						scheduler.Game = game
					})

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
					"lastPing": time.Now().Unix(),
					"status":   status,
				})
				mockPipeline.EXPECT().ZAdd(pKey, gomock.Any())
				mockPipeline.EXPECT().ZRem(lKey, roomName)
				for _, key := range allStatusKeys {
					mockPipeline.EXPECT().SRem(key, rKey)
				}
				mockPipeline.EXPECT().SAdd(newSKey, rKey)
				mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

				app.Router.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				var obj map[string]interface{}
				err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(obj["code"]).To(Equal("MAE-000"))
				Expect(obj["error"]).To(Equal("Status update failed"))
				Expect(obj["description"]).To(Equal("some error in redis"))
				Expect(obj["success"]).To(Equal(false))
			})
		})
	})

	Describe("POST /scheduler/{schedulerName}/rooms/{roomName}/playerevent", func() {
		url := "/scheduler/schedulerName/rooms/roomName/playerevent"
		var app *api.App
		BeforeEach(func() {
			var err error
			app, err = api.NewApp("0.0.0.0", 9998, config, logger, false, "", mockDb, mockRedisClient, clientset)
			Expect(err).NotTo(HaveOccurred())
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
				&eventforwarder.Info{
					Plugin:    "mockplugin",
					Name:      "disabledmockfwd",
					Forwarder: mockEventForwarder3,
				},
				&eventforwarder.Info{
					Plugin:    "mockplugin",
					Name:      "unexistentmockfwd",
					Forwarder: mockEventForwarder4,
				},
				&eventforwarder.Info{
					Plugin:    "unexistentmockplugin",
					Name:      "unexistentmockfwd",
					Forwarder: mockEventForwarder5,
				},
			}
		})
		It("should error if event is nil", func() {
			reader := JSONFor(JSON{
				"roomId":    "somerid",
				"timestamp": 23412342134,
			})
			request, _ = http.NewRequest("POST", url, reader)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(422))
			var obj map[string]interface{}
			err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj["code"]).To(Equal("MAE-004"))
			Expect(obj["error"]).To(Equal("ValidationFailedError"))
			Expect(obj["description"]).To(ContainSubstring(`non zero value required`))
			Expect(obj["success"]).To(Equal(false))
		})

		It("should error if EventForwarder returns error", func() {
			event := "playerJoined"
			reader := JSONFor(JSON{
				"event":     event,
				"timestamp": 23412342134,
				"metadata":  make(map[string]interface{}),
			})
			request, _ = http.NewRequest("POST", url, reader)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name = ?",
				"schedulerName",
			).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yamlStr
				scheduler.Game = game
			})

			mockEventForwarder1.EXPECT().Forward(event, gomock.Any()).Return(int32(500), "", errors.New("no playerId specified"))

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(500))
			var obj map[string]interface{}
			err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj["code"]).To(Equal("MAE-000"))
			Expect(obj["error"]).To(Equal("Player event forward failed"))
			Expect(obj["description"]).To(ContainSubstring(`no playerId specified`))
			Expect(obj["success"]).To(Equal(false))
		})

		It("should error if EventForwarder returns status code other than 200", func() {
			event := "playerJoined"
			reader := JSONFor(JSON{
				"event":     event,
				"timestamp": 23412342134,
				"metadata":  make(map[string]interface{}),
			})
			request, _ = http.NewRequest("POST", url, reader)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name = ?",
				"schedulerName",
			).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yamlStr
				scheduler.Game = game
			})

			mockEventForwarder1.EXPECT().Forward(event, gomock.Any()).Return(int32(403), "UNAUTHORIZED", nil)
			mockEventForwarder2.EXPECT().Forward(event, gomock.Any()).Return(int32(200), "", nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(403))
			var obj map[string]interface{}
			err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj["code"]).To(Equal("MAE-000"))
			Expect(obj["error"]).To(Equal("player event forward failed"))
			Expect(obj["description"]).To(Equal("UNAUTHORIZED"))
			Expect(obj["success"]).To(Equal(false))
		})

		It("should call all enabled forwarders and return 200 if ok", func() {
			event := "playerJoined"
			reader := JSONFor(JSON{
				"event":     event,
				"timestamp": 23412342134,
				"metadata":  make(map[string]interface{}),
			})
			request, _ = http.NewRequest("POST", url, reader)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name = ?",
				"schedulerName",
			).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yamlStr
				scheduler.Game = game
			})

			mockEventForwarder1.EXPECT().Forward(event, gomock.Any()).Return(int32(200), "resp1", nil)
			mockEventForwarder2.EXPECT().Forward(event, gomock.Any()).Return(int32(200), "resp2", nil)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(200))
			var obj map[string]interface{}
			err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj["success"]).To(Equal(true))
			Expect(obj["message"]).To(Equal("resp1;resp2"))
		})
	})

	Describe("POST /scheduler/{schedulerName}/rooms/{roomName}/roomevent", func() {
		url := "/scheduler/schedulerName/rooms/roomName/roomevent"
		var app *api.App

		BeforeEach(func() {
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5000", nil))
			mockPipeline.EXPECT().Exec()

			createNamespace(namespace, clientset)
			err := createPod("roomName", namespace, clientset)
			Expect(err).NotTo(HaveOccurred())
			app, err = api.NewApp("0.0.0.0", 9998, config, logger, false, "", mockDb, mockRedisClient, clientset)
			Expect(err).NotTo(HaveOccurred())
			app.Forwarders = []*eventforwarder.Info{
				&eventforwarder.Info{
					Plugin:    "mockplugin",
					Name:      "mockfwd",
					Forwarder: mockEventForwarder1,
				},
			}
		})

		It("should error if event is nil", func() {
			reader := JSONFor(JSON{
				"roomId":    "somerid",
				"timestamp": 23412342134,
			})
			request, _ = http.NewRequest("POST", url, reader)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(422))
			var obj map[string]interface{}
			err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj["code"]).To(Equal("MAE-004"))
			Expect(obj["error"]).To(Equal("ValidationFailedError"))
			Expect(obj["description"]).To(ContainSubstring(`non zero value required`))
			Expect(obj["success"]).To(Equal(false))
		})

		It("should error if EventForwarder returns error", func() {
			event := "customevent"
			reader := JSONFor(JSON{
				"event":     event,
				"timestamp": 23412342134,
				"metadata":  make(map[string]interface{}),
			})
			request, _ = http.NewRequest("POST", url, reader)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name = ?",
				"schedulerName",
			).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yamlStr
				scheduler.Game = game
			})

			mockEventForwarder1.EXPECT().Forward("roomEvent", gomock.Any()).Return(
				int32(500), "", errors.New("some error occurred"),
			)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(500))
			var obj map[string]interface{}
			err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj["code"]).To(Equal("MAE-000"))
			Expect(obj["error"]).To(Equal("Room event forward failed"))
			Expect(obj["description"]).To(ContainSubstring(`some error occurred`))
			Expect(obj["success"]).To(Equal(false))
		})

		It("should error if EventForwarder returns code != 200", func() {
			event := "customevent"
			reader := JSONFor(JSON{
				"event":     event,
				"timestamp": 23412342134,
				"metadata":  make(map[string]interface{}),
			})
			request, _ = http.NewRequest("POST", url, reader)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name = ?",
				"schedulerName",
			).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yamlStr
				scheduler.Game = game
			})

			mockEventForwarder1.EXPECT().Forward("roomEvent", gomock.Any()).Return(
				int32(500), "nice error reason", nil,
			)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(500))
			var obj map[string]interface{}
			err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj["code"]).To(Equal("MAE-000"))
			Expect(obj["error"]).To(Equal("room event forward failed"))
			Expect(obj["description"]).To(Equal("nice error reason"))
			Expect(obj["success"]).To(Equal(false))
		})

		It("should call all enabled forwarders and return 200 if ok", func() {
			event := "customevent"
			reader := JSONFor(JSON{
				"event":     event,
				"timestamp": 23412342134,
				"metadata":  make(map[string]interface{}),
			})
			request, _ = http.NewRequest("POST", url, reader)
			mockDb.EXPECT().Query(
				gomock.Any(),
				"SELECT * FROM schedulers WHERE name = ?",
				"schedulerName",
			).Do(func(scheduler *models.Scheduler, query string, modifier string) {
				scheduler.YAML = yamlStr
				scheduler.Game = game
			})

			mockEventForwarder1.EXPECT().Forward("roomEvent", gomock.Any()).Return(
				int32(200), "all went well", nil,
			)

			app.Router.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(200))
			var obj map[string]interface{}
			err := json.Unmarshal([]byte(recorder.Body.String()), &obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj["success"]).To(Equal(true))
			Expect(obj["message"]).To(Equal("all went well"))
		})
	})

	Describe("GET /scheduler/{schedulerName}/rooms/{roomName}/address", func() {
		var (
			game      = "pong"
			image     = "pong/pong:v123"
			name      = "roomName"
			namespace = "schedulerName"
			ports     = []*models.Port{
				{
					ContainerPort: 5050,
				},
			}
			limits = &models.Resources{
				CPU:    "2",
				Memory: "128974848",
			}
			requests = &models.Resources{
				CPU:    "1",
				Memory: "64487424",
			}
			shutdownTimeout = 180
		)
		It("should return addresses", func() {
			ns := models.NewNamespace(namespace)
			err := ns.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5000", nil))
			mockPipeline.EXPECT().Exec()

			pod, err := models.NewPod(
				game,
				image,
				name,
				namespace,
				limits,
				requests,
				shutdownTimeout,
				ports,
				nil,
				nil,
				mockClientset,
				mockRedisClient,
			)
			Expect(err).NotTo(HaveOccurred())
			_, err = pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			url := fmt.Sprintf(
				"/scheduler/%s/rooms/%s/address",
				namespace,
				name,
			)
			request, err := http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())
			app.Router.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusOK))

			var obj map[string]interface{}
			err = json.Unmarshal([]byte(recorder.Body.String()), &obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj["ports"]).To(BeNil())
			Expect(obj["host"]).To(BeEmpty())
		})

		It("should return error if name doesn't exist", func() {
			ns := models.NewNamespace(namespace)
			err := ns.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5000", nil))
			mockPipeline.EXPECT().Exec()

			pod, err := models.NewPod(
				game,
				image,
				name,
				namespace,
				limits,
				requests,
				shutdownTimeout,
				ports,
				nil,
				nil,
				mockClientset,
				mockRedisClient,
			)
			Expect(err).NotTo(HaveOccurred())
			_, err = pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			namespace := "unexisting-name"
			url := fmt.Sprintf(
				"/scheduler/%s/rooms/%s/address",
				namespace,
				name,
			)
			request, err := http.NewRequest("GET", url, nil)
			Expect(err).NotTo(HaveOccurred())
			app.Router.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusUnprocessableEntity))

			var obj map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj).To(HaveKeyWithValue("code", "MAE-000"))
			Expect(obj).To(HaveKeyWithValue("description", "Pod \"roomName\" not found"))
			Expect(obj).To(HaveKeyWithValue("error", "Address handler error"))
			Expect(obj).To(HaveKeyWithValue("success", false))
		})
	})
})
