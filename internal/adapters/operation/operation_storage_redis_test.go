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

package operation

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	clockmock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/test"
)

func TestCreateOperation(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)
		createdAtString := "2020-01-01T00:00:00.001Z"
		createdAt, _ := time.Parse(time.RFC3339Nano, createdAtString)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: "test-definition",
			CreatedAt:      createdAt,
		}

		contents := []byte("hello test")
		err := storage.CreateOperation(context.Background(), op, contents)
		require.NoError(t, err)

		operationStored, err := client.HGetAll(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID)).Result()
		require.NoError(t, err)
		require.Equal(t, op.ID, operationStored[idRedisKey])
		require.Equal(t, op.SchedulerName, operationStored[schedulerNameRedisKey])
		require.Equal(t, op.DefinitionName, operationStored[definitionNameRedisKey])
		require.Equal(t, createdAtString, operationStored[createdAtRedisKey])
		require.Equal(t, string(contents), operationStored[definitionContentsRedisKey])

		intStatus, err := strconv.Atoi(operationStored[statusRedisKey])
		require.NoError(t, err)
		require.Equal(t, op.Status, operation.Status(intStatus))
	})

	t.Run("fails on redis", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: "test-definition",
			CreatedAt:      time.Now(),
		}

		// "drop" redis connection
		client.Close()

		err := storage.CreateOperation(context.Background(), op, []byte{})
		require.ErrorIs(t, errors.ErrUnexpected, err)
	})
}

func TestGetOperation(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)

		createdAtString := "2020-01-01T00:00:00.001Z"
		createdAt, _ := time.Parse(time.RFC3339Nano, createdAtString)
		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: "test-definition",
			CreatedAt:      createdAt,
		}

		contents := []byte("hello test")
		err := storage.CreateOperation(context.Background(), op, contents)
		require.NoError(t, err)

		operationStored, definitionContents, err := storage.GetOperation(context.Background(), op.SchedulerName, op.ID)
		require.NoError(t, err)
		require.Equal(t, contents, definitionContents)
		require.Equal(t, op.ID, operationStored.ID)
		require.Equal(t, op.SchedulerName, operationStored.SchedulerName)
		require.Equal(t, op.Status, operationStored.Status)
		require.Equal(t, op.DefinitionName, operationStored.DefinitionName)
		require.Equal(t, createdAt, operationStored.CreatedAt)
	})

	t.Run("not found", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)

		operationStored, definitionContents, err := storage.GetOperation(context.Background(), "test-scheduler", "inexistent-id")
		require.ErrorIs(t, errors.ErrNotFound, err)
		require.Nil(t, operationStored)
		require.Nil(t, definitionContents)
	})

	t.Run("fail to parse created at field", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: "test-definition",
			CreatedAt:      time.Now(),
		}

		contents := []byte("hello test")
		err := storage.CreateOperation(context.Background(), op, contents)
		require.NoError(t, err)

		client.HSet(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
			createdAtRedisKey: "INVALID DATE",
		})

		operationStored, definitionContents, err := storage.GetOperation(context.Background(), "test-scheduler", "some-op-id")
		require.Contains(t, err.Error(), "failed to parse operation createdAt field")
		require.Nil(t, operationStored)
		require.Nil(t, definitionContents)
	})

	t.Run("fails on redis", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)

		// "drop" redis connection
		client.Close()

		operationStored, definitionContents, err := storage.GetOperation(context.Background(), "test-scheduler", "some-op-id")
		require.ErrorIs(t, errors.ErrUnexpected, err)
		require.Nil(t, operationStored)
		require.Nil(t, definitionContents)
	})
}

func TestUpdateOperationStatus(t *testing.T) {
	t.Run("set operation as active", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		now := time.Now()
		clock := clockmock.NewFakeClock(now)
		storage := NewRedisOperationStorage(client, clock)

		op := &operation.Operation{
			ID:            "some-op-id",
			SchedulerName: "test-scheduler",
			Status:        operation.StatusPending,
		}

		err := storage.CreateOperation(context.Background(), op, []byte{})
		require.NoError(t, err)

		err = storage.UpdateOperationStatus(context.Background(), op.SchedulerName, op.ID, operation.StatusInProgress)
		require.NoError(t, err)

		operationStored, err := client.HGetAll(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID)).Result()
		require.NoError(t, err)

		intStatus, err := strconv.Atoi(operationStored[statusRedisKey])
		require.NoError(t, err)
		require.Equal(t, operation.StatusInProgress, operation.Status(intStatus))

		score, err := client.ZScore(context.Background(), storage.buildSchedulerActiveOperationsKey(op.SchedulerName), op.ID).Result()
		require.NoError(t, err)
		require.Equal(t, float64(now.Unix()), score)

		err = client.ZScore(context.Background(), storage.buildSchedulerHistoryOperationsKey(op.SchedulerName), op.ID).Err()
		require.ErrorIs(t, redis.Nil, err)
	})

	t.Run("update operation status to inactive", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		now := time.Now()
		clock := clockmock.NewFakeClock(now)
		storage := NewRedisOperationStorage(client, clock)

		op := &operation.Operation{
			ID:            "some-op-id",
			SchedulerName: "test-scheduler",
			Status:        operation.StatusPending,
		}

		err := storage.CreateOperation(context.Background(), op, []byte{})
		require.NoError(t, err)

		err = storage.UpdateOperationStatus(context.Background(), op.SchedulerName, op.ID, operation.StatusInProgress)
		require.NoError(t, err)

		err = storage.UpdateOperationStatus(context.Background(), op.SchedulerName, op.ID, operation.StatusError)
		require.NoError(t, err)

		operationStored, err := client.HGetAll(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID)).Result()
		require.NoError(t, err)

		intStatus, err := strconv.Atoi(operationStored[statusRedisKey])
		require.NoError(t, err)
		require.Equal(t, operation.StatusError, operation.Status(intStatus))

		score, err := client.ZScore(context.Background(), storage.buildSchedulerHistoryOperationsKey(op.SchedulerName), op.ID).Result()
		require.NoError(t, err)
		require.Equal(t, float64(now.Unix()), score)

		err = client.ZScore(context.Background(), storage.buildSchedulerActiveOperationsKey(op.SchedulerName), op.ID).Err()
		require.ErrorIs(t, redis.Nil, err)
	})
}

func TestUpdateOperationExecutionHistory(t *testing.T) {

	t.Run("set execution history with value", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		now := time.Now()
		clock := clockmock.NewFakeClock(now)
		storage := NewRedisOperationStorage(client, clock)

		op := &operation.Operation{
			ID:            "some-op-id",
			SchedulerName: "test-scheduler",
			Status:        operation.StatusPending,
		}

		err := storage.CreateOperation(context.Background(), op, []byte{})
		require.NoError(t, err)

		events := []operation.OperationEvent{
			operation.OperationEvent{CreatedAt: time.Now(), Event: "event1"},
			operation.OperationEvent{CreatedAt: time.Now(), Event: "event2"},
		}
		op.ExecutionHistory = events

		err = storage.UpdateOperationExecutionHistory(context.Background(), op)
		require.NoError(t, err)

		operationStored, err := client.HGetAll(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID)).Result()
		require.NoError(t, err)

		execHist := []operation.OperationEvent{}
		err = json.Unmarshal([]byte(operationStored[executionHistory]), &execHist)
		require.NoError(t, err)

		for i, _ := range op.ExecutionHistory {
			require.Equal(t, op.ExecutionHistory[i].Event, execHist[i].Event)
			require.Equal(t, op.ExecutionHistory[i].CreatedAt.Unix(), execHist[i].CreatedAt.Unix())
		}
	})

	t.Run("redis connection closed: returns error", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		now := time.Now()
		clock := clockmock.NewFakeClock(now)
		storage := NewRedisOperationStorage(client, clock)

		op := &operation.Operation{
			ID:            "some-op-id",
			SchedulerName: "test-scheduler",
			Status:        operation.StatusPending,
		}
		client.Close()

		err := storage.UpdateOperationExecutionHistory(context.Background(), op)
		require.Error(t, err)
	})

}

func TestListSchedulerActiveOperations(t *testing.T) {
	t.Run("list all operations", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		now := time.Now()
		storage := NewRedisOperationStorage(client, clockmock.NewFakeClock(now))

		schedulerName := "test-scheduler"
		activeOperations := []*operation.Operation{
			{ID: uuid.NewString(), SchedulerName: schedulerName, Status: operation.StatusInProgress},
			{ID: uuid.NewString(), SchedulerName: schedulerName, Status: operation.StatusInProgress},
			{ID: uuid.NewString(), SchedulerName: schedulerName, Status: operation.StatusInProgress},
		}

		pendingOperations := []*operation.Operation{
			{ID: uuid.NewString(), SchedulerName: schedulerName, Status: operation.StatusPending},
			{ID: uuid.NewString(), SchedulerName: schedulerName, Status: operation.StatusPending},
			{ID: uuid.NewString(), SchedulerName: schedulerName, Status: operation.StatusPending},
		}

		for _, op := range append(activeOperations, pendingOperations...) {
			err := storage.CreateOperation(context.Background(), op, []byte{})
			require.NoError(t, err)

			if op.Status == operation.StatusInProgress {
				err := storage.UpdateOperationStatus(context.Background(), op.SchedulerName, op.ID, op.Status)
				require.NoError(t, err)
			}
		}

		resultOperations, err := storage.ListSchedulerActiveOperations(context.Background(), schedulerName)
		require.NoError(t, err)
		require.Len(t, resultOperations, len(activeOperations))

	resultLoop:
		for _, activeOp := range activeOperations {
			for _, resultOp := range resultOperations {
				if resultOp.ID == activeOp.ID {
					continue resultLoop
				}
			}

			require.Fail(t, "%s operation not present on the result", activeOp.ID)
		}
	})

	t.Run("failed to fetch a operation inside the list", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		now := time.Now()
		storage := NewRedisOperationStorage(client, clockmock.NewFakeClock(now))

		schedulerName := "test-scheduler"
		activeOperations := []*operation.Operation{
			{ID: uuid.NewString(), SchedulerName: schedulerName, Status: operation.StatusInProgress},
			{ID: uuid.NewString(), SchedulerName: schedulerName, Status: operation.StatusInProgress},
		}

		for _, op := range activeOperations {
			err := storage.CreateOperation(context.Background(), op, []byte{})
			require.NoError(t, err)

			err = storage.UpdateOperationStatus(context.Background(), op.SchedulerName, op.ID, op.Status)
			require.NoError(t, err)
		}

		// manually add a faulty operation
		err := client.ZAdd(context.Background(), storage.buildSchedulerActiveOperationsKey(schedulerName), &redis.Z{
			Member: uuid.NewString(),
			Score:  float64(now.Unix()),
		}).Err()
		require.NoError(t, err)

		_, err = storage.ListSchedulerActiveOperations(context.Background(), schedulerName)
		require.Error(t, err)
		require.ErrorIs(t, errors.ErrNotFound, err)
	})
}
