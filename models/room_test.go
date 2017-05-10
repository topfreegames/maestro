// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"errors"
	"strconv"
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
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)
			now := time.Now().Unix()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
				"status":   models.StatusCreating,
				"lastPing": now,
			})
			mockPipeline.EXPECT().SAdd(sKey, rKey)
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			mockPipeline.EXPECT().Exec()

			err := room.Create(mockRedisClient)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if redis returns an error", func() {
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			sKey := models.GetRoomStatusSetRedisKey(schedulerName, room.Status)
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)
			now := time.Now().Unix()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
				"status":   models.StatusCreating,
				"lastPing": now,
			})
			mockPipeline.EXPECT().SAdd(sKey, rKey)
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			err := room.Create(mockRedisClient)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})
	})

	Describe("Set Status", func() {
		allStatus := []string{
			models.StatusCreating,
			models.StatusOccupied,
			models.StatusTerminating,
			models.StatusTerminated,
		}

		It("should call redis successfully", func() {
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			status := models.StatusReady
			newSKey := models.GetRoomStatusSetRedisKey(schedulerName, status)
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)
			now := time.Now().Unix()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
				"status":   status,
				"lastPing": now,
			})
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, st), rKey)
			}
			mockPipeline.EXPECT().SAdd(newSKey, rKey)
			mockPipeline.EXPECT().Exec()

			err := room.SetStatus(mockRedisClient, status)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove from redis is status is 'terminated'", func() {
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			status := models.StatusTerminated
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady), rKey)
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, st), rKey)
			}
			mockPipeline.EXPECT().ZRem(pKey, room.ID)
			mockPipeline.EXPECT().Del(rKey)
			mockPipeline.EXPECT().Exec()

			err := room.SetStatus(mockRedisClient, status)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if redis returns an error", func() {
			status := models.StatusReady
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			newSKey := models.GetRoomStatusSetRedisKey(schedulerName, status)
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)
			now := time.Now().Unix()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, map[string]interface{}{
				"status":   status,
				"lastPing": now,
			})
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, st), rKey)
			}
			mockPipeline.EXPECT().SAdd(newSKey, rKey)
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			err := room.SetStatus(mockRedisClient, status)
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

	Describe("ClearAll", func() {
		allStatus := []string{
			models.StatusCreating,
			models.StatusReady,
			models.StatusOccupied,
			models.StatusTerminating,
			models.StatusTerminated,
		}

		It("should call redis successfully", func() {
			name := "pong-free-for-all-0"
			scheduler := "pong-free-for-all"
			room := models.NewRoom(name, scheduler)
			rKey := room.GetRoomRedisKey()
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().Del(rKey)
			mockPipeline.EXPECT().ZRem(pKey, room.ID)
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, st), rKey)
			}
			mockPipeline.EXPECT().Exec()

			err := room.ClearAll(mockRedisClient)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if redis returns an error", func() {
			name := "pong-free-for-all-0"
			scheduler := "pong-free-for-all"
			room := models.NewRoom(name, scheduler)
			rKey := room.GetRoomRedisKey()
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().Del(rKey)
			mockPipeline.EXPECT().ZRem(pKey, room.ID)
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, st), rKey)
			}
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			err := room.ClearAll(mockRedisClient)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})
	})

	Describe("GetRoomsNoPingSince", func() {
		It("should call redis successfully", func() {
			scheduler := "pong-free-for-all"
			pKey := models.GetRoomPingRedisKey(scheduler)
			since := time.Now().Unix()

			expectedRooms := []string{"room1", "room2", "room3"}
			mockRedisClient.EXPECT().ZRangeByScore(
				pKey,
				redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(since, 10)},
			).Return(redis.NewStringSliceResult(expectedRooms, nil))
			rooms, err := models.GetRoomsNoPingSince(mockRedisClient, scheduler, since)
			Expect(err).NotTo(HaveOccurred())
			Expect(rooms).To(Equal(expectedRooms))
		})

		It("should return an error if redis returns an error", func() {
			scheduler := "pong-free-for-all"
			pKey := models.GetRoomPingRedisKey(scheduler)
			since := time.Now().Unix()

			mockRedisClient.EXPECT().ZRangeByScore(
				pKey,
				redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(since, 10)},
			).Return(redis.NewStringSliceResult([]string{}, errors.New("some error")))
			_, err := models.GetRoomsNoPingSince(mockRedisClient, scheduler, since)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error"))
		})
	})
})
