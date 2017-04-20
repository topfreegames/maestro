package models_test

import (
	"time"

	"github.com/topfreegames/maestro/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scheduler", func() {
	Describe("SchedulerState", func() {
		Describe("NewSchedulerState", func() {
			It("should build correct schedulerState struct", func() {
				name := "pong-free-for-all"
				state := "in-sync"
				lastChangedAt := time.Now().Unix()
				lastScaleAt := time.Now().Unix()
				schedulerState := models.NewSchedulerState(name, state, lastChangedAt, lastScaleAt)
				Expect(schedulerState.Name).To(Equal(name))
				Expect(schedulerState.State).To(Equal(state))
				Expect(schedulerState.LastChangedAt).To(Equal(lastScaleAt))
				Expect(schedulerState.LastScaleOpAt).To(Equal(lastChangedAt))
			})
		})

		Describe("Save SchedulerState", func() {
			It("should call redis successfully", func() {
				name := "pong-free-for-all"
				state := "in-sync"
				lastChangedAt := time.Now().Unix()
				lastScaleAt := time.Now().Unix()
				schedulerState := models.NewSchedulerState(name, state, lastChangedAt, lastScaleAt)
				err := schedulerState.Save(redisClient)
				Expect(err).NotTo(HaveOccurred())
				Expect(redisClient.Hashs).To(HaveKey(name))
			})
		})

		Describe("Load SchedulerState", func() {
			It("should call redis successfully", func() {
				name := "pong-free-for-all"
				state := "in-sync"
				lastChangedAt := time.Now().Unix()
				lastScaleAt := time.Now().Unix()
				schedulerState := models.NewSchedulerState(name, state, lastChangedAt, lastScaleAt)
				err := schedulerState.Save(redisClient)
				Expect(err).NotTo(HaveOccurred())
				Expect(redisClient.Hashs).To(HaveKey(name))

				loadedState := models.NewSchedulerState(name, "", 0, 0)
				err = loadedState.Load(redisClient)
				Expect(err).NotTo(HaveOccurred())
				Expect(loadedState).To(Equal(schedulerState))
			})
		})
	})
})
