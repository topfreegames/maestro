package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/go-redis/redis"
	"github.com/topfreegames/maestro/internal/adapters/instance_storage"
)

type redisInstanceStorage struct {
	client       *redis.Client
	scanPageSize int64
}

func (r redisInstanceStorage) GetInstance(ctx context.Context, scheduler string, roomId string) (*game_room.Instance, error) {
	var instance game_room.Instance
	instanceJson, err := r.client.WithContext(ctx).HGet(getPodMapRedisKey(scheduler), roomId).Result()
	if err == redis.Nil {
		return nil, instance_storage.NewInstanceNotFoundError(scheduler, roomId)
	}
	if err != nil {
		return nil, instance_storage.WrapError("error getting instance from redis", err)
	}
	err = json.NewDecoder(strings.NewReader(instanceJson)).Decode(&instance)
	if err != nil {
		return nil, instance_storage.WrapError("error unmarshalling instance json", err)
	}
	return &instance, nil
}

func (r redisInstanceStorage) AddInstance(ctx context.Context, instance *game_room.Instance) error {
	instanceJson, err := json.Marshal(instance)
	if err != nil {
		return instance_storage.WrapError("error marshalling instance json", err)
	}
	err = r.client.WithContext(ctx).HSet(getPodMapRedisKey(instance.SchedulerID), instance.ID, instanceJson).Err()
	if err != nil {
		return instance_storage.WrapError("error adding instance to redis", err)
	}
	return nil
}

func (r redisInstanceStorage) RemoveInstance(ctx context.Context, scheduler string, roomId string) error {
	deleted, err := r.client.WithContext(ctx).HDel(getPodMapRedisKey(scheduler), roomId).Result()
	if err != nil {
		return instance_storage.WrapError("error removing instance from redis", err)
	}
	if deleted == 0 {
		return instance_storage.NewInstanceNotFoundError(scheduler, roomId)
	}
	return nil
}

func (r redisInstanceStorage) GetAllInstances(ctx context.Context, scheduler string) ([]*game_room.Instance, error) {
	client := r.client.WithContext(ctx)
	redisKey := getPodMapRedisKey(scheduler)
	cursor := uint64(0)
	pods := make([]*game_room.Instance, 0)
	for {
		var err error
		var results []string

		results, cursor, err := client.HScan(redisKey, cursor, "*", r.scanPageSize).Result()
		if err != nil {
			return nil, instance_storage.WrapError("error scanning instance map on redis", err)
		}

		// results from HScan is a []string in the following ["key1","value1","key2","value2",...]
		for i := 0; i < len(results); i += 2 {
			var instance game_room.Instance
			err = json.NewDecoder(strings.NewReader(results[i+1])).Decode(&instance)
			if err != nil {
				return nil, instance_storage.WrapError("error unmarshalling instance json", err)
			}
			pods = append(pods, &instance)
		}
		if cursor == 0 {
			break
		}
	}
	return pods, nil
}

func (r redisInstanceStorage) GetInstanceCount(ctx context.Context, scheduler string) (int, error) {
	count, err := r.client.WithContext(ctx).HLen(getPodMapRedisKey(scheduler)).Result()
	if err != nil {
		return 0, instance_storage.WrapError("error counting instances on redis", err)
	}
	return int(count), nil
}

var _ instance_storage.RoomInstanceStorage = (*redisInstanceStorage)(nil)

func NewRedisInstanceStorage(client *redis.Client, scanPageSize int) *redisInstanceStorage {
	return &redisInstanceStorage{client: client, scanPageSize: int64(scanPageSize)}
}

func getPodMapRedisKey(scheduler string) string {
	return fmt.Sprintf("scheduler:%s:podMap", scheduler)
}
