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
	"time"

	"github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/models"
	. "github.com/topfreegames/maestro/testing"
)

var _ = Describe("Room Handler", func() {
	var request *http.Request
	var recorder *httptest.ResponseRecorder

	BeforeEach(func() {
		// Record HTTP responses.
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

		Context("when redis is down", func() {
			It("returns status code of 500 if redis is unavailable", func() {
				reader := JSONFor(JSON{
					"timestamp": time.Now().Unix(),
					"status":    status,
				})
				request, _ = http.NewRequest("PUT", url, reader)

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

		Context("when all services are healthy", func() {
			It("returns a status code of 200 and success body", func() {
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
		})

		Context("when redis is down", func() {
			It("returns status code of 500 if redis is unavailable", func() {
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

			pod := models.NewPod(
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
			)
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

			pod := models.NewPod(
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
			)
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
