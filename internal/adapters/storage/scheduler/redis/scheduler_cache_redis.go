// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/topfreegames/maestro/internal/adapters/metrics"

	"github.com/go-redis/redis/v8"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports"
)

type redisSchedulerCache struct {
	client *redis.Client
}

var _ ports.SchedulerCache = (*redisSchedulerCache)(nil)

const schedulerCacheStorageMetricLabel = "scheduler-cache-storage"

func NewRedisSchedulerCache(client *redis.Client) *redisSchedulerCache {
	return &redisSchedulerCache{client: client}
}

func (r redisSchedulerCache) GetScheduler(ctx context.Context, schedulerName string) (scheduler *entities.Scheduler, err error) {
	schedulerCacheKey := r.buildSchedulerKey(schedulerName)
	var schedulerJson string
	metrics.RunWithMetrics(schedulerCacheStorageMetricLabel, func() error {
		schedulerJson, err = r.client.Get(ctx, schedulerCacheKey).Result()
		return err
	})
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	scheduler = &entities.Scheduler{}
	err = json.Unmarshal([]byte(schedulerJson), scheduler)
	if err != nil {
		return nil, err
	}
	return scheduler, nil
}

func (r redisSchedulerCache) SetScheduler(ctx context.Context, scheduler *entities.Scheduler, ttl time.Duration) (err error) {
	jsonScheduler, err := json.Marshal(scheduler)
	if err != nil {
		return err
	}

	schedulerCacheKey := r.buildSchedulerKey(scheduler.Name)
	metrics.RunWithMetrics(schedulerCacheStorageMetricLabel, func() error {
		err = r.client.Set(ctx, schedulerCacheKey, jsonScheduler, ttl).Err()
		return err
	})
	if err != nil {
		return err
	}

	return nil
}

func (r redisSchedulerCache) DeleteScheduler(ctx context.Context, schedulerName string) (err error) {
	metrics.RunWithMetrics(schedulerCacheStorageMetricLabel, func() error {
		err = r.client.Del(ctx, r.buildSchedulerKey(schedulerName)).Err()
		return err
	})
	return err
}

func (r redisSchedulerCache) buildSchedulerKey(schedulerName string) string {
	return fmt.Sprintf("scheduler:%s", schedulerName)
}
