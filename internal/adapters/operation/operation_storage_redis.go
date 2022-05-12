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

package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/topfreegames/maestro/internal/adapters/metrics"

	"github.com/go-redis/redis/v8"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
)

const (
	// list of hash keys used on redis
	idRedisKey                 = "id"
	schedulerNameRedisKey      = "schedulerName"
	statusRedisKey             = "status"
	definitionNameRedisKey     = "definitionName"
	createdAtRedisKey          = "createdAt"
	definitionContentsRedisKey = "definitionContents"
	executionHistoryRedisKey   = "executionHistory"
)

var _ ports.OperationStorage = (*redisOperationStorage)(nil)

const operationStorageMetricLabel = "operation-storage"

// redisOperationStorage adapter of the OperationStorage port. It store store
// the operations in lists to keep their creation/update order.
type redisOperationStorage struct {
	client *redis.Client
	clock  ports.Clock
}

func NewRedisOperationStorage(client *redis.Client, clock ports.Clock) *redisOperationStorage {
	return &redisOperationStorage{client, clock}
}

// CreateOperation marshal and pushes the operation to the scheduler pending
// operations list.
func (r *redisOperationStorage) CreateOperation(ctx context.Context, op *operation.Operation) (err error) {
	executionHistoryJson, err := json.Marshal(op.ExecutionHistory)
	if err != nil {
		return errors.NewErrUnexpected("failed to create operation on redis").WithError(err)
	}

	metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
		err = r.client.HSet(ctx, r.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
			idRedisKey:                 op.ID,
			schedulerNameRedisKey:      op.SchedulerName,
			statusRedisKey:             strconv.Itoa(int(op.Status)),
			definitionNameRedisKey:     op.DefinitionName,
			createdAtRedisKey:          op.CreatedAt.Format(time.RFC3339Nano),
			definitionContentsRedisKey: op.Input,
			executionHistoryRedisKey:   executionHistoryJson,
		}).Err()
		return err
	})

	if err != nil {
		return errors.NewErrUnexpected("failed to create operation on redis").WithError(err)
	}

	return nil
}

func (r *redisOperationStorage) GetOperation(ctx context.Context, schedulerName, operationID string) (op *operation.Operation, err error) {
	var res map[string]string
	metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
		res, err = r.client.HGetAll(ctx, fmt.Sprintf("operations:%s:%s", schedulerName, operationID)).Result()
		return err
	})
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to fetch operation").WithError(err)
	}

	if len(res) == 0 {
		return nil, errors.NewErrNotFound("operation %s not found in scheduler %s", operationID, schedulerName)
	}

	return buildOperationFromMap(res)
}

func (r *redisOperationStorage) UpdateOperationStatus(ctx context.Context, schedulerName, operationID string, status operation.Status) (err error) {
	pipe := r.client.Pipeline()
	pipe.ZRem(ctx, r.buildSchedulerActiveOperationsKey(schedulerName), operationID)
	pipe.ZRem(ctx, r.buildSchedulerHistoryOperationsKey(schedulerName), operationID)
	pipe.HSet(ctx, r.buildSchedulerOperationKey(schedulerName, operationID), map[string]interface{}{
		statusRedisKey: strconv.Itoa(int(status)),
	})

	operationsList := r.buildSchedulerHistoryOperationsKey(schedulerName)
	if status == operation.StatusInProgress {
		operationsList = r.buildSchedulerActiveOperationsKey(schedulerName)
	}

	pipe.ZAdd(ctx, operationsList, &redis.Z{
		Member: operationID,
		Score:  float64(r.clock.Now().Unix()),
	})
	metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
		_, err = pipe.Exec(ctx)
		return err
	})

	if err != nil {
		return errors.NewErrUnexpected("failed to update operations").WithError(err)
	}

	return nil
}

func (r *redisOperationStorage) UpdateOperationExecutionHistory(ctx context.Context, op *operation.Operation) (err error) {
	jsonExecutionHistory, err := json.Marshal(op.ExecutionHistory)
	if err != nil {
		return errors.NewErrUnexpected("failed to marshal operation execution history").WithError(err)
	}
	metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
		err = r.client.HSet(ctx, r.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
			executionHistoryRedisKey: jsonExecutionHistory,
		}).Err()
		return err
	})

	if err != nil {
		return errors.NewErrUnexpected("failed to update operation execution history").WithError(err)
	}

	return nil
}

func (r *redisOperationStorage) ListSchedulerActiveOperations(ctx context.Context, schedulerName string) (operations []*operation.Operation, err error) {
	var operationsIDs []string
	metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
		operationsIDs, err = r.client.ZRange(ctx, r.buildSchedulerActiveOperationsKey(schedulerName), 0, -1).Result()
		return err
	})
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to list active operations for \"%s\"", schedulerName).WithError(err)
	}

	operations = make([]*operation.Operation, len(operationsIDs))
	for i, operationID := range operationsIDs {
		op, err := r.GetOperation(ctx, schedulerName, operationID)
		if err != nil {
			return nil, err
		}

		operations[i] = op
	}

	return operations, nil
}

func (r *redisOperationStorage) ListSchedulerFinishedOperations(ctx context.Context, schedulerName string) (operations []*operation.Operation, err error) {
	operationsIDs, err := r.getFinishedOperationsFromHistory(ctx, schedulerName)
	if err != nil {
		return nil, err
	}

	pipe := r.client.Pipeline()

	for _, operationID := range operationsIDs {
		pipe.HGetAll(ctx, fmt.Sprintf("operations:%s:%s", schedulerName, operationID))
	}

	var cmders []redis.Cmder
	metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
		cmders, err = pipe.Exec(ctx)
		return err
	})

	if err != nil {
		return nil, errors.NewErrUnexpected("failed execute pipe for retrieving schedulers").WithError(err)
	}

	for _, cmder := range cmders {
		res, err := cmder.(*redis.StringStringMapCmd).Result()
		if err != nil && err != redis.Nil {
			return nil, errors.NewErrUnexpected("failed to fetch operation").WithError(err)
		}

		if len(res) == 0 {
			continue
		}

		op, err := buildOperationFromMap(res)
		if err != nil {
			return nil, errors.NewErrUnexpected("failed to build operation from the hash").WithError(err)
		}
		operations = append(operations, op)
	}

	return operations, nil
}

func (r *redisOperationStorage) buildSchedulerOperationKey(schedulerName, opID string) string {
	return fmt.Sprintf("operations:%s:%s", schedulerName, opID)
}

func (r *redisOperationStorage) buildSchedulerActiveOperationsKey(schedulerName string) string {
	return fmt.Sprintf("operations:%s:lists:active", schedulerName)
}

func (r *redisOperationStorage) buildSchedulerHistoryOperationsKey(schedulerName string) string {
	return fmt.Sprintf("operations:%s:lists:history", schedulerName)
}

func (r *redisOperationStorage) getFinishedOperationsFromHistory(ctx context.Context, schedulerName string) (operationsIDs []string, err error) {
	metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
		operationsIDs, err = r.client.ZRange(ctx, r.buildSchedulerHistoryOperationsKey(schedulerName), 0, -1).Result()
		return err
	})
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to list finished operations for \"%s\"", schedulerName).WithError(err)
	}
	return operationsIDs, err
}

func buildOperationFromMap(opMap map[string]string) (*operation.Operation, error) {
	var executionHistory []operation.OperationEvent
	if history, ok := opMap[executionHistoryRedisKey]; ok {
		err := json.Unmarshal([]byte(history), &executionHistory)
		if err != nil {
			return nil, errors.NewErrEncoding("failed to parse operation execution history").WithError(err)
		}
	}

	statusInt, err := strconv.Atoi(opMap[statusRedisKey])
	if err != nil {
		return nil, errors.NewErrEncoding("failed to parse operation status").WithError(err)
	}

	createdAt, err := time.Parse(time.RFC3339Nano, opMap[createdAtRedisKey])
	if err != nil {
		return nil, errors.NewErrEncoding("failed to parse operation createdAt field").WithError(err)
	}

	return &operation.Operation{
		ID:               opMap[idRedisKey],
		SchedulerName:    opMap[schedulerNameRedisKey],
		DefinitionName:   opMap[definitionNameRedisKey],
		CreatedAt:        createdAt,
		Status:           operation.Status(statusInt),
		Input:            []byte(opMap[definitionContentsRedisKey]),
		ExecutionHistory: executionHistory,
	}, nil
}
