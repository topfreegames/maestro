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
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/orlangure/gnomock"
	predis "github.com/orlangure/gnomock/preset/redis"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/ports"
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

func TestEnqueueOperationCancelationRequest(t *testing.T) {
	schedulerName := uuid.New().String()
	operationID := uuid.New().String()

	t.Run("successfully publishes the request to cancel", func(t *testing.T) {
		client := getRedisConnection(t)
		flow := NewRedisOperationFlow(client)
		ctx := context.Background()

		cancelChan := flow.WatchOperationCancelationRequests(ctx)
		err := flow.EnqueueOperationCancelationRequest(ctx, ports.OperationCancelationRequest{
			SchedulerName: schedulerName,
			OperationID:   operationID,
		})

		require.NoError(t, err)

		require.Eventually(t, func() bool {
			select {
			case request := <-cancelChan:
				require.Equal(t, request.SchedulerName, schedulerName)
				require.Equal(t, request.OperationID, operationID)
				return true
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)
	})
}

func TestWatchOperationCancelationRequests(t *testing.T) {
	schedulerName := uuid.New().String()
	operationID := uuid.New().String()

	t.Run("successfully receives the scheduler name and operation ID to cancel", func(t *testing.T) {
		client := getRedisConnection(t)
		flow := NewRedisOperationFlow(client)
		ctx, ctxCancelFn := context.WithCancel(context.Background())

		cancelChan := flow.WatchOperationCancelationRequests(ctx)

		requestAsString, _ := json.Marshal(ports.OperationCancelationRequest{
			SchedulerName: schedulerName,
			OperationID:   operationID,
		})

		err := client.Publish(ctx, watchOperationCancelationRequestKey, string(requestAsString)).Err()
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			select {
			case request := <-cancelChan:
				require.Equal(t, request.SchedulerName, schedulerName)
				require.Equal(t, request.OperationID, operationID)
				return true
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)

		ctxCancelFn()
	})

	t.Run("when parent context is canceled, stops to watch requests", func(t *testing.T) {
		client := getRedisConnection(t)
		flow := NewRedisOperationFlow(client)
		ctx, ctxCancelFn := context.WithCancel(context.Background())

		cancelChan := flow.WatchOperationCancelationRequests(ctx)
		ctxCancelFn()

		require.Eventually(t, func() bool {
			select {
			case _, ok := <-cancelChan:
				return !ok
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("when redis connection fails, stops to watch requests", func(t *testing.T) {
		client := getRedisConnection(t)
		flow := NewRedisOperationFlow(client)
		ctx := context.Background()

		cancelChan := flow.WatchOperationCancelationRequests(ctx)

		client.Close()

		require.Eventually(t, func() bool {
			select {
			case _, ok := <-cancelChan:
				return !ok
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)
	})
}
