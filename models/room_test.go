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

	"github.com/go-redis/redis"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/maestro/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Room", func() {
	schedulerName := uuid.NewV4().String()
	name := uuid.NewV4().String()

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
				"status":   "creating",
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
				"status":   "creating",
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
			status := "room-ready"
			lastStatus := "creating"
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
			status := "room-ready"
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
			status := "terminated"
			lastStatus := "room-ready"
			oldSKey := models.GetRoomStatusSetRedisKey(schedulerName, "terminating")

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SRem(oldSKey, rKey)
			mockPipeline.EXPECT().Del(rKey)
			mockPipeline.EXPECT().Exec()

			err := room.SetStatus(mockRedisClient, lastStatus, status)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if redis returns an error", func() {
			status := "room-ready"
			lastStatus := "creating"
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

	Describe("Ping", func() {
		It("should call redis successfully", func() {
			name := "pong-free-for-all-0"
			scheduler := "pong-free-for-all"
			status := "room-ready"
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
			status := "room-ready"
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
