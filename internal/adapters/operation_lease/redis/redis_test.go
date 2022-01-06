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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/operation"

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
	schedulerName := "schedulerName"
	operationId := "operationID"

	t.Run("with success", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		err := storage.GrantLease(context.Background(), schedulerName, operationId, time.Minute)
		require.NoError(t, err)

		score_initial, err := client.ZScore(context.Background(), buildOperationLeaseKey(schedulerName), operationId).Result()
		require.NotEqual(t, err, redis.Nil)

		err = storage.RenewLease(context.Background(), schedulerName, operationId, time.Minute)
		require.Equal(t, err, nil)

		score_after_renew, err := client.ZScore(context.Background(), buildOperationLeaseKey(schedulerName), operationId).Result()
		require.NoError(t, err)
		require.Greater(t, score_after_renew, score_initial)
	})

	t.Run("with error - not exists", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		err := storage.RenewLease(context.Background(), schedulerName, operationId, time.Minute)
		require.Equal(t, err, errors.NewErrNotFound("Lease of scheduler \"%s\" and operationId \"%s\" does not exist", schedulerName, operationId))
	})

	t.Run("with error - redis fails", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		client.Close()

		err := storage.RenewLease(context.Background(), schedulerName, operationId, time.Minute)
		require.Error(t, err)
	})
}

func TestFetchLeaseOperationLeases(t *testing.T) {
	t.Run("when all the operations lease exists it returns its ttls", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)
		expectedOperationLeases := []*operation.OperationLease{
			{
				OperationID: "operationID1",
				Ttl:         time.Unix(clock.Now().Add(time.Minute).Unix(), 0),
			},
			{
				OperationID: "operationID2",
				Ttl:         time.Unix(clock.Now().Add(2*time.Minute).Unix(), 0),
			},
			{
				OperationID: "operationID3",
				Ttl:         time.Unix(clock.Now().Add(3*time.Minute).Unix(), 0),
			},
		}

		err := storage.GrantLease(context.Background(), "schedulerName", "operationID1", time.Minute)
		require.NoError(t, err)
		err = storage.GrantLease(context.Background(), "schedulerName", "operationID2", 2*time.Minute)
		require.NoError(t, err)
		err = storage.GrantLease(context.Background(), "schedulerName", "operationID3", 3*time.Minute)
		require.NoError(t, err)

		leases, err := storage.FetchOperationsLease(context.Background(), "schedulerName", "operationID1", "operationID2", "operationID3")

		require.NoError(t, err)
		require.Equal(t, expectedOperationLeases, leases)
	})

	t.Run("when not all the operations lease exists it returns error", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		err := storage.GrantLease(context.Background(), "schedulerName", "operationID1", time.Minute)
		require.NoError(t, err)
		err = storage.GrantLease(context.Background(), "schedulerName", "operationID3", 3*time.Minute)
		require.NoError(t, err)

		_, err = storage.FetchOperationsLease(context.Background(), "schedulerName", "operationID1", "operationID2", "operationID3")

		require.Error(t, err, "failed on fetching ttl for operation schedulerName in scheduler operationID2")
	})

	t.Run("when the provided operation ids args are empty then it returns an empty list", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		err := storage.GrantLease(context.Background(), "schedulerName", "operationID1", time.Minute)
		require.NoError(t, err)
		err = storage.GrantLease(context.Background(), "schedulerName", "operationID3", 3*time.Minute)
		require.NoError(t, err)

		leases, err := storage.FetchOperationsLease(context.Background(), "schedulerName", []string{}...)

		require.NoError(t, err)
		require.Empty(t, leases)
	})

	t.Run("when no operation lease exists it returns an error", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		_, err := storage.FetchOperationsLease(context.Background(), "schedulerName", "operationID1", "operationID2", "operationID3")

		require.Error(t, err, "failed on fetching ttl for operation schedulerName in scheduler operationID1")
	})
}

func TestFetchLeaseTTL(t *testing.T) {
	t.Run("when the lease exists it returns its ttl", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		err := storage.GrantLease(context.Background(), "schedulerName", "operationID", time.Minute)
		require.NoError(t, err)

		ttl, err := storage.FetchLeaseTTL(context.Background(), "schedulerName", "operationID")
		require.NoError(t, err)
		require.Equal(t, clock.Now().Add(time.Minute).Unix(), ttl.Unix())
	})
	t.Run("when the lease doesn't exists it returns an error", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		_, err := storage.FetchLeaseTTL(context.Background(), "schedulerName", "operationID")
		require.Error(t, err, errors.NewErrNotFound("Lease of scheduler schedulerName and operationId operationID does not exist"))
	})
}

func TestListExpiredLeases(t *testing.T) {
	t.Run("when some lease's are expired it returns a list", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		err := storage.GrantLease(context.Background(), "schedulerName", "opId1", 5*time.Minute)
		require.NoError(t, err)
		err = storage.GrantLease(context.Background(), "schedulerName", "opId2", 6*time.Minute)
		require.NoError(t, err)
		err = storage.GrantLease(context.Background(), "schedulerName", "opId3", 4*time.Minute)
		require.NoError(t, err)

		ops, err := storage.ListExpiredLeases(context.Background(), "schedulerName", clock.Now().Add(5*time.Minute))

		require.NoError(t, err)
		require.Equal(t, 2, len(ops))
		require.Equal(t, "opId3", ops[0].OperationID)
		require.Equal(t, clock.Now().Add(4*time.Minute).Unix(), ops[0].Ttl.Unix())
		require.Equal(t, "opId1", ops[1].OperationID)
		require.Equal(t, clock.Now().Add(5*time.Minute).Unix(), ops[1].Ttl.Unix())
	})

	t.Run("when no lease is expired it returns an empty list", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)

		err := storage.GrantLease(context.Background(), "schedulerName", "opId1", 5*time.Minute)
		require.NoError(t, err)
		err = storage.GrantLease(context.Background(), "schedulerName", "opId2", 4*time.Minute)
		require.NoError(t, err)
		err = storage.GrantLease(context.Background(), "schedulerName", "opId3", 6*time.Minute)
		require.NoError(t, err)

		ops, err := storage.ListExpiredLeases(context.Background(), "schedulerName", clock.Now())
		require.NoError(t, err)
		require.Equal(t, 0, len(ops))
	})

	t.Run("when no lease exists it returns an empty list", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationLeaseStorage(client, clock)
		ops, err := storage.ListExpiredLeases(context.Background(), "schedulerName", clock.Now())
		require.NoError(t, err)
		require.Equal(t, 0, len(ops))
	})
}

func buildOperationLeaseKey(schedulerName string) string {
	return fmt.Sprintf("operations:%s:operationsLease", schedulerName)
}
