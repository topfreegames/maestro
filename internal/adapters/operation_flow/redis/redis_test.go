//+build integration

package redis

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/orlangure/gnomock"
	predis "github.com/orlangure/gnomock/preset/redis"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
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
		client.FlushDB(context.Background())
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

func TestInsertOperationID(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		client := getRedisConnection(t)
		flow := NewRedisOperationFlow(client)
		schedulerName := "test-scheduler"
		expectedOperationID := "some-op-id"

		err := flow.InsertOperationID(context.Background(), schedulerName, expectedOperationID)
		require.NoError(t, err)

		opID, err := client.LPop(context.Background(), flow.buildSchedulerPendingOperationsKey(schedulerName)).Result()
		require.NoError(t, err)
		require.Equal(t, expectedOperationID, opID)
	})

	t.Run("fails on redis", func(t *testing.T) {
		client := getRedisConnection(t)
		flow := NewRedisOperationFlow(client)

		// "drop" redis connection
		client.Close()

		err := flow.InsertOperationID(context.Background(), "", "")
		require.ErrorIs(t, errors.ErrUnexpected, err)
	})
}

func TestNextOperationID(t *testing.T) {
	t.Run("successfully receives the operation ID", func(t *testing.T) {
		client := getRedisConnection(t)
		flow := NewRedisOperationFlow(client)

		schedulerName := "test-scheduler"
		expectedOperationID := "some-op-id"

		err := flow.InsertOperationID(context.Background(), schedulerName, expectedOperationID)
		require.NoError(t, err)

		opID, err := flow.NextOperationID(context.Background(), schedulerName)
		require.NoError(t, err)
		require.Equal(t, expectedOperationID, opID)
	})

	t.Run("failed with context canceled", func(t *testing.T) {
		client := getRedisConnection(t)
		flow := NewRedisOperationFlow(client)

		ctx, cancel := context.WithCancel(context.Background())
		nextWait := make(chan error)

		go func() {
			_, err := flow.NextOperationID(ctx, "some-random-scheduler")
			nextWait <- err
		}()

		cancel()

		require.Eventually(t, func() bool {
			select {
			case err := <-nextWait:
				require.Error(t, err)
				return true
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("failed redis connection", func(t *testing.T) {
		client := getRedisConnection(t)
		storage := NewRedisOperationFlow(client)

		nextWait := make(chan error)
		go func() {
			_, err := storage.NextOperationID(context.Background(), "some-random-scheduler")
			nextWait <- err
		}()

		client.Close()

		require.Eventually(t, func() bool {
			select {
			case err := <-nextWait:
				require.Error(t, err)
				return true
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)
	})
}
