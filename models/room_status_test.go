// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"errors"

	"github.com/go-redis/redis"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/maestro/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Room Status", func() {
	schedulerName := uuid.NewV4().String()
	creating := 1
	occupied := 2
	ready := 3
	terminating := 4

	Describe("Total", func() {
		It("should return the sum of all status counts", func() {
			rStatusCount := &models.RoomsStatusCount{
				Creating:    creating,
				Occupied:    occupied,
				Ready:       ready,
				Terminating: terminating,
			}
			total := rStatusCount.Total()
			Expect(total).To(Equal(creating + occupied + ready + terminating))
		})
	})

	Describe("Get Room Status Set Redis Key", func() {
		It("should build the redis key using the scheduler name and the status", func() {
			status := uuid.NewV4().String()
			key := models.GetRoomStatusSetRedisKey(schedulerName, status)
			Expect(key).To(ContainSubstring(schedulerName))
			Expect(key).To(ContainSubstring(status))
		})
	})

	Describe("Get rooms count by status", func() {
		It("should call redis successfully", func() {
			kCreating := models.GetRoomStatusSetRedisKey(schedulerName, "creating")
			kReady := models.GetRoomStatusSetRedisKey(schedulerName, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(schedulerName, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(schedulerName, "terminating")
			expectedCountByStatus := &models.RoomsStatusCount{
				Creating:    creating,
				Occupied:    occupied,
				Ready:       ready,
				Terminating: terminating,
			}

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating).Return(redis.NewIntResult(int64(creating), nil))
			mockPipeline.EXPECT().SCard(kReady).Return(redis.NewIntResult(int64(ready), nil))
			mockPipeline.EXPECT().SCard(kOccupied).Return(redis.NewIntResult(int64(occupied), nil))
			mockPipeline.EXPECT().SCard(kTerminating).Return(redis.NewIntResult(int64(terminating), nil))
			mockPipeline.EXPECT().Exec()

			countByStatus, err := models.GetRoomsCountByStatus(mockRedisClient, schedulerName)
			Expect(err).NotTo(HaveOccurred())
			Expect(countByStatus).To(Equal(expectedCountByStatus))
		})

		It("should return an error if redis returns an error", func() {
			kCreating := models.GetRoomStatusSetRedisKey(schedulerName, "creating")
			kReady := models.GetRoomStatusSetRedisKey(schedulerName, "ready")
			kOccupied := models.GetRoomStatusSetRedisKey(schedulerName, "occupied")
			kTerminating := models.GetRoomStatusSetRedisKey(schedulerName, "terminating")
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SCard(kCreating)
			mockPipeline.EXPECT().SCard(kReady)
			mockPipeline.EXPECT().SCard(kOccupied)
			mockPipeline.EXPECT().SCard(kTerminating)
			mockPipeline.EXPECT().Exec().Return([]redis.Cmder{}, errors.New("some error in redis"))

			_, err := models.GetRoomsCountByStatus(mockRedisClient, schedulerName)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})
	})
})
