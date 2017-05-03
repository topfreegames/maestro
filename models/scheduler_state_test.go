package models_test

import (
	"errors"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/topfreegames/maestro/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SchedulerState", func() {
	name := "pong-free-for-all"
	state := "in-sync"
	lastChangedAt := time.Now().Unix()
	lastScaleAt := time.Now().Unix()

	Describe("NewSchedulerState", func() {
		It("should build correct schedulerState struct", func() {
			schedulerState := models.NewSchedulerState(name, state, lastChangedAt, lastScaleAt)
			Expect(schedulerState.Name).To(Equal(name))
			Expect(schedulerState.State).To(Equal(state))
			Expect(schedulerState.LastChangedAt).To(Equal(lastScaleAt))
			Expect(schedulerState.LastScaleOpAt).To(Equal(lastChangedAt))
		})
	})

	Describe("Save SchedulerState", func() {
		It("should call redis successfully", func() {
			mockRedisClient.EXPECT().HMSet(name, map[string]interface{}{
				"state":         state,
				"lastChangedAt": lastChangedAt,
				"lastScaleOpAt": lastScaleAt,
			}).Return(redis.NewStatusResult("OK", nil))
			schedulerState := models.NewSchedulerState(name, state, lastChangedAt, lastScaleAt)
			err := schedulerState.Save(mockRedisClient)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if redis returns an error", func() {
			mockRedisClient.EXPECT().HMSet(name, map[string]interface{}{
				"state":         state,
				"lastChangedAt": lastChangedAt,
				"lastScaleOpAt": lastScaleAt,
			}).Return(redis.NewStatusResult("", errors.New("some error in redis")))
			schedulerState := models.NewSchedulerState(name, state, lastChangedAt, lastScaleAt)
			err := schedulerState.Save(mockRedisClient)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})
	})

	Describe("Load SchedulerState", func() {
		It("should call redis successfully", func() {
			m := map[string]interface{}{
				"state":         state,
				"lastChangedAt": lastChangedAt,
				"lastScaleOpAt": lastScaleAt,
			}
			mockRedisClient.EXPECT().HMSet(name, m).Return(redis.NewStatusResult("OK", nil))
			schedulerState := models.NewSchedulerState(name, state, lastChangedAt, lastScaleAt)
			err := schedulerState.Save(mockRedisClient)
			Expect(err).NotTo(HaveOccurred())

			loadedState := models.NewSchedulerState(name, "", 0, 0)
			mRes := map[string]string{
				"state":         state,
				"lastChangedAt": strconv.Itoa(int(lastChangedAt)),
				"lastScaleOpAt": strconv.Itoa(int(lastScaleAt)),
			}
			mockRedisClient.EXPECT().HGetAll(name).Return(redis.NewStringStringMapResult(mRes, nil))
			err = loadedState.Load(mockRedisClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(loadedState).To(Equal(schedulerState))
		})

		It("should return error if redis returns an error", func() {
			m := map[string]interface{}{
				"state":         state,
				"lastChangedAt": lastChangedAt,
				"lastScaleOpAt": lastScaleAt,
			}
			mockRedisClient.EXPECT().HMSet(name, m).Return(&redis.StatusCmd{})
			schedulerState := models.NewSchedulerState(name, state, lastChangedAt, lastScaleAt)
			err := schedulerState.Save(mockRedisClient)
			Expect(err).NotTo(HaveOccurred())

			loadedState := models.NewSchedulerState(name, "", 0, 0)
			mockRedisClient.EXPECT().HGetAll(name).Return(redis.NewStringStringMapResult(map[string]string{}, errors.New("some error in redis")))
			err = loadedState.Load(mockRedisClient)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("some error in redis"))
		})
	})
})
