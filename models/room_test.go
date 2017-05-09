// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"errors"
	"time"

	"k8s.io/client-go/kubernetes/fake"

	"github.com/go-redis/redis"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/maestro/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Room", func() {
	var clientset *fake.Clientset
	schedulerName := uuid.NewV4().String()
	name := uuid.NewV4().String()

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
	})

	Describe("NewRoom", func() {
		It("should build correct room struct", func() {
			room := models.NewRoom(name, schedulerName)
			Expect(room.ID).To(Equal(name))
			Expect(room.SchedulerName).To(Equal(schedulerName))
			Expect(room.Status).To(Equal("creating"))
			Expect(room.LastPingAt).To(BeEquivalentTo(0))
		})
	})

	Describe("Get Room Redis Key", func() {
		It("should build the redis key using the scheduler name and the id", func() {
			room := models.NewRoom(name, schedulerName)
			key := room.GetRoomRedisKey()
			Expect(key).To(ContainSubstring(schedulerName))
			Expect(key).To(ContainSubstring(room.ID))
		})
	})

	Describe("Create Room", func() {
		It("should call redis successfully", func() {
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			sKey := models.GetRoomStatusSetRedisKey(schedulerName, room.Status)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
				"status":   models.StatusCreating,
				"lastPing": int64(0),
			})
			mockPipeline.EXPECT().SAdd(sKey, rKey)
			mockPipeline.EXPECT().Exec()

			err := room.Create(mockRedisClient)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if redis returns an error", func() {
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			sKey := models.GetRoomStatusSetRedisKey(schedulerName, room.Status)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
				"status":   models.StatusCreating,
				"lastPing": int64(0),
			})
			mockPipeline.EXPECT().SAdd(sKey, rKey)
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			err := room.Create(mockRedisClient)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})
	})

	Describe("Set Status", func() {
		It("should call redis successfully", func() {
			status := models.StatusReady
			lastStatus := models.StatusCreating
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			oldSKey := models.GetRoomStatusSetRedisKey(schedulerName, lastStatus)
			newSKey := models.GetRoomStatusSetRedisKey(schedulerName, status)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{"status": status})
			mockPipeline.EXPECT().SRem(oldSKey, rKey)
			mockPipeline.EXPECT().SAdd(newSKey, rKey)
			mockPipeline.EXPECT().Exec()

			err := room.SetStatus(mockRedisClient, lastStatus, status)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call redis successfully with empty last status", func() {
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			status := models.StatusReady
			lastStatus := ""
			newSKey := models.GetRoomStatusSetRedisKey(schedulerName, status)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{"status": status})
			mockPipeline.EXPECT().SAdd(newSKey, rKey)
			mockPipeline.EXPECT().Exec()

			err := room.SetStatus(mockRedisClient, lastStatus, status)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove from redis is status is 'terminated'", func() {
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			status := models.StatusTerminated
			lastStatus := models.StatusReady
			oldSKey := models.GetRoomStatusSetRedisKey(schedulerName, "terminating")

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SRem(oldSKey, rKey)
			mockPipeline.EXPECT().Del(rKey)
			mockPipeline.EXPECT().Exec()

			err := room.SetStatus(mockRedisClient, lastStatus, status)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if redis returns an error", func() {
			status := models.StatusReady
			lastStatus := models.StatusCreating
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			oldSKey := models.GetRoomStatusSetRedisKey(schedulerName, lastStatus)
			newSKey := models.GetRoomStatusSetRedisKey(schedulerName, status)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{"status": status})
			mockPipeline.EXPECT().SRem(oldSKey, rKey)
			mockPipeline.EXPECT().SAdd(newSKey, rKey)
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			err := room.SetStatus(mockRedisClient, lastStatus, status)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})
	})

	Describe("GetAddresses", func() {
		It("should not crash if pod does not exist", func() {
			name := "pong-free-for-all-0"
			scheduler := "pong-free-for-all"
			room := models.NewRoom(name, scheduler)
			_, err := room.GetAddresses(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(`Pod "pong-free-for-all-0" not found`))
		})

		It("should return no address if no node assigned to the room", func() {
			command := []string{
				"./room-binary",
				"-serverType",
				"6a8e136b-2dc1-417e-bbe8-0f0a2d2df431",
			}
			env := []*models.EnvVar{
				{
					Name:  "EXAMPLE_ENV_VAR",
					Value: "examplevalue",
				},
				{
					Name:  "ANOTHER_ENV_VAR",
					Value: "anothervalue",
				},
			}
			game := "pong"
			image := "pong/pong:v123"
			name := "pong-free-for-all-0"
			namespace := "pong-free-for-all"
			ports := []*models.Port{
				{
					ContainerPort: 5050,
				},
				{
					ContainerPort: 8888,
				},
			}
			resourcesLimitsCPU := "2"
			resourcesLimitsMemory := "128974848"
			resourcesRequestsCPU := "1"
			resourcesRequestsMemory := "64487424"
			shutdownTimeout := 180

			pod := models.NewPod(
				game,
				image,
				name,
				namespace,
				resourcesLimitsCPU,
				resourcesLimitsMemory,
				resourcesRequestsCPU,
				resourcesRequestsMemory,
				shutdownTimeout,
				ports,
				command,
				env,
			)
			_, err := pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())
			room := models.NewRoom(name, namespace)
			addresses, err := room.GetAddresses(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(addresses.Addresses)).To(Equal(0))
		})
	})

	Describe("Ping", func() {
		It("should call redis successfully", func() {
			name := "pong-free-for-all-0"
			scheduler := "pong-free-for-all"
			status := models.StatusReady
			room := models.NewRoom(name, scheduler)
			rKey := room.GetRoomRedisKey()

			mockRedisClient.EXPECT().HMSet(rKey, map[string]interface{}{
				"lastPing": time.Now().Unix(),
				"status":   status,
			}).Return(redis.NewStatusResult("OK", nil))

			err := room.Ping(mockRedisClient, status)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if redis returns an error", func() {
			name := "pong-free-for-all-0"
			scheduler := "pong-free-for-all"
			status := models.StatusReady
			room := models.NewRoom(name, scheduler)
			rKey := room.GetRoomRedisKey()

			mockRedisClient.EXPECT().HMSet(rKey, map[string]interface{}{
				"lastPing": time.Now().Unix(),
				"status":   status,
			}).Return(redis.NewStatusResult("", errors.New("some error in redis")))

			err := room.Ping(mockRedisClient, status)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})
	})
})
