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
	"strings"

	"github.com/topfreegames/maestro/internal/adapters/metrics"

	"github.com/topfreegames/maestro/internal/core/ports/errors"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/go-redis/redis/v8"
)

type redisInstanceStorage struct {
	client       *redis.Client
	scanPageSize int64
}

const instanceStorageMetricLabel = "instance-storage"

func (r redisInstanceStorage) GetInstance(ctx context.Context, scheduler string, instanceId string) (instance *game_room.Instance, err error) {
	var instanceJson string
	podMapRedisKey := getPodMapRedisKey(scheduler)
	metrics.RunWithMetrics(instanceStorageMetricLabel, func() error {
		instanceJson, err = r.client.HGet(ctx, podMapRedisKey, instanceId).Result()
		return err
	})
	if err == redis.Nil {
		return nil, errors.NewErrNotFound("instance %s not found in scheduler %s", instanceId, scheduler)
	}
	if err != nil {
		return nil, errors.NewErrUnexpected("error reading member %s in hash %s", instanceId, podMapRedisKey).WithError(err)
	}
	err = json.NewDecoder(strings.NewReader(instanceJson)).Decode(&instance)
	if err != nil {
		return nil, errors.NewErrEncoding("error unmarshalling room %s json", instanceId).WithError(err)
	}
	return instance, nil
}

func (r redisInstanceStorage) UpsertInstance(ctx context.Context, instance *game_room.Instance) error {
	if instance == nil {
		return errors.NewErrUnexpected("Cannot upsert nil instance")
	}
	instanceJson, err := json.Marshal(instance)
	if err != nil {
		return errors.NewErrUnexpected("error marshalling room %s json", instance.ID).WithError(err)
	}
	podMapRedisKey := getPodMapRedisKey(instance.SchedulerID)
	metrics.RunWithMetrics(instanceStorageMetricLabel, func() error {
		err = r.client.HSet(ctx, podMapRedisKey, instance.ID, instanceJson).Err()
		return err
	})
	if err != nil {
		return errors.NewErrUnexpected("error updating member %s from hash %s", instance.ID, podMapRedisKey).WithError(err)
	}
	return nil
}

func (r redisInstanceStorage) DeleteInstance(ctx context.Context, scheduler string, instanceId string) (err error) {
	podMapRedisKey := getPodMapRedisKey(scheduler)
	var deleted int64
	metrics.RunWithMetrics(instanceStorageMetricLabel, func() error {
		deleted, err = r.client.HDel(ctx, getPodMapRedisKey(scheduler), instanceId).Result()
		return err
	})
	if err != nil {
		return errors.NewErrUnexpected("error removing member %s from hash %s", instanceId, podMapRedisKey).WithError(err)
	}
	if deleted == 0 {
		return errors.NewErrNotFound("instance %s not found in scheduler %s", instanceId, scheduler)
	}
	return nil
}

func (r redisInstanceStorage) GetAllInstances(ctx context.Context, scheduler string) ([]*game_room.Instance, error) {
	client := r.client.WithContext(ctx)
	redisKey := getPodMapRedisKey(scheduler)
	cursor := uint64(0)
	instances := make([]*game_room.Instance, 0)
	for {
		var err error
		var results []string
		var resultCursor uint64

		metrics.RunWithMetrics(instanceStorageMetricLabel, func() error {
			results, resultCursor, err = client.HScan(ctx, redisKey, cursor, "*", r.scanPageSize).Result()
			return err
		})
		cursor = resultCursor
		if err != nil {
			return nil, errors.NewErrUnexpected("error scanning %s on redis", redisKey).WithError(err)
		}

		// results from HScan is a []string in the following ["key1","value1","key2","value2",...]
		for i := 0; i < len(results); i += 2 {
			var instance game_room.Instance
			err = json.NewDecoder(strings.NewReader(results[i+1])).Decode(&instance)
			if err != nil {
				return nil, errors.NewErrEncoding("error unmarshalling instance %s json", results[i]).WithError(err)
			}
			instances = append(instances, &instance)
		}
		if cursor == 0 {
			break
		}
	}
	return instances, nil
}

func (r redisInstanceStorage) GetInstanceCount(ctx context.Context, scheduler string) (count int, err error) {
	podMapRedisKey := getPodMapRedisKey(scheduler)
	var resultCount int64
	metrics.RunWithMetrics(instanceStorageMetricLabel, func() error {
		resultCount, err = r.client.HLen(ctx, podMapRedisKey).Result()
		return err
	})
	if err != nil {
		return 0, errors.NewErrUnexpected("error counting %s on redis", podMapRedisKey).WithError(err)
	}
	return int(resultCount), nil
}

var _ ports.GameRoomInstanceStorage = (*redisInstanceStorage)(nil)

func NewRedisInstanceStorage(client *redis.Client, scanPageSize int) *redisInstanceStorage {
	return &redisInstanceStorage{client: client, scanPageSize: int64(scanPageSize)}
}

func getPodMapRedisKey(scheduler string) string {
	return fmt.Sprintf("scheduler:%s:podMap", scheduler)
}
