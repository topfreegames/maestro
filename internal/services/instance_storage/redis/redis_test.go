//+build integration

package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-redis/redis"
	"github.com/orlangure/gnomock"
	predis "github.com/orlangure/gnomock/preset/redis"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/entities"
)

var dbNumber int32 = 0
var redisContainer *gnomock.Container

func getRedisConnection(t *testing.T) *redis.Client {
	db := atomic.AddInt32(&dbNumber, 1)
	client := redis.NewClient(&redis.Options{
		Addr: redisContainer.DefaultAddress(),
		DB:   int(db),
	})
	t.Cleanup(func() {
		client.FlushDB()
	})
	return client
}

func TestMain(m *testing.M) {
	var err error
	redisContainer, err = gnomock.Start(predis.Preset())
	if err != nil {
		panic(fmt.Sprintf("error creating redis docker instance: %s\n", err))
	}
	code := m.Run()
	_ = gnomock.Stop(redisContainer)
	os.Exit(code)
}

func assertInstanceRedis(t *testing.T, client *redis.Client, expectedInstance *entities.GameRoomInstance) {
	actualInstance := new(entities.GameRoomInstance)
	instanceJson, err := client.HGet(getPodMapRedisKey("game"), "1").Result()
	require.NoError(t, err)
	require.NoError(t, json.NewDecoder(strings.NewReader(instanceJson)).Decode(actualInstance))
	require.Equal(t, expectedInstance, actualInstance)
}

func TestRedisInstanceStorage_AddInstance(t *testing.T) {
	client := getRedisConnection(t)
	storage := NewRedisInstanceStorage(client, 0)
	instance := &entities.GameRoomInstance{
		ID:          "1",
		SchedulerID: "game",
		Version:     "1",
		Status: entities.GameRoomInstanceStatus{
			Type: entities.GameRoomInstancePending,
		},
	}

	require.NoError(t, storage.AddInstance(context.Background(), instance))
	assertInstanceRedis(t, client, instance)

	instance.Status.Type = entities.GameRoomInstanceReady
	instance.Address = &entities.GameRoomAddress{
		Host: "host",
		Ports: []entities.GameRoomPort{
			{
				Name:     "game",
				Port:     7000,
				Protocol: "udp",
			},
		},
	}

	require.NoError(t, storage.AddInstance(context.Background(), instance))
	assertInstanceRedis(t, client, instance)
}

func TestRedisInstanceStorage_GetInstance(t *testing.T) {
	t.Run("when instance exists", func(t *testing.T) {
		storage := NewRedisInstanceStorage(getRedisConnection(t), 0)
		instance := &entities.GameRoomInstance{
			ID:          "1",
			SchedulerID: "game",
			Version:     "1",
			Status: entities.GameRoomInstanceStatus{
				Type: entities.GameRoomInstanceReady,
			},
			Address: &entities.GameRoomAddress{
				Host: "host",
				Ports: []entities.GameRoomPort{
					{
						Name:     "game",
						Port:     7000,
						Protocol: "udp",
					},
				},
			},
		}

		require.NoError(t, storage.AddInstance(context.Background(), instance))
		actualInstance, err := storage.GetInstance(context.Background(), "game", "1")
		require.NoError(t, err)
		require.Equal(t, instance, actualInstance)
	})

	t.Run("when instance does not exists", func(t *testing.T) {
		storage := NewRedisInstanceStorage(getRedisConnection(t), 0)
		_, err := storage.GetInstance(context.Background(), "game", "1")
		require.Error(t, err)
	})
}

func TestRedisInstanceStorage_RemoveInstance(t *testing.T) {
	t.Run("when instance exists", func(t *testing.T) {
		storage := NewRedisInstanceStorage(getRedisConnection(t), 0)
		instance := &entities.GameRoomInstance{
			ID:          "1",
			SchedulerID: "game",
			Version:     "1",
			Status: entities.GameRoomInstanceStatus{
				Type: entities.GameRoomInstanceReady,
			},
			Address: &entities.GameRoomAddress{
				Host: "host",
				Ports: []entities.GameRoomPort{
					{
						Name:     "game",
						Port:     7000,
						Protocol: "udp",
					},
				},
			},
		}

		require.NoError(t, storage.AddInstance(context.Background(), instance))
		require.NoError(t, storage.RemoveInstance(context.Background(), "game", "1"))
	})

	t.Run("when instance does not exists", func(t *testing.T) {
		storage := NewRedisInstanceStorage(getRedisConnection(t), 0)
		require.Error(t, storage.RemoveInstance(context.Background(), "game", "1"))
	})
}

func TestRedisInstanceStorage_GetAllInstances(t *testing.T) {
	storage := NewRedisInstanceStorage(getRedisConnection(t), 0)
	instances := []*entities.GameRoomInstance{
		{
			ID:          "1",
			SchedulerID: "game",
			Version:     "1",
			Status: entities.GameRoomInstanceStatus{
				Type: entities.GameRoomInstanceReady,
			},
			Address: &entities.GameRoomAddress{
				Host: "host",
				Ports: []entities.GameRoomPort{
					{
						Name:     "game",
						Port:     7000,
						Protocol: "udp",
					},
				},
			},
		},
		{
			ID:          "2",
			SchedulerID: "game",
			Version:     "1",
			Status: entities.GameRoomInstanceStatus{
				Type:        entities.GameRoomInstanceError,
				Description: "error",
			},
			Address: &entities.GameRoomAddress{
				Host: "host",
				Ports: []entities.GameRoomPort{
					{
						Name:     "game",
						Port:     7000,
						Protocol: "udp",
					},
				},
			},
		},
	}

	for _, instance := range instances {
		require.NoError(t, storage.AddInstance(context.Background(), instance))
	}
	actualInstances, err := storage.GetAllInstances(context.Background(), "game")
	require.NoError(t, err)
	require.ElementsMatch(t, instances, actualInstances)
}

func TestRedisInstanceStorage_GetInstanceCount(t *testing.T) {
	storage := NewRedisInstanceStorage(getRedisConnection(t), 0)
	instances := []*entities.GameRoomInstance{
		{
			ID:          "1",
			SchedulerID: "game",
			Version:     "1",
			Status: entities.GameRoomInstanceStatus{
				Type: entities.GameRoomInstanceReady,
			},
			Address: &entities.GameRoomAddress{
				Host: "host",
				Ports: []entities.GameRoomPort{
					{
						Name:     "game",
						Port:     7000,
						Protocol: "udp",
					},
				},
			},
		},
		{
			ID:          "2",
			SchedulerID: "game",
			Version:     "1",
			Status: entities.GameRoomInstanceStatus{
				Type:        entities.GameRoomInstanceError,
				Description: "error",
			},
			Address: &entities.GameRoomAddress{
				Host: "host",
				Ports: []entities.GameRoomPort{
					{
						Name:     "game",
						Port:     7000,
						Protocol: "udp",
					},
				},
			},
		},
	}

	for _, instance := range instances {
		require.NoError(t, storage.AddInstance(context.Background(), instance))
	}
	count, err := storage.GetInstanceCount(context.Background(), "game")
	require.NoError(t, err)
	require.Equal(t, 2, count)
}
