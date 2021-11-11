// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package redis

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	uuid "github.com/satori/go.uuid"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
	"github.com/topfreegames/maestro/models"
	. "github.com/topfreegames/maestro/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage Suite")
}

var (
	schedulerName   string
	mockCtrl        *gomock.Controller
	mockRedisClient *redismocks.MockRedisClient
	mockPipeline    *redismocks.MockPipeliner
)

var _ = Describe("Scheduler events", func() {
	BeforeEach(func() {
		schedulerName = uuid.NewV4().String()
		mockCtrl = gomock.NewController(GinkgoT())
		mockRedisClient = redismocks.NewMockRedisClient(mockCtrl)
		mockPipeline = redismocks.NewMockPipeliner(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("PersistSchedulerEvent", func() {
		It("should return no error when event is persisted successfully", func() {
			metadata := make(map[string]interface{})
			metadata["reason"] = "rollback"

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().ZAdd(fmt.Sprintf("scheduler:%s:events", schedulerName), gomock.Any()).Times(1)
			mockPipeline.EXPECT().ZRemRangeByScore(fmt.Sprintf("scheduler:%s:events", schedulerName), "-inf", gomock.Any()).Times(1)
			mockPipeline.EXPECT().Exec().Times(1)

			event := NewSchedulerEvent("UPDATE_STARTED", schedulerName, metadata)

			err := NewRedisSchedulerEventStorage(mockRedisClient).PersistSchedulerEvent(event)
			Expect(err).To(BeNil())
		})

		It("should return error when redis pipe execution throws error", func() {
			metadata := make(map[string]interface{})
			metadata["reason"] = "rollback"

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().ZAdd(fmt.Sprintf("scheduler:%s:events", schedulerName), gomock.Any()).Times(1)
			mockPipeline.EXPECT().ZRemRangeByScore(fmt.Sprintf("scheduler:%s:events", schedulerName), gomock.Any(), gomock.Any()).Times(1)
			mockPipeline.EXPECT().Exec().Return(nil, errors.New("redis failed"))

			event := NewSchedulerEvent("UPDATE_STARTED", schedulerName, metadata)

			err := NewRedisSchedulerEventStorage(mockRedisClient).PersistSchedulerEvent(event)
			Expect(err).To(HaveOccurred())
		})

		It("should persist error event message", func() {
			metadata := make(map[string]interface{})
			metadata[ErrorMetadataName] = errors.New("Forced Error test")

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().ZAdd(fmt.Sprintf("scheduler:%s:events", schedulerName), gomock.Any()).Times(1)
			mockPipeline.EXPECT().ZRemRangeByScore(fmt.Sprintf("scheduler:%s:events", schedulerName), gomock.Any(), gomock.Any()).Times(1)
			mockPipeline.EXPECT().Exec().Return(nil, errors.New("redis failed"))

			event := NewSchedulerEvent("UPDATE_STARTED", schedulerName, metadata)

			err := NewRedisSchedulerEventStorage(mockRedisClient).PersistSchedulerEvent(event)
			Expect(err).To(HaveOccurred())
			Expect(event.Metadata).To(Equal(map[string]interface{}{
				"error": "Forced Error test",
			}))
		})

		It("should persist error event message if error is not an error", func() {
			metadata := make(map[string]interface{})
			metadata[ErrorMetadataName] = "Forced Error test"

			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().ZAdd(fmt.Sprintf("scheduler:%s:events", schedulerName), gomock.Any()).Times(1)
			mockPipeline.EXPECT().ZRemRangeByScore(fmt.Sprintf("scheduler:%s:events", schedulerName), gomock.Any(), gomock.Any()).Times(1)
			mockPipeline.EXPECT().Exec().Return(nil, errors.New("redis failed"))

			event := NewSchedulerEvent("UPDATE_STARTED", schedulerName, metadata)

			err := NewRedisSchedulerEventStorage(mockRedisClient).PersistSchedulerEvent(event)
			Expect(err).To(HaveOccurred())
			Expect(event.Metadata).To(Equal(map[string]interface{}{
				"error": "Forced Error test",
			}))
		})
	})

	Describe("LoadSchedulerEvents", func() {
		It("should return list of events and no error when events are retrieved with success", func() {
			metadata := make(map[string]interface{})
			metadata["reason"] = "rollback"
			page := 30

			createdAt := time.Now()
			expectedEvent := models.SchedulerEvent{
				Name:          "UPDATE_STARTED",
				SchedulerName: schedulerName,
				CreatedAt:     createdAt,
				Metadata:      metadata,
			}
			expectedEventString, _ := json.Marshal(expectedEvent)

			mockRedisClient.EXPECT().ZRevRangeByScore(fmt.Sprintf("scheduler:%s:events", schedulerName), redis.ZRangeBy{
				Min:    "-inf",
				Max:    "+inf",
				Count:  30,
				Offset: int64((page-1)*30 + 1),
			}).Return(redis.NewStringSliceResult([]string{string(expectedEventString)}, nil))

			events, err := NewRedisSchedulerEventStorage(mockRedisClient).LoadSchedulerEvents(schedulerName, page)
			Expect(err).To(BeNil())
			Expect(events).To(HaveLen(1))
			event := events[0]
			Expect(event.Name).To(Equal(expectedEvent.Name))
			Expect(event.SchedulerName).To(Equal(expectedEvent.SchedulerName))
			Expect(event.CreatedAt.UnixNano()).To(Equal(expectedEvent.CreatedAt.UnixNano()))
			Expect(event.Metadata).To(Equal(expectedEvent.Metadata))
		})

		It("should unmarshall metadata error correctly", func() {
			metadata := make(map[string]interface{})
			metadata["error"] = "Forced Error Test"
			page := 30

			createdAt := time.Now()
			expectedEvent := models.SchedulerEvent{
				Name:          "AUTO_SCALE_FINISHED",
				SchedulerName: schedulerName,
				CreatedAt:     createdAt,
				Metadata:      metadata,
			}
			expectedEventString, _ := json.Marshal(expectedEvent)

			mockRedisClient.EXPECT().ZRevRangeByScore(fmt.Sprintf("scheduler:%s:events", schedulerName), redis.ZRangeBy{
				Min:    "-inf",
				Max:    "+inf",
				Count:  30,
				Offset: int64((page-1)*30 + 1),
			}).Return(redis.NewStringSliceResult([]string{string(expectedEventString)}, nil))

			events, err := NewRedisSchedulerEventStorage(mockRedisClient).LoadSchedulerEvents(schedulerName, page)
			Expect(err).To(BeNil())
			Expect(events).To(HaveLen(1))
			event := events[0]
			Expect(event.Name).To(Equal(expectedEvent.Name))
			Expect(event.SchedulerName).To(Equal(expectedEvent.SchedulerName))
			Expect(event.CreatedAt.UnixNano()).To(Equal(expectedEvent.CreatedAt.UnixNano()))
			Expect(event.Metadata).To(Equal(expectedEvent.Metadata))
		})
	})
})
