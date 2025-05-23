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

	"github.com/golang/mock/gomock"
	"github.com/topfreegames/maestro/internal/core/operations"
	clockmock "github.com/topfreegames/maestro/internal/core/ports/clock_mock.go"

	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"
	mockoperation "github.com/topfreegames/maestro/internal/core/operations/mock"

	"github.com/stretchr/testify/assert"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/test"
)

const definitionName string = "test-definition"

func TestCreateOperation(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		operationsTTLMap := map[Definition]time.Duration{}
		definitionProvider, _ := createOperationDefinitionProvider(t)
		storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)
		createdAtString := "2020-01-01T00:00:00.001Z"
		createdAt, _ := time.Parse(time.RFC3339Nano, createdAtString)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: definitionName,
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
		operationsTTLMap := map[Definition]time.Duration{
			healthcontroller.OperationName: time.Second,
		}
		definitionProvider, _ := createOperationDefinitionProvider(t)
		storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)
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
		operationsTTLMap := map[Definition]time.Duration{}
		definitionProvider, _ := createOperationDefinitionProvider(t)
		storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

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
		operationsTTLMap := map[Definition]time.Duration{}
		definitionProvider, _ := createOperationDefinitionProvider(t)
		storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

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
		operationsTTLMap := map[Definition]time.Duration{}
		definitionProvider, _ := createOperationDefinitionProvider(t)
		storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

		operationStored, err := storage.GetOperation(context.Background(), "test-scheduler", "inexistent-id")
		require.ErrorIs(t, errors.ErrNotFound, err)
		require.Nil(t, operationStored)
	})

	t.Run("fail to parse created at field", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		operationsTTLMap := map[Definition]time.Duration{}
		definitionProvider, _ := createOperationDefinitionProvider(t)
		storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

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
		operationsTTLMap := map[Definition]time.Duration{}
		definitionProvider, _ := createOperationDefinitionProvider(t)
		storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

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
		now := time.Date(2022, time.January, 19, 6, 12, 15, 0, time.UTC)

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

		t.Run("return operations list and total using pagination parameters, it returns less elements than the page size", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

			expectedOperations := []*operation.Operation{operations[0], operations[1], operations[2], operations[3]}
			page := int64(0)
			pageSize := int64(4)

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

			operationsReturned, total, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName, page, pageSize, "desc")
			assert.NoError(t, err)
			assert.NotEmptyf(t, operationsReturned, "expected at least one operation")
			assert.Equal(t, expectedOperations, operationsReturned)
			assert.Equal(t, int64(4), total)
		})

		t.Run("return operations list and total using pagination parameters, it returns same element's number as the page size", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

			expectedOperations := []*operation.Operation{operations[2], operations[3]}
			page := int64(1)
			pageSize := int64(2)

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

			operationsReturned, total, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName, page, pageSize, "desc")
			assert.NoError(t, err)
			assert.NotEmptyf(t, operationsReturned, "expected at least one operation")
			assert.Equal(t, expectedOperations, operationsReturned)
			assert.Equal(t, int64(4), total)
		})

		t.Run("return operations list and total using pagination parameters, it returns an empty page", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

			page := int64(2)
			pageSize := int64(2)

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

			operationsReturned, total, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName, page, pageSize, "")
			assert.NoError(t, err)
			assert.Empty(t, operationsReturned, "expected result to be empty")
			assert.Equal(t, int64(4), total)
		})

		t.Run("return empty list when there is no operation stored", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

			operationsReturned, total, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName, 0, 10, "")
			assert.NoError(t, err)
			assert.Empty(t, operationsReturned, "expected result to be empty")
			assert.Equal(t, int64(0), total)
		})

		t.Run("return no error when there is expired operations, and remove expired ones from history", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

			for _, op := range operations {
				err := client.ZAdd(context.Background(), storage.buildSchedulerHistoryOperationsKey(op.SchedulerName), &redis.Z{
					Member: op.ID,
					Score:  float64(clock.Now().Unix()),
				}).Err()
				require.NoError(t, err)
			}

			operationsReturned, _, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName, 0, 10, "")
			assert.NoError(t, err)
			assert.Empty(t, operationsReturned)
			ids, _ := client.ZRange(context.Background(), storage.buildSchedulerHistoryOperationsKey(schedulerName), 0, -1).Result()
			assert.Empty(t, ids)
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
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

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

			_, _, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName, 0, 10, "")
			assert.ErrorContains(t, err, "failed to build operation from the hash: failed to parse operation status: strconv.Atoi: parsing \"\": invalid syntax")
		})

		t.Run("return error when there is some error in redis call", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)
			client.Close()
			_, _, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName, 0, 10, "")
			assert.ErrorContains(t, err, "failed to clean scheduler expired operations: failed to list operations for \"test-scheduler\" when trying to clean expired operations")
		})
	})
}

func TestUpdateOperationStatus(t *testing.T) {
	schedulerName := "test-scheduler"

	baseOperation := &operation.Operation{
		ID:             "some-op-id-1",
		SchedulerName:  schedulerName,
		DefinitionName: "test-definition",
		CreatedAt:      time.Date(2022, time.November, 19, 6, 12, 15, 0, time.UTC),
		Input:          []byte("hello test"),
		ExecutionHistory: []operation.OperationEvent{
			{
				CreatedAt: time.Date(1999, time.November, 19, 6, 12, 15, 0, time.UTC),
				Event:     "some-event",
			},
		},
	}

	t.Run("with success", func(t *testing.T) {

		t.Run("transit operation from pending to in-progress, update status and store it in active sorted set", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			now := time.Now()
			clock := clockmock.NewFakeClock(now)
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, mockDefinition := createOperationDefinitionProvider(t)
			mockDefinition.EXPECT().HasNoAction().Return(false)
			mockDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

			op := *baseOperation
			op.Status = operation.StatusPending

			// Create Operation hashmap
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

			err = storage.UpdateOperationStatus(context.Background(), op.SchedulerName, op.ID, operation.StatusInProgress)
			assert.NoError(t, err)

			// Assert the status was updated
			updatedStatus, err := client.HGet(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID), statusRedisKey).Result()
			assert.NoError(t, err)
			intStatus, err := strconv.Atoi(updatedStatus)
			assert.NoError(t, err)
			assert.Equal(t, operation.StatusInProgress, operation.Status(intStatus))

			// Assert the operation was added to the active sorted set
			updatedAt, err := client.ZScore(context.Background(), storage.buildSchedulerActiveOperationsKey(op.SchedulerName), op.ID).Result()
			assert.NoError(t, err)
			assert.Equal(t, float64(now.Unix()), updatedAt)
		})

		t.Run("transit operation from in-progress to finished, update status and store it in history sorted set", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			now := time.Now()
			clock := clockmock.NewFakeClock(now)
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, mockDefinition := createOperationDefinitionProvider(t)
			mockDefinition.EXPECT().HasNoAction().Return(false)
			mockDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

			op := *baseOperation
			op.Status = operation.StatusInProgress

			// Create Operation hashmap and add it to active sorted set
			err := client.ZAdd(context.Background(), storage.buildSchedulerActiveOperationsKey(op.SchedulerName), &redis.Z{
				Member: op.ID,
				Score:  float64(op.CreatedAt.Unix()),
			}).Err()
			require.NoError(t, err)
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

			err = storage.UpdateOperationStatus(context.Background(), op.SchedulerName, op.ID, operation.StatusFinished)
			assert.NoError(t, err)

			// Assert the status was updated
			updatedStatus, err := client.HGet(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID), statusRedisKey).Result()
			assert.NoError(t, err)
			intStatus, err := strconv.Atoi(updatedStatus)
			assert.NoError(t, err)
			assert.Equal(t, operation.StatusFinished, operation.Status(intStatus))

			// Assert the operation was added to the history sorted set
			updatedAt, err := client.ZScore(context.Background(), storage.buildSchedulerHistoryOperationsKey(op.SchedulerName), op.ID).Result()
			assert.NoError(t, err)
			assert.Equal(t, float64(now.Unix()), updatedAt)

			// Assert the operation was removed from the active sorted set
			_, err = client.ZScore(context.Background(), storage.buildSchedulerActiveOperationsKey(op.SchedulerName), op.ID).Result()
			assert.ErrorIs(t, redis.Nil, err)
		})

		t.Run("transit a no-action operation from in-progress to finished, update status and store it in noaction sorted set", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			now := time.Now()
			clock := clockmock.NewFakeClock(now)
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, mockDefinition := createOperationDefinitionProvider(t)
			mockDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil)
			mockDefinition.EXPECT().HasNoAction().Return(true)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)
			op := *baseOperation
			op.Status = operation.StatusInProgress

			// Create Operation hashmap and add it to active sorted set
			err := client.ZAdd(context.Background(), storage.buildSchedulerActiveOperationsKey(op.SchedulerName), &redis.Z{
				Member: op.ID,
				Score:  float64(op.CreatedAt.Unix()),
			}).Err()
			require.NoError(t, err)
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

			err = storage.UpdateOperationStatus(context.Background(), op.SchedulerName, op.ID, operation.StatusFinished)
			assert.NoError(t, err)

			// Assert the status was updated
			updatedStatus, err := client.HGet(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID), statusRedisKey).Result()
			assert.NoError(t, err)
			intStatus, err := strconv.Atoi(updatedStatus)
			assert.NoError(t, err)
			assert.Equal(t, operation.StatusFinished, operation.Status(intStatus))

			// Assert the operation was added to the noaction sorted set
			updatedAt, err := client.ZScore(context.Background(), storage.buildSchedulerNoActionKey(op.SchedulerName), op.ID).Result()
			assert.NoError(t, err)
			assert.Equal(t, float64(now.Unix()), updatedAt)

			// Assert the operation was removed from the active sorted set
			_, err = client.ZScore(context.Background(), storage.buildSchedulerActiveOperationsKey(op.SchedulerName), op.ID).Result()
			assert.ErrorIs(t, redis.Nil, err)
		})
	})

	t.Run("with error", func(t *testing.T) {

		t.Run("returns error when the client is closed", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			now := time.Now()
			clock := clockmock.NewFakeClock(now)
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)
			op := *baseOperation

			err := client.Close()
			require.NoError(t, err)

			err = storage.UpdateOperationStatus(context.Background(), op.SchedulerName, op.ID, operation.StatusInProgress)
			assert.ErrorContains(t, err, "failed to fetch operation for updating status: redis: client is closed")
		})

		t.Run("returns error when the definition stored in redis is broken", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			now := time.Now()
			clock := clockmock.NewFakeClock(now)
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)
			op := *baseOperation

			err := client.Close()
			require.NoError(t, err)

			err = storage.UpdateOperationStatus(context.Background(), op.SchedulerName, op.ID, operation.StatusInProgress)
			assert.ErrorContains(t, err, "failed to fetch operation for updating status: redis: client is closed")
		})
	})

}

func TestUpdateOperationDefinition(t *testing.T) {
	schedulerName := "test-scheduler"

	op := operation.Operation{
		ID:             "some-op-id-1",
		SchedulerName:  schedulerName,
		Status:         operation.StatusFinished,
		DefinitionName: "test-definition",
		Input:          []byte("hello test"),
	}

	t.Run("return no error and update operation definition", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		operationsTTLMap := map[Definition]time.Duration{}
		definitionProvider, _ := createOperationDefinitionProvider(t)
		storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

		tookAction := true
		definition := healthcontroller.Definition{TookAction: &tookAction}

		err := client.HSet(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
			idRedisKey:                 op.ID,
			schedulerNameRedisKey:      op.SchedulerName,
			statusRedisKey:             strconv.Itoa(int(op.Status)),
			definitionNameRedisKey:     op.DefinitionName,
			createdAtRedisKey:          op.CreatedAt.Format(time.RFC3339Nano),
			definitionContentsRedisKey: op.Input,
		}).Err()
		require.NoError(t, err)

		err = storage.UpdateOperationDefinition(context.Background(), op.SchedulerName, op.ID, &definition)
		require.NoError(t, err)

		resultMap := client.HGetAll(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID)).Val()
		updatedDefinition := healthcontroller.Definition{}
		definitionContents := resultMap[definitionContentsRedisKey]
		err = updatedDefinition.Unmarshal([]byte(definitionContents))
		require.NoError(t, err)
		assert.Equal(t, definition, updatedDefinition)
	})

	t.Run("return error if some error occurs with redis connection", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		clock := clockmock.NewFakeClock(time.Now())
		operationsTTLMap := map[Definition]time.Duration{}
		definitionProvider, _ := createOperationDefinitionProvider(t)
		storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)
		tookAction := true
		definition := healthcontroller.Definition{TookAction: &tookAction}

		err := client.HSet(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
			idRedisKey:                 op.ID,
			schedulerNameRedisKey:      op.SchedulerName,
			statusRedisKey:             strconv.Itoa(int(op.Status)),
			definitionNameRedisKey:     op.DefinitionName,
			createdAtRedisKey:          op.CreatedAt.Format(time.RFC3339Nano),
			definitionContentsRedisKey: op.Input,
		}).Err()
		require.NoError(t, err)
		client.Close()
		err = storage.UpdateOperationDefinition(context.Background(), op.SchedulerName, op.ID, &definition)
		require.ErrorContains(t, err, "failed to update operation definition: redis: client is closed")
	})

}

func TestUpdateOperationExecutionHistory(t *testing.T) {

	t.Run("set execution history with value", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		now := time.Now()
		clock := clockmock.NewFakeClock(now)
		operationsTTLMap := map[Definition]time.Duration{}
		definitionProvider, _ := createOperationDefinitionProvider(t)
		storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

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
		operationsTTLMap := map[Definition]time.Duration{}
		definitionProvider, _ := createOperationDefinitionProvider(t)
		storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

		op := &operation.Operation{
			ID:            "some-op-id",
			SchedulerName: "test-scheduler",
			Status:        operation.StatusPending,
		}
		client.Close()

		err := storage.UpdateOperationExecutionHistory(context.Background(), op)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to update operation execution history: redis: client is closed")
	})

}

func TestListSchedulerActiveOperations(t *testing.T) {
	t.Run("list all operations", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		now := time.Now()
		operationsTTLMap := map[Definition]time.Duration{}
		definitionProvider, mockDefinition := createOperationDefinitionProvider(t)
		mockDefinition.EXPECT().HasNoAction().Return(false).AnyTimes()
		mockDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil).AnyTimes()
		storage := NewRedisOperationStorage(client, clockmock.NewFakeClock(now), operationsTTLMap, definitionProvider)

		schedulerName := "test-scheduler"
		activeOperations := []*operation.Operation{
			{ID: uuid.NewString(), SchedulerName: schedulerName, DefinitionName: definitionName, Status: operation.StatusInProgress},
			{ID: uuid.NewString(), SchedulerName: schedulerName, DefinitionName: definitionName, Status: operation.StatusInProgress},
			{ID: uuid.NewString(), SchedulerName: schedulerName, DefinitionName: definitionName, Status: operation.StatusInProgress},
		}

		pendingOperations := []*operation.Operation{
			{ID: uuid.NewString(), SchedulerName: schedulerName, DefinitionName: definitionName, Status: operation.StatusPending},
			{ID: uuid.NewString(), SchedulerName: schedulerName, DefinitionName: definitionName, Status: operation.StatusPending},
			{ID: uuid.NewString(), SchedulerName: schedulerName, DefinitionName: definitionName, Status: operation.StatusPending},
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
		operationsTTLMap := map[Definition]time.Duration{}
		definitionProvider, mockDefinition := createOperationDefinitionProvider(t)
		mockDefinition.EXPECT().HasNoAction().Return(false).AnyTimes()
		mockDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil).AnyTimes()
		storage := NewRedisOperationStorage(client, clockmock.NewFakeClock(now), operationsTTLMap, definitionProvider)

		schedulerName := "test-scheduler"
		activeOperations := []*operation.Operation{
			{ID: uuid.NewString(), SchedulerName: schedulerName, DefinitionName: definitionName, Status: operation.StatusInProgress},
			{ID: uuid.NewString(), SchedulerName: schedulerName, DefinitionName: definitionName, Status: operation.StatusInProgress},
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
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

			expectedOperations := []*operation.Operation{operations[0], operations[1]}

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

			err := storage.CleanOperationsHistory(context.Background(), schedulerName)
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
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

			operationsReturned, total, err := storage.ListSchedulerFinishedOperations(context.Background(), schedulerName, 0, 10, "")
			assert.NoError(t, err)
			assert.Empty(t, operationsReturned)
			assert.Equal(t, total, int64(0))

			err = storage.CleanOperationsHistory(context.Background(), schedulerName)

			assert.NoError(t, err)
		})

	})

	t.Run("with error", func(t *testing.T) {

		t.Run("if client is closed it returns error", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

			client.Close()

			err := storage.CleanOperationsHistory(context.Background(), schedulerName)

			assert.ErrorContains(t, err, "failed to list finished operations for \"test-scheduler\": redis: client is closed")
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

		t.Run("if operations lists are not empty it deletes all expired operations", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

			for _, op := range operations[:2] {
				err := client.ZAdd(context.Background(), storage.buildSchedulerHistoryOperationsKey(op.SchedulerName), &redis.Z{
					Member: op.ID,
					Score:  float64(op.CreatedAt.Unix()),
				}).Err()
				require.NoError(t, err)
			}

			for _, op := range operations[2:] {
				err := client.ZAdd(context.Background(), storage.buildSchedulerNoActionKey(op.SchedulerName), &redis.Z{
					Member: op.ID,
					Score:  float64(op.CreatedAt.Unix()),
				}).Err()
				require.NoError(t, err)
			}

			firstOperation := operations[0]
			executionHistoryJson, err := json.Marshal(firstOperation.ExecutionHistory)
			require.NoError(t, err)
			err = client.HSet(context.Background(), storage.buildSchedulerOperationKey(operations[0].SchedulerName, operations[0].ID), map[string]interface{}{
				idRedisKey:                 firstOperation.ID,
				schedulerNameRedisKey:      firstOperation.SchedulerName,
				statusRedisKey:             strconv.Itoa(int(firstOperation.Status)),
				definitionNameRedisKey:     firstOperation.DefinitionName,
				createdAtRedisKey:          firstOperation.CreatedAt.Format(time.RFC3339Nano),
				definitionContentsRedisKey: firstOperation.Input,
				executionHistoryRedisKey:   executionHistoryJson,
			}).Err()
			require.NoError(t, err)

			err = storage.CleanExpiredOperations(context.Background(), schedulerName)
			assert.NoError(t, err)

			operationIDs, err := client.ZRangeByScore(context.Background(), storage.buildSchedulerHistoryOperationsKey(schedulerName), &redis.ZRangeBy{
				Min: "-inf",
				Max: "+inf",
			}).Result()
			require.NoError(t, err)
			assert.NotEmpty(t, operationIDs)
			assert.Contains(t, operationIDs, operations[0].ID)
			assert.NotContains(t, operationIDs, operations[1].ID)

			noActionOpIDs, err := client.ZRangeByScore(context.Background(), storage.buildSchedulerNoActionKey(schedulerName), &redis.ZRangeBy{
				Min: "-inf",
				Max: "+inf",
			}).Result()
			require.NoError(t, err)
			assert.Empty(t, noActionOpIDs)
		})

		t.Run("if operations list is empty it returns with no error", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

			err := storage.CleanExpiredOperations(context.Background(), schedulerName)
			assert.NoError(t, err)

			operationIDs, err := client.ZRangeByScore(context.Background(), storage.buildSchedulerHistoryOperationsKey(schedulerName), &redis.ZRangeBy{
				Min: "-inf",
				Max: "+inf",
			}).Result()
			require.NoError(t, err)
			assert.Empty(t, operationIDs)

			noActionOpIDs, err := client.ZRangeByScore(context.Background(), storage.buildSchedulerNoActionKey(schedulerName), &redis.ZRangeBy{
				Min: "-inf",
				Max: "+inf",
			}).Result()
			require.NoError(t, err)
			assert.Empty(t, noActionOpIDs)
		})

		t.Run("if no action operation is empty it deletes all expired operations", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)

			for _, op := range operations {
				executionHistoryJson, err := json.Marshal(op.ExecutionHistory)
				require.NoError(t, err)

				err = client.ZAdd(context.Background(), storage.buildSchedulerHistoryOperationsKey(op.SchedulerName), &redis.Z{
					Member: op.ID,
					Score:  float64(op.CreatedAt.Unix()),
				}).Err()
				require.NoError(t, err)

				for _, op := range operations[0:1] {
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

				err = storage.CleanExpiredOperations(context.Background(), schedulerName)
				assert.NoError(t, err)

				operationIds, err := client.ZRangeByScore(context.Background(), fmt.Sprintf("operations:%s:lists:history", schedulerName), &redis.ZRangeBy{
					Min: "-inf",
					Max: "+inf",
				}).Result()
				require.NoError(t, err)
				assert.NotEmpty(t, operationIds)
				assert.Contains(t, operationIds, operations[0].ID, operations[1].ID)
				assert.NotContains(t, operationIds, operations[2].ID, operations[3].ID)
			}
		})

	})

	t.Run("with error", func(t *testing.T) {

		t.Run("if client is closed it returns error", func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			clock := clockmock.NewFakeClock(time.Now())
			operationsTTLMap := map[Definition]time.Duration{}
			definitionProvider, _ := createOperationDefinitionProvider(t)
			storage := NewRedisOperationStorage(client, clock, operationsTTLMap, definitionProvider)
			client.Close()

			err := storage.CleanExpiredOperations(context.Background(), schedulerName)

			assert.ErrorContains(t, err, "failed to list operations for \"test-scheduler\" when trying to clean expired operations")
		})

	})
}
func createOperationDefinitionProvider(t *testing.T) (map[string]operations.DefinitionConstructor, *mockoperation.MockDefinition) {
	mockCtrl := gomock.NewController(t)
	mockDefinition := mockoperation.NewMockDefinition(mockCtrl)
	definitionMap := map[string]operations.DefinitionConstructor{
		definitionName: func() operations.Definition {
			return mockDefinition
		},
	}

	return definitionMap, mockDefinition
}
