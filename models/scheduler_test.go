package models_test

import (
	"strconv"
	"time"

	"github.com/go-redis/redis"
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
				redisClient.EXPECT().HMSet("pong-free-for-all", map[string]interface{}{
					"state":         state,
					"lastChangedAt": lastChangedAt,
					"lastScaleOpAt": lastScaleAt,
				}).Return(&redis.StatusCmd{})
				schedulerState := models.NewSchedulerState(name, state, lastChangedAt, lastScaleAt)
				err := schedulerState.Save(redisClient)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("Load SchedulerState", func() {
			It("should call redis successfully", func() {
				name := "pong-free-for-all"
				state := "in-sync"
				lastChangedAt := time.Now().Unix()
				lastScaleAt := time.Now().Unix()
				m := map[string]interface{}{
					"state":         state,
					"lastChangedAt": lastChangedAt,
					"lastScaleOpAt": lastScaleAt,
				}
				redisClient.EXPECT().HMSet("pong-free-for-all", m).Return(&redis.StatusCmd{})
				schedulerState := models.NewSchedulerState(name, state, lastChangedAt, lastScaleAt)
				err := schedulerState.Save(redisClient)
				Expect(err).NotTo(HaveOccurred())

				loadedState := models.NewSchedulerState(name, "", 0, 0)
				mRes := map[string]string{
					"state":         state,
					"lastChangedAt": strconv.Itoa(int(lastChangedAt)),
					"lastScaleOpAt": strconv.Itoa(int(lastScaleAt)),
				}
				redisClient.EXPECT().HGetAll("pong-free-for-all").Return(redis.NewStringStringMapResult(mRes, nil))
				err = loadedState.Load(redisClient)
				Expect(err).NotTo(HaveOccurred())
				Expect(loadedState).To(Equal(schedulerState))
			})
		})
	})
})
