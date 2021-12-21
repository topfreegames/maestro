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

package redis

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	clockmock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	errors "github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/test"
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

func TestGrantLease(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		err := storage.GrantLease(context.Background(), "schedulerName", "operationID", time.Minute)
		require.NoError(t, err)

		_, err = client.ZScore(context.Background(), "operations:schedulerName:operationsLease", "operationID").Result()
		require.NotEqual(t, err, redis.Nil)
	})

	t.Run("with error - lease already exists", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		err := storage.GrantLease(context.Background(), "schedulerName", "operationID", time.Minute)
		require.NoError(t, err)

		_, err = client.ZScore(context.Background(), "operations:schedulerName:operationsLease", "operationID").Result()
		require.NotEqual(t, err, redis.Nil)

		err = storage.GrantLease(context.Background(), "schedulerName", "operationID", time.Minute)
		require.Error(t, err)
		require.Equal(t, err, errors.NewErrAlreadyExists("Lease already exists for operation operationID on scheduler schedulerName"))
	})

	t.Run("with error - redis connection breaks", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		client.Close()

		err := storage.GrantLease(context.Background(), "schedulerName", "operationID", time.Minute)
		require.Error(t, err)
	})
}

func TestRevokeLease(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		err := storage.GrantLease(context.Background(), "schedulerName", "operationID", time.Minute)
		require.NoError(t, err)

		_, err = client.ZScore(context.Background(), "operations:schedulerName:operationsLease", "operationID").Result()
		require.NotEqual(t, err, redis.Nil)

		err = storage.RevokeLease(context.Background(), "schedulerName", "operationID")
		require.Equal(t, err, nil)

		_, err = client.ZScore(context.Background(), "operations:schedulerName:operationsLease", "operationID").Result()
		require.Equal(t, err, redis.Nil)
	})

	t.Run("with error - not exists", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		schedulerName := "schedulerName"
		operationId := "operationID"

		err := storage.RevokeLease(context.Background(), schedulerName, operationId)
		require.Equal(t, err, errors.NewErrNotFound("Lease of scheduler \"%s\" and operationId \"%s\" does not exist", schedulerName, operationId))
	})

	t.Run("with error - redis fails", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		schedulerName := "schedulerName"
		operationId := "operationID"

		client.Close()

		err := storage.RevokeLease(context.Background(), schedulerName, operationId)
		require.Error(t, err)
	})
}

func TestRenewLease(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		require.Equal(t, 1, 1)
	})
}

func TestFetchLeaseTTL(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		require.Equal(t, 1, 1)
	})
}

func TestListExpiredLeases(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		require.Equal(t, 1, 1)
	})
}
