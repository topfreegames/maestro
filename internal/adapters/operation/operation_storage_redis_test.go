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
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"

	"github.com/stretchr/testify/assert"

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
		operationsTTlMap := map[Definition]time.Duration{}
		storage := NewRedisOperationStorage(client, clock, operationsTTlMap)
		createdAtString := "2020-01-01T00:00:00.001Z"
		createdAt, _ := time.Parse(time.RFC3339Nano, createdAtString)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: "test-definition",
			CreatedAt:      createdAt,
			Input:          []byte("hello test"),
			ExecutionHistory: []operation.OperationEvent{
				{
					CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
					Event:     "some-event",
				},
			},
		}

		err := storage.CreateOperation(context.Background(), op)
		require.NoError(t, err)

		executionHistoryJson, err := json.Marshal(op.ExecutionHistory)
		require.NoError(t, err)

		operationStored, err := client.HGetAll(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID)).Result()
		require.NoError(t, err)
		require.Equal(t, op.ID, operationStored[idRedisKey])
		require.Equal(t, op.SchedulerName, operationStored[schedulerNameRedisKey])
		require.Equal(t, op.DefinitionName, operationStored[definitionNameRedisKey])
		require.Equal(t, createdAtString, operationStored[createdAtRedisKey])
		require.EqualValues(t, op.Input, operationStored[definitionContentsRedisKey])
		require.EqualValues(t, executionHistoryJson, operationStored[executionHistoryRedisKey])

		intStatus, err := strconv.Atoi(operationStored[statusRedisKey])
		require.NoError(t, err)
		require.Equal(t, op.Status, operation.Status(intStatus))
	})

	t.Run("with success when operation have ttl", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		operationsTTlMap := map[Definition]time.Duration{
			healthcontroller.OperationName: time.Second,
		}
		storage := NewRedisOperationStorage(client, clock, operationsTTlMap)
		createdAtString := "2020-01-01T00:00:00.001Z"
		createdAt, _ := time.Parse(time.RFC3339Nano, createdAtString)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: healthcontroller.OperationName,
			CreatedAt:      createdAt,
			Input:          []byte("hello test"),
			ExecutionHistory: []operation.OperationEvent{
				{
					CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
					Event:     "some-event",
				},
			},
		}

		err := storage.CreateOperation(context.Background(), op)
		require.NoError(t, err)

		time.Sleep(time.Second * 2)
		operationStored, _ := client.HGetAll(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID)).Result()
		require.True(t, len(operationStored) == 0)
	})

	t.Run("fails on redis", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		operationsTTlMap := map[Definition]time.Duration{}
		storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: "test-definition",
			CreatedAt:      time.Now(),
		}

		// "drop" redis connection
		client.Close()

		err := storage.CreateOperation(context.Background(), op)
		require.ErrorIs(t, errors.ErrUnexpected, err)
	})
}

func TestGetOperation(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		operationsTTlMap := map[Definition]time.Duration{}
		storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

		createdAtString := "2020-01-01T00:00:00.001Z"
		createdAt, _ := time.Parse(time.RFC3339Nano, createdAtString)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: "test-definition",
			CreatedAt:      createdAt,
			Input:          []byte("hello test"),
			ExecutionHistory: []operation.OperationEvent{
				{
					CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
					Event:     "some-event",
				},
			},
		}

		t.Run("when there is ExecutionHistory", func(t *testing.T) {
			executionHistoryJson, err := json.Marshal(op.ExecutionHistory)
			require.NoError(t, err)

			err = client.HSet(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
				idRedisKey:                 op.ID,
				schedulerNameRedisKey:      op.SchedulerName,
				statusRedisKey:             strconv.Itoa(int(op.Status)),
				definitionNameRedisKey:     op.DefinitionName,
				createdAtRedisKey:          op.CreatedAt.Format(time.RFC3339Nano),
				definitionContentsRedisKey: op.Input,
				executionHistoryRedisKey:   executionHistoryJson,
			}).Err()
			require.NoError(t, err)

			operationStored, err := storage.GetOperation(context.Background(), op.SchedulerName, op.ID)
			require.NoError(t, err)
			assert.Equal(t, op.Input, operationStored.Input)
			assert.Equal(t, op.ExecutionHistory, operationStored.ExecutionHistory)
			assert.Equal(t, op.ID, operationStored.ID)
			assert.Equal(t, op.SchedulerName, operationStored.SchedulerName)
			assert.Equal(t, op.Status, operationStored.Status)
			assert.Equal(t, op.DefinitionName, operationStored.DefinitionName)
			assert.Equal(t, createdAt, operationStored.CreatedAt)
		})

		t.Run("when there is no ExecutionHistory", func(t *testing.T) {
			op.ID = "some-other-id"
			err := client.HSet(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
				idRedisKey:                 op.ID,
				schedulerNameRedisKey:      op.SchedulerName,
				statusRedisKey:             strconv.Itoa(int(op.Status)),
				definitionNameRedisKey:     op.DefinitionName,
				createdAtRedisKey:          op.CreatedAt.Format(time.RFC3339Nano),
				definitionContentsRedisKey: op.Input,
			}).Err()
			require.NoError(t, err)

			operationStored, err := storage.GetOperation(context.Background(), op.SchedulerName, op.ID)
			require.NoError(t, err)
			assert.Equal(t, op.Input, operationStored.Input)
			assert.Empty(t, operationStored.ExecutionHistory)
			assert.Equal(t, op.ID, operationStored.ID)
			assert.Equal(t, op.SchedulerName, operationStored.SchedulerName)
			assert.Equal(t, op.Status, operationStored.Status)
			assert.Equal(t, op.DefinitionName, operationStored.DefinitionName)
			assert.Equal(t, createdAt, operationStored.CreatedAt)
		})
	})

	t.Run("not found", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		operationsTTlMap := map[Definition]time.Duration{}
		storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

		operationStored, err := storage.GetOperation(context.Background(), "test-scheduler", "inexistent-id")
		require.ErrorIs(t, errors.ErrNotFound, err)
		require.Nil(t, operationStored)
	})

	t.Run("fail to parse created at field", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		operationsTTlMap := map[Definition]time.Duration{}
		storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: "test-definition",
			CreatedAt:      time.Now(),
			Input:          []byte("hello test"),
		}

		err := storage.CreateOperation(context.Background(), op)
		require.NoError(t, err)

		client.HSet(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
			createdAtRedisKey: "INVALID DATE",
		})

		operationStored, err := storage.GetOperation(context.Background(), "test-scheduler", "some-op-id")
		require.Contains(t, err.Error(), "failed to parse operation createdAt field")
		require.Nil(t, operationStored)
	})

	t.Run("fails on redis", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		operationsTTlMap := map[Definition]time.Duration{}
		storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

		// "drop" redis connection
		client.Close()

		operationStored, err := storage.GetOperation(context.Background(), "test-scheduler", "some-op-id")
		require.ErrorIs(t, errors.ErrUnexpected, err)
		require.Nil(t, operationStored)
	})
}

func TestListSchedulerFinishedOperations(t *testing.T) {
	schedulerName := "test-scheduler"

	t.Run("with success", func(t *testing.T) {
		nowTime := time.Now()
		now, _ := time.Parse(time.RFC3339Nano, nowTime.Format(time.RFC3339Nano))

		operations := []*operation.Operation{
			{
				ID:             "some-op-id-1",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      now,
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
			{
				ID:             "some-op-id-2",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      now.Add(-23 * time.Hour),
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
			{
				ID:             "some-op-id-3",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      now.Add(-25 * time.Hour),
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
			{
				ID:             "some-op-id-4",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      now.Add(-29 * time.Hour),
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
		}

		t.Run("return operations list from the last day when there is more than one operation stored", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTlMap := map[Definition]time.Duration{}
			storage := NewRedisOperationStorage(client, clock, operationsTTlMap)
			expectedOperations := []*operation.Operation{operations[1], operations[0]}

			for _, op := range operations {
				executionHistoryJson, err := json.Marshal(op.ExecutionHistory)
				require.NoError(t, err)

				err = client.ZAdd(context.Background(), storage.buildSchedulerHistoryOperationsKey(op.SchedulerName), &redis.Z{
					Member: op.ID,
					Score:  float64(op.CreatedAt.Unix()),
				}).Err()
				require.NoError(t, err)

				err = client.HSet(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
					idRedisKey:                 op.ID,
					schedulerNameRedisKey:      op.SchedulerName,
					statusRedisKey:             strconv.Itoa(int(op.Status)),
					definitionNameRedisKey:     op.DefinitionName,
					createdAtRedisKey:          op.CreatedAt.Format(time.RFC3339Nano),
					definitionContentsRedisKey: op.Input,
					executionHistoryRedisKey:   executionHistoryJson,
				}).Err()
				require.NoError(t, err)
			}

			operationsReturned, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName)
			assert.NoError(t, err)
			assert.NotEmptyf(t, operationsReturned, "expected at least one operation")
			assert.Equal(t, expectedOperations, operationsReturned)
		})

		t.Run("return empty list when there is no operation stored", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTlMap := map[Definition]time.Duration{}
			storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

			operationsReturned, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName)
			assert.NoError(t, err)
			assert.Empty(t, operationsReturned, "expected result to be empty")
		})

		t.Run("return no error when operations are in the history but not stored, remove nonexistent operations from history asynchronously", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTlMap := map[Definition]time.Duration{}
			storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

			for _, op := range operations {
				err := client.ZAdd(context.Background(), storage.buildSchedulerHistoryOperationsKey(op.SchedulerName), &redis.Z{
					Member: op.ID,
					Score:  float64(clock.Now().Unix()),
				}).Err()
				require.NoError(t, err)
			}

			operationsReturned, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName)
			assert.NoError(t, err)
			assert.Empty(t, operationsReturned)
			assert.Eventually(t, func() bool {
				ids, _ := client.ZRange(context.Background(), storage.buildSchedulerHistoryOperationsKey(schedulerName), 0, -1).Result()
				return len(ids) == 0
			}, time.Second, 10*time.Millisecond, "expected non existent operations to not be in the history anymore")
		})

		t.Run("return no error when some operation is in the history but not stored", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTlMap := map[Definition]time.Duration{}
			storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

			for _, op := range operations {
				err := client.ZAdd(context.Background(), storage.buildSchedulerHistoryOperationsKey(op.SchedulerName), &redis.Z{
					Member: op.ID,
					Score:  float64(clock.Now().Unix()),
				}).Err()
				require.NoError(t, err)
			}

			firstOp := operations[0]
			executionHistoryJson, err := json.Marshal(firstOp.ExecutionHistory)
			require.NoError(t, err)

			err = client.HSet(context.Background(), storage.buildSchedulerOperationKey(firstOp.SchedulerName, firstOp.ID), map[string]interface{}{
				idRedisKey:                 firstOp.ID,
				schedulerNameRedisKey:      firstOp.SchedulerName,
				statusRedisKey:             strconv.Itoa(int(firstOp.Status)),
				definitionNameRedisKey:     firstOp.DefinitionName,
				createdAtRedisKey:          firstOp.CreatedAt.Format(time.RFC3339Nano),
				definitionContentsRedisKey: firstOp.Input,
				executionHistoryRedisKey:   executionHistoryJson,
			}).Err()
			require.NoError(t, err)

			operationsReturned, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName)
			assert.NoError(t, err)
			assert.Equal(t, operationsReturned[0], firstOp)
		})

	})

	t.Run("with error", func(t *testing.T) {
		operations := []*operation.Operation{
			{
				ID:             "some-op-id-1",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      time.Now(),
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
			{
				ID:             "some-op-id-2",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      time.Now(),
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
		}

		t.Run("return error when some error occurs parsing any operation hash", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTlMap := map[Definition]time.Duration{}
			storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

			for _, op := range operations {
				err := client.ZAdd(context.Background(), storage.buildSchedulerHistoryOperationsKey(op.SchedulerName), &redis.Z{
					Member: op.ID,
					Score:  float64(clock.Now().Unix()),
				}).Err()
				require.NoError(t, err)

				err = client.HSet(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
					idRedisKey: op.ID,
				}).Err()
				require.NoError(t, err)
			}

			_, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName)
			assert.EqualError(t, err, "failed to build operation from the hash: failed to parse operation status: strconv.Atoi: parsing \"\": invalid syntax")
		})

		t.Run("return error when there is some error in redis call", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTlMap := map[Definition]time.Duration{}
			storage := NewRedisOperationStorage(client, clock, operationsTTlMap)
			client.Close()

			_, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName)
			assert.EqualError(t, err, "failed to list finished operations for \"test-scheduler\": redis: client is closed")
		})
	})
}

func TestUpdateOperationStatus(t *testing.T) {
	t.Run("set operation as active", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		now := time.Now()
		clock := clockmock.NewFakeClock(now)
		operationsTTlMap := map[Definition]time.Duration{}
		storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

		op := &operation.Operation{
			ID:            "some-op-id",
			SchedulerName: "test-scheduler",
			Status:        operation.StatusPending,
		}

		err := storage.CreateOperation(context.Background(), op)
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
		operationsTTlMap := map[Definition]time.Duration{}
		storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

		op := &operation.Operation{
			ID:            "some-op-id",
			SchedulerName: "test-scheduler",
			Status:        operation.StatusPending,
		}

		err := storage.CreateOperation(context.Background(), op)
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
		operationsTTlMap := map[Definition]time.Duration{}
		storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

		op := &operation.Operation{
			ID:            "some-op-id",
			SchedulerName: "test-scheduler",
			Status:        operation.StatusPending,
		}

		err := storage.CreateOperation(context.Background(), op)
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
		err = json.Unmarshal([]byte(operationStored[executionHistoryRedisKey]), &execHist)
		require.NoError(t, err)

		for i := range op.ExecutionHistory {
			assert.Equal(t, op.ExecutionHistory[i].Event, execHist[i].Event)
			assert.Equal(t, op.ExecutionHistory[i].CreatedAt.Unix(), execHist[i].CreatedAt.Unix())
		}
	})

	t.Run("redis connection closed: returns error", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		now := time.Now()
		clock := clockmock.NewFakeClock(now)
		operationsTTlMap := map[Definition]time.Duration{}
		storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

		op := &operation.Operation{
			ID:            "some-op-id",
			SchedulerName: "test-scheduler",
			Status:        operation.StatusPending,
		}
		client.Close()

		err := storage.UpdateOperationExecutionHistory(context.Background(), op)
		require.Error(t, err)
		require.EqualError(t, err, "failed to update operation execution history: redis: client is closed")
	})

}

func TestListSchedulerActiveOperations(t *testing.T) {
	t.Run("list all operations", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		now := time.Now()
		operationsTTlMap := map[Definition]time.Duration{}
		storage := NewRedisOperationStorage(client, clockmock.NewFakeClock(now), operationsTTlMap)

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
			err := storage.CreateOperation(context.Background(), op)
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

	t.Run("fetching an operation that doesnt exists in the list, return without them", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		now := time.Now()
		operationsTTlMap := map[Definition]time.Duration{}
		storage := NewRedisOperationStorage(client, clockmock.NewFakeClock(now), operationsTTlMap)

		schedulerName := "test-scheduler"
		activeOperations := []*operation.Operation{
			{ID: uuid.NewString(), SchedulerName: schedulerName, Status: operation.StatusInProgress},
			{ID: uuid.NewString(), SchedulerName: schedulerName, Status: operation.StatusInProgress},
		}

		for _, op := range activeOperations {
			err := storage.CreateOperation(context.Background(), op)
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

		resultOperations, err := storage.ListSchedulerActiveOperations(context.Background(), schedulerName)
		require.NoError(t, err)
		require.Len(t, resultOperations, len(activeOperations))
	})
}

func TestCleanOperationsHistory(t *testing.T) {
	schedulerName := "test-scheduler"

	t.Run("with success", func(t *testing.T) {
		nowTime := time.Now()
		now, _ := time.Parse(time.RFC3339Nano, nowTime.Format(time.RFC3339Nano))

		operations := []*operation.Operation{
			{
				ID:             "some-op-id-1",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      now,
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
			{
				ID:             "some-op-id-2",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      now.Add(-23 * time.Hour),
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
			{
				ID:             "some-op-id-3",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      now.Add(-25 * time.Hour),
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
			{
				ID:             "some-op-id-4",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      now.Add(-29 * time.Hour),
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
		}

		t.Run("if operations history is not empty it deletes all operations and history", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTlMap := map[Definition]time.Duration{}
			storage := NewRedisOperationStorage(client, clock, operationsTTlMap)
			expectedOperations := []*operation.Operation{operations[1], operations[0]}

			for _, op := range operations {
				executionHistoryJson, err := json.Marshal(op.ExecutionHistory)
				require.NoError(t, err)

				err = client.ZAdd(context.Background(), storage.buildSchedulerHistoryOperationsKey(op.SchedulerName), &redis.Z{
					Member: op.ID,
					Score:  float64(op.CreatedAt.Unix()),
				}).Err()
				require.NoError(t, err)

				err = client.HSet(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
					idRedisKey:                 op.ID,
					schedulerNameRedisKey:      op.SchedulerName,
					statusRedisKey:             strconv.Itoa(int(op.Status)),
					definitionNameRedisKey:     op.DefinitionName,
					createdAtRedisKey:          op.CreatedAt.Format(time.RFC3339Nano),
					definitionContentsRedisKey: op.Input,
					executionHistoryRedisKey:   executionHistoryJson,
				}).Err()
				require.NoError(t, err)
			}

			operationsReturned, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName)
			assert.NoError(t, err)
			assert.NotEmptyf(t, operationsReturned, "expected at least one operation")
			assert.Equal(t, expectedOperations, operationsReturned)

			err = storage.CleanOperationsHistory(context.Background(), schedulerName)
			assert.NoError(t, err)

			// Assert that the operation's history sorted set is now empty
			operationIds, err := client.ZRangeByScore(context.Background(), fmt.Sprintf("operations:%s:lists:history", schedulerName), &redis.ZRangeBy{
				Min: "-inf",
				Max: "-inf",
			}).Result()
			require.NoError(t, err)
			assert.Empty(t, operationIds)
			// Assert that all operations hashes were deleted
			for _, op := range expectedOperations {
				result, _ := client.HGetAll(context.Background(), fmt.Sprintf("operations:%s:%s", schedulerName, op.ID)).Result()
				assert.Empty(t, result)
			}
		})

		t.Run("if operations history is empty it returns with no error", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTlMap := map[Definition]time.Duration{}
			storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

			operationsReturned, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName)
			assert.NoError(t, err)
			assert.Empty(t, operationsReturned)

			err = storage.CleanOperationsHistory(context.Background(), schedulerName)

			assert.NoError(t, err)
		})

	})

	t.Run("with error", func(t *testing.T) {

		t.Run("if client is closed it returns error", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTlMap := map[Definition]time.Duration{}
			storage := NewRedisOperationStorage(client, clock, operationsTTlMap)
			client.Close()

			err := storage.CleanOperationsHistory(context.Background(), schedulerName)

			assert.EqualError(t, err, "failed to list finished operations for \"test-scheduler\": redis: client is closed")
		})

	})

}
func TestCleanExpiredOperations(t *testing.T) {
	schedulerName := "test-scheduler"

	t.Run("with success", func(t *testing.T) {
		nowTime := time.Now()
		now, _ := time.Parse(time.RFC3339Nano, nowTime.Format(time.RFC3339Nano))

		operations := []*operation.Operation{
			{
				ID:             "some-op-id-1",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      now,
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
			{
				ID:             "some-op-id-2",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      now.Add(-23 * time.Hour),
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
			{
				ID:             "some-op-id-3",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      now.Add(-25 * time.Hour),
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
			{
				ID:             "some-op-id-4",
				SchedulerName:  schedulerName,
				Status:         operation.StatusFinished,
				DefinitionName: "test-definition",
				CreatedAt:      now.Add(-29 * time.Hour),
				Input:          []byte("hello test"),
				ExecutionHistory: []operation.OperationEvent{
					{
						CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
						Event:     "some-event",
					},
				},
			},
		}

		t.Run("if operations history is not empty it deletes all expired operations", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTlMap := map[Definition]time.Duration{}
			storage := NewRedisOperationStorage(client, clock, operationsTTlMap)
			expectedOperations := []*operation.Operation{operations[1], operations[0]}

			for _, op := range operations {
				executionHistoryJson, err := json.Marshal(op.ExecutionHistory)
				require.NoError(t, err)

				err = client.ZAdd(context.Background(), storage.buildSchedulerHistoryOperationsKey(op.SchedulerName), &redis.Z{
					Member: op.ID,
					Score:  float64(op.CreatedAt.Unix()),
				}).Err()
				require.NoError(t, err)

				err = client.HSet(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
					idRedisKey:                 op.ID,
					schedulerNameRedisKey:      op.SchedulerName,
					statusRedisKey:             strconv.Itoa(int(op.Status)),
					definitionNameRedisKey:     op.DefinitionName,
					createdAtRedisKey:          op.CreatedAt.Format(time.RFC3339Nano),
					definitionContentsRedisKey: op.Input,
					executionHistoryRedisKey:   executionHistoryJson,
				}).Err()
				require.NoError(t, err)
			}

			operationsReturned, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName)
			assert.NoError(t, err)
			assert.NotEmptyf(t, operationsReturned, "expected at least one operation")
			assert.Equal(t, expectedOperations, operationsReturned)

			err = storage.CleanExpiredOperations(context.Background(), schedulerName)
			assert.NoError(t, err)

			operationIds, err := client.ZRangeByScore(context.Background(), fmt.Sprintf("operations:%s:lists:history", schedulerName), &redis.ZRangeBy{
				Min: "-inf",
				Max: "+inf",
			}).Result()
			require.NoError(t, err)
			assert.NotEmpty(t, operationIds)

			for _, op := range expectedOperations {
				result, _ := client.HGetAll(context.Background(), fmt.Sprintf("operations:%s:%s", schedulerName, op.ID)).Result()
				require.Equal(t, op.ID, result[idRedisKey])
				require.Equal(t, op.SchedulerName, result[schedulerNameRedisKey])
				require.Equal(t, op.DefinitionName, result[definitionNameRedisKey])
				require.EqualValues(t, op.Input, result[definitionContentsRedisKey])
			}
		})

		t.Run("if operations history is empty it returns with no error", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTlMap := map[Definition]time.Duration{}
			storage := NewRedisOperationStorage(client, clock, operationsTTlMap)

			operationsReturned, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName)
			assert.NoError(t, err)
			assert.Empty(t, operationsReturned)

			err = storage.CleanExpiredOperations(context.Background(), schedulerName)

			assert.NoError(t, err)
		})

	})
}
