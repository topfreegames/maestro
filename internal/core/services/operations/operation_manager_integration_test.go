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

package operations

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	timeClock "github.com/topfreegames/maestro/internal/adapters/clock/time"
	leaseredis "github.com/topfreegames/maestro/internal/adapters/lease/redis/operation"
	operationredis "github.com/topfreegames/maestro/internal/adapters/storage/redis/operation"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/operations/schedulers/switchversion"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
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

func TestCancelOperationWithoutCancelFunctionOnRedis(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redisAddress})
	t.Cleanup(func() { _ = client.Close() })

	clock := timeClock.NewClock()
	definitionConstructors := map[string]operations.DefinitionConstructor{
		switchversion.OperationName: func() operations.Definition { return &switchversion.Definition{} },
	}
	operationStorage := operationredis.NewRedisOperationStorage(client, clock, map[operationredis.Definition]time.Duration{}, definitionConstructors)
	leaseStorage := leaseredis.NewRedisOperationLeaseStorage(client, clock)
	opManager := New(nil, operationStorage, definitionConstructors, leaseStorage, OperationManagerConfig{OperationLeaseTtl: 5 * time.Second}, nil)

	createInProgressOperation := func(t *testing.T, schedulerName string, leaseTtl time.Duration) *operation.Operation {
		definition := &switchversion.Definition{NewActiveVersion: "v2.0.0"}
		op := operation.New(schedulerName, definition.Name(), definition.Marshal())
		require.NoError(t, operationStorage.CreateOperation(ctx, op))
		require.NoError(t, leaseStorage.GrantLease(ctx, schedulerName, op.ID, leaseTtl))
		require.NoError(t, operationStorage.UpdateOperationStatus(ctx, schedulerName, op.ID, operation.StatusInProgress))

		activeOperations, err := operationStorage.ListSchedulerActiveOperations(ctx, schedulerName)
		require.NoError(t, err)
		require.Len(t, activeOperations, 1)
		return op
	}

	t.Run("cancels an in-progress operation left behind by a worker that is gone", func(t *testing.T) {
		schedulerName := uuid.NewString()
		op := createInProgressOperation(t, schedulerName, -2*abandonedOperationLeaseMargin)

		abandoned, err := CountAbandonedOperations(ctx, operationStorage, leaseStorage, schedulerName)
		require.NoError(t, err)
		require.Equal(t, 1, abandoned)

		require.NoError(t, opManager.cancelOperation(ctx, schedulerName, op.ID))

		abandoned, err = CountAbandonedOperations(ctx, operationStorage, leaseStorage, schedulerName)
		require.NoError(t, err)
		require.Equal(t, 0, abandoned)

		storedOperation, err := operationStorage.GetOperation(ctx, schedulerName, op.ID)
		require.NoError(t, err)
		require.Equal(t, operation.StatusCanceled, storedOperation.Status)

		activeOperations, err := operationStorage.ListSchedulerActiveOperations(ctx, schedulerName)
		require.NoError(t, err)
		require.Empty(t, activeOperations)

		_, err = leaseStorage.FetchLeaseTTL(ctx, schedulerName, op.ID)
		require.True(t, errors.Is(err, porterrors.ErrNotFound))
	})

	t.Run("keeps an in-progress operation whose lease is still being renewed", func(t *testing.T) {
		schedulerName := uuid.NewString()
		op := createInProgressOperation(t, schedulerName, 5*time.Second)

		abandoned, err := CountAbandonedOperations(ctx, operationStorage, leaseStorage, schedulerName)
		require.NoError(t, err)
		require.Equal(t, 0, abandoned)

		require.Error(t, opManager.cancelOperation(ctx, schedulerName, op.ID))

		storedOperation, err := operationStorage.GetOperation(ctx, schedulerName, op.ID)
		require.NoError(t, err)
		require.Equal(t, operation.StatusInProgress, storedOperation.Status)

		activeOperations, err := operationStorage.ListSchedulerActiveOperations(ctx, schedulerName)
		require.NoError(t, err)
		require.Len(t, activeOperations, 1)
	})
}
