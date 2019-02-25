// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	"github.com/topfreegames/maestro/models"

	goredis "github.com/go-redis/redis"
	uuid "github.com/satori/go.uuid"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
)

const (
	yamlRoom = `
name: %s
game: game-name
limits:
  memory: "128Mi"
  cpu: "1"
autoscaling:
  min: 100
  up:
    delta: 10
    trigger:
      usage: 70
      time: 600
      threshold: 80
    cooldown: 300
  down:
    delta: 2
    trigger:
      usage: 50
      time: 900
      threshold: 80
    cooldown: 300
cmd:
  - "./room-binary"
  - "-serverType"
  - "6a8e136b-2dc1-417e-bbe8-0f0a2d2df431"
`
	yamlRoom2 = `
name: %s
game: game-name
requests:
  memory: "128Mi"
  cpu: "1"
autoscaling:
  min: 100
  up:
    delta: 10
    trigger:
      usage: 70
      time: 600
      threshold: 80
    cooldown: 300
    metricsTrigger:
    - type: cpu
      delta: 2
      time: 60
      usage: 80
      threshold: 80
      limit: 90
  down:
    delta: 2
    trigger:
      usage: 50
      time: 900
      threshold: 80
    metricsTrigger:
    - type: mem
      delta: 2
      time: 60
      usage: 80
      threshold: 80
      limit: 90
    cooldown: 300
cmd:
  - "./room-binary"
`
)

var _ = Describe("Room", func() {
	var (
		schedulerName = uuid.NewV4().String()
		name          = uuid.NewV4().String()
		scheduler     *models.Scheduler
	)

	reportStatus := func(scheduler, status, roomKey, statusKey string) {
		mockRedisClient.EXPECT().HGetAll(roomKey).
			Return(redis.NewStringStringMapResult(map[string]string{
				"status": "diff",
			}, nil))

		mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
		mockPipeline.EXPECT().SCard(statusKey).Return(redis.NewIntResult(int64(5), nil))
		mockPipeline.EXPECT().Exec()

		mr.EXPECT().Report("gru.status", map[string]interface{}{
			reportersConstants.TagGame:      "game-name",
			reportersConstants.TagScheduler: scheduler,
			"status":                        status,
			"gauge":                         "5",
		})
	}

	BeforeEach(func() {
		scheduler = &models.Scheduler{
			Name: schedulerName,
			Game: "game-name",
			YAML: fmt.Sprintf(yamlRoom, schedulerName),
		}
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
			metric := uuid.NewV4().String()
			key := models.GetRoomMetricsRedisKey(schedulerName, metric)
			Expect(key).To(ContainSubstring(schedulerName))
			Expect(key).To(ContainSubstring(metric))
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
			mockPipeline.EXPECT().HMSet(rKey, gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", now, 1))
				},
			)
			mockPipeline.EXPECT().SAdd(sKey, rKey)
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			mockPipeline.EXPECT().Exec()
			reportStatus(room.SchedulerName, room.Status, rKey, sKey)

			err := room.Create(mockRedisClient, mockDb, mmr, scheduler)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if redis returns an error", func() {
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			sKey := models.GetRoomStatusSetRedisKey(schedulerName, room.Status)
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)
			now := time.Now().Unix()

			mockRedisClient.EXPECT().HGetAll(rKey).
				Return(redis.NewStringStringMapResult(map[string]string{
					"status": "diff",
				}, nil))

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(models.StatusCreating))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", now, 1))
				},
			)
			mockPipeline.EXPECT().SAdd(sKey, rKey)
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			err := room.Create(mockRedisClient, mockDb, mmr, scheduler)
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
			mockPipeline.EXPECT().HMSet(rKey, gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(status))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", now, 1))
				},
			)
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, st), rKey)
			}
			mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(schedulerName, models.StatusOccupied), name)
			mockPipeline.EXPECT().SAdd(newSKey, rKey)

			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusCreating),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady),
			).Return(goredis.NewIntResult(5, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusOccupied),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusTerminating),
			).Return(goredis.NewIntResult(0, nil))

			mockPipeline.EXPECT().Exec()
			reportStatus(room.SchedulerName, status, rKey, newSKey)

			roomsCountByStatus, err := room.SetStatus(mockRedisClient, mockDb, mmr, status, scheduler, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(roomsCountByStatus).NotTo(BeNil())
			Expect(roomsCountByStatus.Total()).To(Equal(5))
		})

		It("should remove from redis is status is 'terminated'", func() {
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			status := models.StatusTerminated
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady), rKey)
			mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(schedulerName, models.StatusReady), name)
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, st), rKey)
				mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(schedulerName, st), name)
			}

			mockPipeline.EXPECT().ZRem(models.GetRoomMetricsRedisKey(schedulerName, "cpu"), room.ID)
			mockPipeline.EXPECT().ZRem(models.GetRoomMetricsRedisKey(schedulerName, "mem"), room.ID)
			mockPipeline.EXPECT().ZRem(pKey, room.ID)
			mockPipeline.EXPECT().Del(rKey)
			mockPipeline.EXPECT().Exec()

			roomsCountByStatus, err := room.SetStatus(mockRedisClient, mockDb, mmr, status, scheduler, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(roomsCountByStatus).To(BeNil())
		})

		It("should return an error if redis returns an error", func() {
			status := models.StatusReady
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			newSKey := models.GetRoomStatusSetRedisKey(schedulerName, status)
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)
			now := time.Now().Unix()

			mockRedisClient.EXPECT().HGetAll(rKey).
				Return(redis.NewStringStringMapResult(map[string]string{
					"status": "diff",
				}, nil))

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(status))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", now, 1))
				},
			)
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(schedulerName, models.StatusOccupied), name)
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, st), rKey)
			}
			mockPipeline.EXPECT().SAdd(newSKey, rKey)
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusCreating),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady),
			).Return(goredis.NewIntResult(5, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusOccupied),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusTerminating),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			roomsCountByStatus, err := room.SetStatus(mockRedisClient, mockDb, mmr, status, scheduler, true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
			Expect(roomsCountByStatus).To(BeNil())
		})

		It("should insert into zadd if status is occupied", func() {
			status := models.StatusOccupied
			room := models.NewRoom(name, schedulerName)
			rKey := room.GetRoomRedisKey()
			newSKey := models.GetRoomStatusSetRedisKey(schedulerName, status)
			pKey := models.GetRoomPingRedisKey(room.SchedulerName)
			now := time.Now().Unix()

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().HMSet(rKey, gomock.Any()).Do(
				func(schedulerName string, statusInfo map[string]interface{}) {
					Expect(statusInfo["status"]).To(Equal(status))
					Expect(statusInfo["lastPing"]).To(BeNumerically("~", now, 1))
				},
			)
			mockPipeline.EXPECT().ZAdd(pKey, redis.Z{float64(now), room.ID})
			mockPipeline.EXPECT().Eval(models.ZaddIfNotExists, gomock.Any(), name)
			allStatus := []string{models.StatusCreating, models.StatusReady, models.StatusTerminating, models.StatusTerminated}
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(schedulerName, st), rKey)
			}
			mockPipeline.EXPECT().SAdd(newSKey, rKey)
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusCreating),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady),
			).Return(goredis.NewIntResult(5, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusOccupied),
			).Return(goredis.NewIntResult(5, nil))
			mockPipeline.EXPECT().SCard(
				models.GetRoomStatusSetRedisKey(schedulerName, models.StatusTerminating),
			).Return(goredis.NewIntResult(0, nil))
			mockPipeline.EXPECT().Exec()
			reportStatus(room.SchedulerName, status, rKey, newSKey)

			roomsCountByStatus, err := room.SetStatus(mockRedisClient, mockDb, mmr, status, scheduler, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(roomsCountByStatus.Total()).To(Equal(10))
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

		allMetrics := []string{
			string(models.CPUAutoScalingPolicyType),
			string(models.MemAutoScalingPolicyType),
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
			for _, mt := range allMetrics {
				mockPipeline.EXPECT().ZRem(models.GetRoomMetricsRedisKey(room.SchedulerName, mt), room.ID)
			}
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, st), rKey)
				mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(room.SchedulerName, st), room.ID)
			}
			mockPipeline.EXPECT().Exec()

			err := room.ClearAll(mockRedisClient, mmr)
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
			for _, mt := range allMetrics {
				mockPipeline.EXPECT().ZRem(models.GetRoomMetricsRedisKey(room.SchedulerName, mt), room.ID)
			}
			mockPipeline.EXPECT().ZRem(pKey, room.ID)
			for _, st := range allStatus {
				mockPipeline.EXPECT().SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, st), rKey)
				mockPipeline.EXPECT().ZRem(models.GetLastStatusRedisKey(room.SchedulerName, st), room.ID)
			}
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			err := room.ClearAll(mockRedisClient, mmr)
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
			rooms, err := models.GetRoomsNoPingSince(mockRedisClient, scheduler, since, mmr)
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
			_, err := models.GetRoomsNoPingSince(mockRedisClient, scheduler, since, mmr)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error"))
		})
	})

	Describe("GetRoomsByMetric", func() {
		Context("for metric scaling policy", func() {
			It("should call redis successfully", func() {
				metric := "cpu"
				scheduler := "pong-free-for-all"
				size := 3
				pKey := models.GetRoomMetricsRedisKey(scheduler, metric)
				readyKey := models.GetRoomStatusSetRedisKey(scheduler, models.StatusReady)
				occupiedKey := models.GetRoomStatusSetRedisKey(scheduler, models.StatusOccupied)

				expectedRooms := []string{"room1", "room2", "room3"}
				for _, room := range expectedRooms {
					roomObj := models.NewRoom(room, scheduler)
					mockPipeline.EXPECT().SIsMember(readyKey, roomObj.GetRoomRedisKey()).Return(
						redis.NewBoolResult(true, nil))
					mockPipeline.EXPECT().SIsMember(occupiedKey, roomObj.GetRoomRedisKey()).Return(
						redis.NewBoolResult(true, nil))
				}

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().ZRange(pKey, int64(0), int64(size-1)).Return(
					redis.NewStringSliceResult(expectedRooms, nil))
				mockPipeline.EXPECT().Exec().Times(2)

				rooms, err := models.GetRoomsByMetric(mockRedisClient, scheduler, metric, size, mmr)
				Expect(err).NotTo(HaveOccurred())
				Expect(rooms).To(Equal(expectedRooms))
			})

			It("should return an error if redis returns an error", func() {
				metric := "cpu"
				scheduler := "pong-free-for-all"
				size := 3
				pKey := models.GetRoomMetricsRedisKey(scheduler, metric)

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().ZRange(pKey, int64(0), int64(size-1)).Return(
					redis.NewStringSliceResult([]string{}, errors.New("some error")))
				mockPipeline.EXPECT().Exec()

				_, err := models.GetRoomsByMetric(mockRedisClient, scheduler, metric, size, mmr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("some error"))
			})
		})

		Context("for room or legacy scaling policy", func() {
			It("should call redis successfully", func() {
				metric := "room"
				scheduler := "pong-free-for-all"
				size := 3
				pKey := models.GetRoomStatusSetRedisKey(scheduler, models.StatusReady)

				expectedRooms := []string{"room1", "room2", "room3"}
				expectedRet := make([]string, len(expectedRooms))
				for idx, room := range expectedRooms {
					r := &models.Room{SchedulerName: scheduler, ID: room}
					expectedRet[idx] = r.GetRoomRedisKey()
				}
				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().SRandMemberN(pKey, int64(size)).Return(
					redis.NewStringSliceResult(expectedRet, nil))
				mockPipeline.EXPECT().Exec()

				rooms, err := models.GetRoomsByMetric(mockRedisClient, scheduler, metric, size, mmr)
				Expect(err).NotTo(HaveOccurred())
				Expect(rooms).To(Equal(expectedRooms))
			})

			It("should return an error if redis returns an error", func() {
				metric := "room"
				scheduler := "pong-free-for-all"
				size := 3
				pKey := models.GetRoomStatusSetRedisKey(scheduler, models.StatusReady)

				mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
				mockPipeline.EXPECT().SRandMemberN(pKey, int64(size)).Return(
					redis.NewStringSliceResult([]string{}, errors.New("some error")))
				mockPipeline.EXPECT().Exec()

				_, err := models.GetRoomsByMetric(mockRedisClient, scheduler, metric, size, mmr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("some error"))
			})
		})
	})
})
