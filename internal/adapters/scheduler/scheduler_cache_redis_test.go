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

//go:build integration
// +build integration

package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/test"
)

func TestSetScheduler(t *testing.T) {

	newScheduler := generateScheduler()

	t.Run("with success", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		storage := NewRedisSchedulerCache(client)

		err := storage.SetScheduler(context.Background(), newScheduler, time.Minute)
		require.NoError(t, err)

		schedulerJson, err := client.Get(context.Background(), storage.buildSchedulerKey(newScheduler.Name)).Result()
		require.NoError(t, err)
		require.NotEmpty(t, schedulerJson)
	})

	t.Run("with error - connection to redis closed", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		storage := NewRedisSchedulerCache(client)

		client.Close()

		err := storage.SetScheduler(context.Background(), newScheduler, time.Minute)
		require.Error(t, err)
	})
}

func TestGetScheduler(t *testing.T) {

	newScheduler := generateScheduler()

	t.Run("with success - Scheduler not found, but no error", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		storage := NewRedisSchedulerCache(client)

		scheduler, err := storage.GetScheduler(context.Background(), "schedulerName")
		require.NoError(t, err)
		require.Empty(t, scheduler)
	})

	t.Run("with success - Scheduler found", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		storage := NewRedisSchedulerCache(client)

		ctx := context.Background()
		err := storage.SetScheduler(ctx, newScheduler, time.Minute)
		require.NoError(t, err)

		scheduler, err := storage.GetScheduler(ctx, newScheduler.Name)
		require.NoError(t, err)
		require.NotEmpty(t, scheduler)
	})

	t.Run("with error - redis connection closed", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		storage := NewRedisSchedulerCache(client)

		ctx := context.Background()
		err := storage.SetScheduler(ctx, newScheduler, time.Minute)
		require.NoError(t, err)

		client.Close()

		_, err = storage.GetScheduler(ctx, newScheduler.Name)
		require.Error(t, err)
	})

}

func TestDeleteScheduler(t *testing.T) {

	newScheduler := generateScheduler()

	t.Run("with success - Scheduler not found, but no error", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		storage := NewRedisSchedulerCache(client)

		err := storage.DeleteScheduler(context.Background(), "schedulerName")
		require.NoError(t, err)
	})

	t.Run("with success - Scheduler found, no error", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		storage := NewRedisSchedulerCache(client)

		ctx := context.Background()

		err := storage.SetScheduler(ctx, newScheduler, time.Minute)
		require.NoError(t, err)

		err = storage.DeleteScheduler(ctx, newScheduler.Name)
		require.NoError(t, err)

		scheduler, err := storage.GetScheduler(ctx, newScheduler.Name)
		require.NoError(t, err)
		require.Nil(t, scheduler)
	})

	t.Run("with error - redis connection closed, returns error", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		storage := NewRedisSchedulerCache(client)

		client.Close()

		err := storage.DeleteScheduler(context.Background(), "schedulerName")
		require.Error(t, err)
	})

}

func generateScheduler() *entities.Scheduler {
	fwd := &forwarder.Forwarder{
		Name:        "fwd",
		Enabled:     true,
		ForwardType: forwarder.TypeGrpc,
		Address:     "address",
		Options: &forwarder.ForwardOptions{
			Timeout:  time.Second * 5,
			Metadata: nil,
		},
	}
	forwarders := []*forwarder.Forwarder{fwd}

	return &entities.Scheduler{
		Name:            "scheduler",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "10%",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Containers:             []game_room.Container{},
			Toleration:             "toleration",
			Affinity:               "affinity",
		},
		PortRange: &entities.PortRange{
			Start: 40000,
			End:   60000,
		},
		Forwarders: forwarders,
	}
}
