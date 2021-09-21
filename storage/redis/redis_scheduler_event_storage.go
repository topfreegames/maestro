// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package redis

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/topfreegames/extensions/redis/interfaces"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/storage"
)

var schedulerEventsPageSize = 30
var minScoreDuration = -14 * 24 * time.Hour

type RedisSchedulerEventStorage struct {
	redisClient interfaces.RedisClient
}

func NewRedisSchedulerEventStorage(redisClient interfaces.RedisClient) *RedisSchedulerEventStorage {
	return &RedisSchedulerEventStorage {
		redisClient: redisClient,
	}
}

var _ storage.SchedulerEventStorage = (*RedisSchedulerEventStorage)(nil)

func (es *RedisSchedulerEventStorage) PersistSchedulerEvent(event *models.SchedulerEvent) error {

	member, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("scheduler event failed to parse to json: %w", err)
	}

	redisPipeline := es.redisClient.TxPipeline()
	score := float64(event.CreatedAt.UnixNano()	/ int64(time.Millisecond))
	redisPipeline.ZAdd(getSchedulerEventKey(event.SchedulerName), redis.Z{
		Score:  score,
		Member: string(member),
	})

	minScore := time.Now().Add(minScoreDuration).UnixNano() / int64(time.Millisecond)
	redisPipeline.ZRemRangeByScore(getSchedulerEventKey(event.SchedulerName), "-inf", fmt.Sprintf("(%d", minScore))

	_, err = redisPipeline.Exec()
	if err != nil {
		return fmt.Errorf("scheduler event failed to persist on redis: %w", err)
	}

	return nil
}

// Load loads scheduler events from the redis using the scheduler name and a page
func (es *RedisSchedulerEventStorage) LoadSchedulerEvents(schedulerName string, page int) ([]*models.SchedulerEvent, error) {
	offset := int64((page-1)*schedulerEventsPageSize + 1)
	if offset == 1 {
		offset = 0
	}

	eventsString, err := es.redisClient.ZRevRangeByScore(getSchedulerEventKey(schedulerName), redis.ZRangeBy{
		Min:    "-inf",
		Max:    "+inf",
		Count:  int64(schedulerEventsPageSize),
		Offset: offset,
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve scheduler %s events: %w", schedulerName, err)
	}

	events := make([]*models.SchedulerEvent, len(eventsString))
	for index, eventString := range eventsString {
		var event models.SchedulerEvent
		err = json.Unmarshal([]byte(eventString), &event)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal scheduler %s event: %w", schedulerName, err)
		}
		events[index] = &event
	}

	return events, nil
}

func getSchedulerEventKey(schedulerName string) string {
	return fmt.Sprintf("scheduler:%s:events", schedulerName)
}
