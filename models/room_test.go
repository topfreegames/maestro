package models_test

import (
	"github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	"github.com/topfreegames/maestro/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Room", func() {
	Describe("NewRoom", func() {
		It("should build correct room struct", func() {
			room := models.NewRoom("pong-free-for-all-0", "pong-free-for-all")
			Expect(room.ID).To(Equal("pong-free-for-all-0"))
			Expect(room.ConfigName).To(Equal("pong-free-for-all"))
			Expect(room.Status).To(Equal("creating"))
		})
	})

	Describe("Create Room", func() {
		It("should call the database successfully", func() {
			room := models.NewRoom("pong-free-for-all-0", "pong-free-for-all")
			mockRedisClient.EXPECT().TxPipeline()
			err := room.Create(mockRedisClient)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Set Status", func() {
		It("should call the database successfully", func() {
			name := "pong-free-for-all-0"
			scheduler := "pong-free-for-all"
			mockRedisClient.EXPECT().TxPipeline()
			room := models.NewRoom(name, scheduler)
			status := "room-ready"
			lastStatus := "createing"
			err := room.SetStatus(mockRedisClient, lastStatus, status)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Ping", func() {
		It("should call the database successfully", func() {
			name := "pong-free-for-all-0"
			scheduler := "pong-free-for-all"
			status := "room-ready"
			room := models.NewRoom(name, scheduler)
			mockRedisClient.EXPECT().HMSet(room.GetRoomRedisKey(), gomock.Any()).Return(redis.NewStatusResult("OK", nil))
			err := room.Ping(mockRedisClient, status)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Get rooms count by status", func() {
		It("should call the database successfully", func() {
			mockRedisClient.EXPECT().TxPipeline()
			_, err := models.GetRoomsCountByStatus(mockRedisClient, "config-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(db.Execs).To(HaveLen(1))
		})
	})
})
