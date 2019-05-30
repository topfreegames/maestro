// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/maestro/models"
)

var _ = Describe("Room", func() {
	schedulerName := uuid.NewV4().String()
	name := uuid.NewV4().String()
	var room *models.Room
	// configYaml := &models.ConfigYAML{Game: "game-name"}
	scheduler := models.NewScheduler(name, "game-name", "")

	AfterEach(func() {
		mr := &models.MixedMetricsReporter{}
		err := room.ClearAll(redisClient.Client, mr)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("SetStatus", func() {
		It("should set status ready", func() {
			room = models.NewRoom(name, schedulerName)
			now := time.Now().Unix()
			payload := &models.RoomStatusPayload{
				Status: models.StatusReady,
			}
			_, err := room.SetStatus(redisClient.Client, mockDb, mmr, payload, scheduler)
			Expect(err).NotTo(HaveOccurred())
			pipe := redisClient.Client.TxPipeline()
			status := pipe.HMGet(room.GetRoomRedisKey(), "status")
			lastPing := pipe.HMGet(room.GetRoomRedisKey(), "lastPing")
			roomsLastPing := pipe.ZRange(models.GetRoomPingRedisKey(schedulerName), int64(0), int64(-1))
			roomsOccupied := pipe.ZRange(models.GetLastStatusRedisKey(schedulerName, models.StatusOccupied), int64(0), int64(-1))

			_, err = pipe.Exec()
			Expect(err).NotTo(HaveOccurred())

			times, err := lastPing.Result()
			Expect(err).NotTo(HaveOccurred())
			lastPingTime, err := strconv.ParseInt(times[0].(string), 10, 64)
			Expect(err).NotTo(HaveOccurred())

			Expect(err).NotTo(HaveOccurred())
			Expect(status.Val()).To(ContainElement(models.StatusReady))
			Expect(lastPingTime).To(BeNumerically("~", now, 1))
			Expect(roomsOccupied.Val()).To(BeEmpty())

			roomCount, err := models.GetRoomsCountByStatus(redisClient.Client, schedulerName)
			Expect(err).NotTo(HaveOccurred())
			Expect(roomCount.Creating).To(BeZero())
			Expect(roomCount.Occupied).To(BeZero())
			Expect(roomCount.Ready).To(Equal(1))
			Expect(roomCount.Terminating).To(BeZero())

			Expect(roomsLastPing.Val()).To(ContainElement(name))
		})

		It("should set status occupied", func() {
			room = models.NewRoom(name, schedulerName)
			now := time.Now().Unix()
			payload := &models.RoomStatusPayload{
				Status: models.StatusOccupied,
			}
			_, err := room.SetStatus(redisClient.Client, mockDb, mmr, payload, scheduler)
			Expect(err).NotTo(HaveOccurred())
			pipe := redisClient.Client.TxPipeline()
			status := pipe.HMGet(room.GetRoomRedisKey(), "status")
			lastPing := pipe.HMGet(room.GetRoomRedisKey(), "lastPing")
			roomsLastPing := pipe.ZRange(models.GetRoomPingRedisKey(schedulerName), int64(0), int64(-1))
			roomsOccupied := pipe.ZRange(models.GetLastStatusRedisKey(schedulerName, models.StatusOccupied), int64(0), int64(-1))

			_, err = pipe.Exec()
			Expect(err).NotTo(HaveOccurred())

			times, err := lastPing.Result()
			Expect(err).NotTo(HaveOccurred())
			lastPingTime, err := strconv.ParseInt(times[0].(string), 10, 64)
			Expect(err).NotTo(HaveOccurred())

			Expect(err).NotTo(HaveOccurred())
			Expect(status.Val()).To(ContainElement(models.StatusOccupied))
			Expect(lastPingTime).To(BeNumerically("~", now, 1))
			Expect(roomsOccupied.Val()).To(ContainElement(name))

			roomCount, err := models.GetRoomsCountByStatus(redisClient.Client, schedulerName)
			Expect(err).NotTo(HaveOccurred())
			Expect(roomCount.Creating).To(BeZero())
			Expect(roomCount.Occupied).To(Equal(1))
			Expect(roomCount.Ready).To(BeZero())
			Expect(roomCount.Terminating).To(BeZero())

			Expect(roomsLastPing.Val()).To(ContainElement(name))
		})

		It("should not update timestamp if status is still occupied", func() {
			room = models.NewRoom(name, schedulerName)
			now := time.Now().UnixNano()
			payload := &models.RoomStatusPayload{
				Status: models.StatusReady,
			}
			_, err := room.SetStatus(redisClient.Client, mockDb, mmr, payload, scheduler)
			Expect(err).NotTo(HaveOccurred())

			pipe := redisClient.Client.TxPipeline()
			lastPing := pipe.HMGet(room.GetRoomRedisKey(), "lastPing")
			_, err = pipe.Exec()
			Expect(err).NotTo(HaveOccurred())
			times, err := lastPing.Result()
			Expect(err).NotTo(HaveOccurred())
			lastPingTime, err := strconv.ParseInt(times[0].(string), 10, 64)
			Expect(err).NotTo(HaveOccurred())
			Expect(lastPingTime).NotTo(BeNumerically("~", now, 1000))

			time.Sleep(100 * time.Millisecond)
			_, err = room.SetStatus(redisClient.Client, mockDb, mmr, payload, scheduler)
			Expect(err).NotTo(HaveOccurred())

			pipe = redisClient.Client.TxPipeline()
			lastPing = pipe.HMGet(room.GetRoomRedisKey(), "lastPing")
			_, err = pipe.Exec()
			Expect(err).NotTo(HaveOccurred())

			times, err = lastPing.Result()
			Expect(err).NotTo(HaveOccurred())
			lastPingTime, err = strconv.ParseInt(times[0].(string), 10, 64)
			Expect(err).NotTo(HaveOccurred())
			Expect(lastPingTime).NotTo(BeNumerically("~", now, 1000))
		})
	})
})
