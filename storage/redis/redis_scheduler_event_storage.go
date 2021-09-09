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

	"github.com/go-redis/redis"
	"github.com/topfreegames/extensions/redis/interfaces"
	"github.com/topfreegames/maestro/models"
)

type RedisSchedulerEventStorage struct {
	redisClient interfaces.RedisClient
}

func NewRedisSchedulerEventStorage(redisClient interfaces.RedisClient) *RedisSchedulerEventStorage {
	return &RedisSchedulerEventStorage {
		redisClient: redisClient,
	}
}

func (this *RedisSchedulerEventStorage) PersistSchedulerEvent(event *models.SchedulerEvent) error {

	member, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("scheduler event failed to parse to json: %w", err)
	}

	redisPipeline := this.redisClient.TxPipeline()

	redisPipeline.ZAdd(getSchedulerEventKey(event.SchedulerName), redis.Z{
		Score:  float64(event.CreatedAt.UnixNano()),
		Member: string(member),
	})

	_, err = redisPipeline.Exec()
	if err != nil {
		return err
	}

	return nil
}

// Load loads scheduler events from the redis using the scheduler name and a page
func (this *RedisSchedulerEventStorage) LoadSchedulerEvents(schedulerName string, page int) ([]*models.SchedulerEvent, error) {
	eventsString, err := this.redisClient.ZRangeByScore(getSchedulerEventKey(schedulerName), redis.ZRangeBy{
		Min:    "-inf",
		Max:    "+inf",
		Count:  30,
		Offset: int64((page-1)*30 + 1),
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
