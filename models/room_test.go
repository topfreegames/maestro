package models_test

import (
	"github.com/topfreegames/maestro/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Room", func() {
	Describe("NewRoom", func() {
		It("should build correct room struct", func() {
			room := models.NewRoom("pong-free-for-all-0", "pong-free-for-all")
			Expect(room.ID).To(Equal("pong-free-for-all-0"))
			Expect(room.ConfigID).To(Equal("pong-free-for-all"))
			Expect(room.Status).To(Equal("creating"))
		})
	})

	Describe("Create Room", func() {
		It("should save room in the database", func() {
			room := models.NewRoom("pong-free-for-all-0", "pong-free-for-all")
			err := room.Create(db)
			Expect(err).NotTo(HaveOccurred())
			Expect(db.Execs).To(HaveLen(1))
		})
	})

	Describe("Set Status", func() {
		It("should update a room status", func() {
			room := models.NewRoom("pong-free-for-all-0", "pong-free-for-all")
			err := room.SetStatus(db, "room-ready")
			Expect(err).NotTo(HaveOccurred())
			Expect(db.Execs).To(HaveLen(1))
		})
	})

	Describe("Get rooms count by status", func() {
		It("should return the count of rooms for each status", func() {
			_, err := models.GetRoomsCountByStatus(db, "config-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(db.Execs).To(HaveLen(1))
		})
	})
})
