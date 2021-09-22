// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package redis

import (
	"fmt"
	"time"
	"testing"

	uuid "github.com/satori/go.uuid"
	. "github.com/topfreegames/maestro/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/topfreegames/extensions/redis"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/topfreegames/maestro/extensions"
	mtesting "github.com/topfreegames/maestro/testing"
)

var (
	schedulerName   string
	redisClient *redis.Client
	logger      *logrus.Logger
	hook        *test.Hook
	eventStorage *RedisSchedulerEventStorage
)

func TestIntStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage Integration Suite")
}

var _ = BeforeSuite(func() {
	config, err := mtesting.GetDefaultConfig()
	Expect(err).NotTo(HaveOccurred())

	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel

	redisClient, err = extensions.GetRedisClient(logger, config)
	Expect(err).NotTo(HaveOccurred())

	eventStorage = NewRedisSchedulerEventStorage(redisClient.Client)
})

var _ = Describe("Scheduler events", func() {
	BeforeEach(func() {
		schedulerName = uuid.NewV4().String()
	})

	Describe("PersistSchedulerEvent", func() {
		It("should return no error when event is persisted successfully", func() {
			metadata := make(map[string]interface{})
			metadata["reason"] = "rollback"

			event := NewSchedulerEvent("UPDATE_STARTED", schedulerName, metadata)

			err := eventStorage.PersistSchedulerEvent(event)
			Expect(err).To(BeNil())

			events, err := eventStorage.LoadSchedulerEvents(schedulerName, 1)
			Expect(err).To(BeNil())
			Expect(events).To(HaveLen(1))
		})

		It("should crop events older than limit", func() {
			metadata := make(map[string]interface{})
			metadata["reason"] = "rollback"

			oldEvent := NewSchedulerEvent("OLD_UPDATE_STARTED", schedulerName, metadata)
			oldEvent.CreatedAt = oldEvent.CreatedAt.Add(minScoreDuration).Add(-1000)
			err := eventStorage.PersistSchedulerEvent(oldEvent)
			Expect(err).To(BeNil())

			newEvent := NewSchedulerEvent("NEW_UPDATE_STARTED", schedulerName, metadata)
			err = eventStorage.PersistSchedulerEvent(newEvent)
			Expect(err).To(BeNil())

			events, err := eventStorage.LoadSchedulerEvents(schedulerName, 1)
			Expect(err).To(BeNil())
			Expect(events).To(HaveLen(1))
			Expect(events[0].Name).To(BeEquivalentTo(newEvent.Name))
		})
	})

	Describe("LoadSchedulerEvents", func() {
		It("should return list of events and no error when events are retrieved with success", func() {
			now := time.Now()
			for i := 1; i <= 100; i++ {
				event := NewSchedulerEvent(
					fmt.Sprintf("UPDATE_STARTED_%d", i), 
					schedulerName, 
					make(map[string]interface{}),
					)

				event.CreatedAt = now.Add(time.Hour * time.Duration(i))
				eventStorage.PersistSchedulerEvent(event)

			}

			events, err := eventStorage.LoadSchedulerEvents(schedulerName, 1)
			Expect(err).To(BeNil())
			Expect(events).To(HaveLen(30))
			Expect(events[0].Name).To(Equal("UPDATE_STARTED_100"))
			Expect(events[29].Name).To(Equal("UPDATE_STARTED_71"))
		})
	})
})
