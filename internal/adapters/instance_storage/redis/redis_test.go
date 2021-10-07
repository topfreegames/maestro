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

//+build integration

package redis

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/test"

	"github.com/go-redis/redis/v8"

	"github.com/stretchr/testify/require"
)

var redisAddress string

func TestMain(m *testing.M) {
	var code int
	test.WithRedisContainer(func(redisContainerAddress string) {
		redisAddress = redisContainerAddress
		code = m.Run()
	})
	os.Exit(code)
}

func assertInstanceRedis(t *testing.T, client *redis.Client, expectedInstance *game_room.Instance) {
	actualInstance := new(game_room.Instance)
	instanceJson, err := client.HGet(context.Background(), getPodMapRedisKey("game"), "1").Result()
	require.NoError(t, err)
	require.NoError(t, json.NewDecoder(strings.NewReader(instanceJson)).Decode(actualInstance))
	require.Equal(t, expectedInstance, actualInstance)
}

func TestRedisInstanceStorage_UpsertInstance(t *testing.T) {
	client := test.GetRedisConnection(t, redisAddress)
	storage := NewRedisInstanceStorage(client, 0)
	instance := &game_room.Instance{
		ID:          "1",
		SchedulerID: "game",
		Status: game_room.InstanceStatus{
			Type: game_room.InstancePending,
		},
	}

	require.NoError(t, storage.UpsertInstance(context.Background(), instance))
	assertInstanceRedis(t, client, instance)

	instance.Status.Type = game_room.InstanceReady
	instance.Address = &game_room.Address{
		Host: "host",
		Ports: []game_room.Port{
			{
				Name:     "game",
				Port:     7000,
				Protocol: "udp",
			},
		},
	}

	require.NoError(t, storage.UpsertInstance(context.Background(), instance))
	assertInstanceRedis(t, client, instance)
}

func TestRedisInstanceStorage_GetInstance(t *testing.T) {
	t.Run("when instance exists", func(t *testing.T) {
		storage := NewRedisInstanceStorage(test.GetRedisConnection(t, redisAddress), 0)
		instance := &game_room.Instance{
			ID:          "1",
			SchedulerID: "game",
			Status: game_room.InstanceStatus{
				Type: game_room.InstanceReady,
			},
			Address: &game_room.Address{
				Host: "host",
				Ports: []game_room.Port{
					{
						Name:     "game",
						Port:     7000,
						Protocol: "udp",
					},
				},
			},
		}

		require.NoError(t, storage.UpsertInstance(context.Background(), instance))
		actualInstance, err := storage.GetInstance(context.Background(), "game", "1")
		require.NoError(t, err)
		require.Equal(t, instance, actualInstance)
	})

	t.Run("when instance does not exists", func(t *testing.T) {
		storage := NewRedisInstanceStorage(test.GetRedisConnection(t, redisAddress), 0)
		_, err := storage.GetInstance(context.Background(), "game", "1")
		require.Error(t, err)
	})
}

func TestRedisInstanceStorage_RemoveInstance(t *testing.T) {
	t.Run("when instance exists", func(t *testing.T) {
		storage := NewRedisInstanceStorage(test.GetRedisConnection(t, redisAddress), 0)
		instance := &game_room.Instance{
			ID:          "1",
			SchedulerID: "game",
			Status: game_room.InstanceStatus{
				Type: game_room.InstanceReady,
			},
			Address: &game_room.Address{
				Host: "host",
				Ports: []game_room.Port{
					{
						Name:     "game",
						Port:     7000,
						Protocol: "udp",
					},
				},
			},
		}

		require.NoError(t, storage.UpsertInstance(context.Background(), instance))
		require.NoError(t, storage.DeleteInstance(context.Background(), "game", "1"))
	})

	t.Run("when instance does not exists", func(t *testing.T) {
		storage := NewRedisInstanceStorage(test.GetRedisConnection(t, redisAddress), 0)
		require.Error(t, storage.DeleteInstance(context.Background(), "game", "1"))
	})
}

func TestRedisInstanceStorage_GetAllInstances(t *testing.T) {
	storage := NewRedisInstanceStorage(test.GetRedisConnection(t, redisAddress), 0)
	instances := []*game_room.Instance{
		{
			ID:          "1",
			SchedulerID: "game",
			Status: game_room.InstanceStatus{
				Type: game_room.InstanceReady,
			},
			Address: &game_room.Address{
				Host: "host",
				Ports: []game_room.Port{
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
			Status: game_room.InstanceStatus{
				Type:        game_room.InstanceError,
				Description: "error",
			},
			Address: &game_room.Address{
				Host: "host",
				Ports: []game_room.Port{
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
		require.NoError(t, storage.UpsertInstance(context.Background(), instance))
	}
	actualInstances, err := storage.GetAllInstances(context.Background(), "game")
	require.NoError(t, err)
	require.ElementsMatch(t, instances, actualInstances)
}

func TestRedisInstanceStorage_GetInstanceCount(t *testing.T) {
	storage := NewRedisInstanceStorage(test.GetRedisConnection(t, redisAddress), 0)
	instances := []*game_room.Instance{
		{
			ID:          "1",
			SchedulerID: "game",
			Status: game_room.InstanceStatus{
				Type: game_room.InstanceReady,
			},
			Address: &game_room.Address{
				Host: "host",
				Ports: []game_room.Port{
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
			Status: game_room.InstanceStatus{
				Type:        game_room.InstanceError,
				Description: "error",
			},
			Address: &game_room.Address{
				Host: "host",
				Ports: []game_room.Port{
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
		require.NoError(t, storage.UpsertInstance(context.Background(), instance))
	}
	count, err := storage.GetInstanceCount(context.Background(), "game")
	require.NoError(t, err)
	require.Equal(t, 2, count)
}
