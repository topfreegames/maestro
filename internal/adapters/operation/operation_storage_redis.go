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

	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"

	"github.com/topfreegames/maestro/internal/core/operations"

	"go.uber.org/zap"

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

type Definition string

// redisOperationStorage adapter of the OperationStorage port. It store store
// the operations in lists to keep their creation/update order.
type redisOperationStorage struct {
	client           *redis.Client
	clock            ports.Clock
	operationsTTlMap map[Definition]time.Duration
}

func NewRedisOperationStorage(client *redis.Client, clock ports.Clock, operationsTTlMap map[Definition]time.Duration) *redisOperationStorage {
	return &redisOperationStorage{client, clock, operationsTTlMap}
}

// CreateOperation marshal and pushes the operation to the scheduler pending
// operations list.
func (r *redisOperationStorage) CreateOperation(ctx context.Context, op *operation.Operation) (err error) {
	executionHistoryJson, err := json.Marshal(op.ExecutionHistory)
	if err != nil {
		return errors.NewErrUnexpected("failed to create operation on redis").WithError(err)
	}

	pipe := r.client.Pipeline()

	pipe.HSet(ctx, r.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
		idRedisKey:                 op.ID,
		schedulerNameRedisKey:      op.SchedulerName,
		statusRedisKey:             strconv.Itoa(int(op.Status)),
		definitionNameRedisKey:     op.DefinitionName,
		createdAtRedisKey:          op.CreatedAt.Format(time.RFC3339Nano),
		definitionContentsRedisKey: op.Input,
		executionHistoryRedisKey:   executionHistoryJson,
	})

	if tll, ok := r.operationsTTlMap[Definition(op.DefinitionName)]; ok {
		pipe.Expire(ctx, r.buildSchedulerOperationKey(op.SchedulerName, op.ID), tll)
	}

	metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
		_, err = pipe.Exec(ctx)
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
	operationHash, err := r.client.HGetAll(ctx, r.buildSchedulerOperationKey(schedulerName, operationID)).Result()
	if err != nil {
		return errors.NewErrUnexpected("failed to fetch operation for updating status").WithError(err)
	}
	op, err := buildOperationFromMap(operationHash)
	if err != nil {
		return errors.NewErrUnexpected("failed to parse operation for updating status").WithError(err)
	}

	pipe := r.client.Pipeline()

	err = r.updateOperationInSortedSet(ctx, pipe, schedulerName, op, status)
	if err != nil {
		return errors.NewErrUnexpected("failed to update the operation on sorted set").WithError(err)
	}

	pipe.HSet(ctx, r.buildSchedulerOperationKey(schedulerName, operationID), map[string]interface{}{
		statusRedisKey: strconv.Itoa(int(status)),
	})

	metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
		_, err = pipe.Exec(ctx)
		return err
	})

	if err != nil {
		return errors.NewErrUnexpected("failed to update operation status").WithError(err)
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
		return nil, errors.NewErrUnexpected("failed execute pipe for retrieving operations").WithError(err)
	}
	operations = make([]*operation.Operation, 0, len(operationsIDs))
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

func (r *redisOperationStorage) ListSchedulerFinishedOperations(ctx context.Context, schedulerName string) (operations []*operation.Operation, err error) {
	currentTime := r.clock.Now()
	lastDay := r.clock.Now().Add(-24 * time.Hour)
	operationsIDs, err := r.getFinishedOperationsFromHistory(ctx, schedulerName, lastDay, currentTime)
	var nonExistentOperations []string
	var executions = make(map[string]redis.Cmder)

	if err != nil {
		return nil, err
	}

	pipe := r.client.Pipeline()

	for _, operationID := range operationsIDs {
		cmd := pipe.HGetAll(ctx, fmt.Sprintf("operations:%s:%s", schedulerName, operationID))
		executions[operationID] = cmd
	}

	metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
		_, err = pipe.Exec(ctx)
		return err
	})

	if err != nil {
		return nil, errors.NewErrUnexpected("failed execute pipe for retrieving schedulers").WithError(err)
	}

	for opID, cmder := range executions {
		res, err := cmder.(*redis.StringStringMapCmd).Result()
		if err != nil && err != redis.Nil {
			return nil, errors.NewErrUnexpected("failed to fetch operation").WithError(err)
		}

		if len(res) == 0 {
			nonExistentOperations = append(nonExistentOperations, opID)
			continue
		}

		op, err := buildOperationFromMap(res)
		if err != nil {
			return nil, errors.NewErrUnexpected("failed to build operation from the hash").WithError(err)
		}
		operations = append(operations, op)
	}

	r.removeNonExistentOperationFromHistory(ctx, schedulerName, nonExistentOperations)

	return operations, nil
}

func (r *redisOperationStorage) CleanOperationsHistory(ctx context.Context, schedulerName string) error {
	operationsIDs, err := r.getFinishedOperationsFromHistory(ctx, schedulerName, time.Time{}, time.Now())
	if err != nil {
		return err
	}

	if len(operationsIDs) > 0 {
		operationIDsKeys := make([]string, len(operationsIDs))
		for i, operationID := range operationsIDs {
			operationIDsKeys[i] = r.buildSchedulerOperationKey(schedulerName, operationID)
		}
		pipe := r.client.Pipeline()
		pipe.Del(ctx, r.buildSchedulerHistoryOperationsKey(schedulerName))
		pipe.Del(ctx, operationIDsKeys...)

		metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
			_, err = pipe.Exec(ctx)
			return err
		})

		if err != nil {
			return fmt.Errorf("failed to delete operations from history: %w", err)
		}
	}

	return nil
}

func (r *redisOperationStorage) UpdateOperationDefinition(ctx context.Context, schedulerName string, operationID string, def operations.Definition) error {
	_, err := r.client.HSet(ctx, r.buildSchedulerOperationKey(schedulerName, operationID), map[string]interface{}{
		definitionNameRedisKey:     def.Name(),
		definitionContentsRedisKey: def.Marshal(),
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to update operation definition: %w", err)
	}
	return nil
}

func (r *redisOperationStorage) CleanExpiredOperations(ctx context.Context, schedulerName string) error {
	operationsIDs, err := r.getAllFinishedOperationIDs(ctx, schedulerName)
	if err != nil {
		return errors.NewErrUnexpected("failed to list operations for \"%s\" when trying to clean expired operations", schedulerName).WithError(err)
	}

	if len(operationsIDs) > 0 {

		pipe := r.client.Pipeline()
		for _, operationID := range operationsIDs {
			operationExists, _ := r.client.Exists(ctx, r.buildSchedulerOperationKey(schedulerName, operationID)).Result()
			if operationExists == 0 {
				pipe.ZRem(ctx, r.buildSchedulerHistoryOperationsKey(schedulerName), operationID)
				pipe.ZRem(ctx, r.buildSchedulerNoActionKey(schedulerName), operationID)
			}
		}

		metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
			_, err = pipe.Exec(ctx)
			return err
		})

		if err != nil {
			return fmt.Errorf("failed to clean expired operations: %w", err)
		}
	}
	return nil
}

func (r *redisOperationStorage) removeNonExistentOperationFromHistory(ctx context.Context, name string, operations []string) {
	go func() {
		pipe := r.client.Pipeline()
		for _, operationID := range operations {
			pipe.ZRem(ctx, r.buildSchedulerHistoryOperationsKey(name), operationID)
		}
		metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
			_, err := pipe.Exec(context.Background())
			if err != nil {
				zap.L().Error("failed to remove non-existent operations from history", zap.Error(err))
			}
			return err
		})
	}()
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

func (r *redisOperationStorage) buildSchedulerNoActionKey(schedulerName string) string {
	return fmt.Sprintf("operations:%s:lists:noaction", schedulerName)
}

func (r *redisOperationStorage) getAllFinishedOperationIDs(ctx context.Context, schedulerName string) ([]string, error) {
	operationsIDs, err := r.client.ZRangeByScore(ctx, r.buildSchedulerHistoryOperationsKey(schedulerName), &redis.ZRangeBy{
		Min: "-inf",
		Max: "+inf",
	}).Result()

	if err != nil {
		return nil, errors.NewErrUnexpected("failed to list operations from history for \"%s\"", schedulerName).WithError(err)
	}

	noActionOpIDs, err := r.client.ZRangeByScore(ctx, r.buildSchedulerNoActionKey(schedulerName), &redis.ZRangeBy{
		Min: "-inf",
		Max: "+inf",
	}).Result()

	if err != nil {
		return nil, errors.NewErrUnexpected("failed to list no action operations for \"%s\" ", schedulerName).WithError(err)
	}

	return append(operationsIDs, noActionOpIDs...), nil
}

func (r *redisOperationStorage) getFinishedOperationsFromHistory(ctx context.Context, schedulerName string, from, to time.Time) (operationsIDs []string, err error) {
	// TODO(guilhermocc): receive this time range as filter argument
	metrics.RunWithMetrics(operationStorageMetricLabel, func() error {
		operationsIDs, err = r.client.ZRangeByScore(ctx, r.buildSchedulerHistoryOperationsKey(schedulerName), &redis.ZRangeBy{
			Min: fmt.Sprint(from.Unix()),
			Max: fmt.Sprint(to.Unix()),
		}).Result()
		return err
	})
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to list finished operations for \"%s\"", schedulerName).WithError(err)
	}
	return operationsIDs, err
}

func (r *redisOperationStorage) updateOperationInSortedSet(ctx context.Context, pipe redis.Pipeliner, schedulerName string, op *operation.Operation, newStatus operation.Status) error {
	var listKey string
	operationTookAction, err := operationHasAction(op)
	if err != nil {
		return errors.NewErrUnexpected("failed to check if operation took action when updating status").WithError(err)
	}

	switch {
	case newStatus == operation.StatusInProgress:
		listKey = r.buildSchedulerActiveOperationsKey(schedulerName)
	case newStatus == operation.StatusFinished && !operationTookAction:
		listKey = r.buildSchedulerNoActionKey(schedulerName)
	default:
		listKey = r.buildSchedulerHistoryOperationsKey(schedulerName)
	}

	pipe.ZRem(ctx, r.buildSchedulerActiveOperationsKey(schedulerName), op.ID)
	pipe.ZRem(ctx, r.buildSchedulerHistoryOperationsKey(schedulerName), op.ID)
	pipe.ZRem(ctx, r.buildSchedulerNoActionKey(schedulerName), op.ID)
	pipe.ZAdd(ctx, listKey, &redis.Z{Member: op.ID, Score: float64(r.clock.Now().Unix())})
	return nil
}

func operationHasAction(op *operation.Operation) (bool, error) {
	if op.DefinitionName == healthcontroller.OperationName {
		def := healthcontroller.SchedulerHealthControllerDefinition{}
		err := def.Unmarshal(op.Input)
		if err != nil {
			return false, err
		}
		return def.TookAction != nil && *def.TookAction, nil
	}
	return true, nil
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
