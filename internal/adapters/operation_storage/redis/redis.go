package redis

import (
	"context"
	"fmt"
	"strconv"

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
	definitionContentsRedisKey = "definitionContents"
)

var _ ports.OperationStorage = (*redisOperationStorage)(nil)

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
func (r *redisOperationStorage) CreateOperation(ctx context.Context, op *operation.Operation, definitionContents []byte) error {
	err := r.client.HSet(ctx, r.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
		idRedisKey:                 op.ID,
		schedulerNameRedisKey:      op.SchedulerName,
		statusRedisKey:             strconv.Itoa(int(op.Status)),
		definitionNameRedisKey:     op.DefinitionName,
		definitionContentsRedisKey: definitionContents,
	}).Err()

	if err != nil {
		return errors.NewErrUnexpected("failed to create operation on redis").WithError(err)
	}

	return nil
}

func (r *redisOperationStorage) GetOperation(ctx context.Context, schedulerName, operationID string) (*operation.Operation, []byte, error) {
	res, err := r.client.HGetAll(ctx, fmt.Sprintf("operations:%s:%s", schedulerName, operationID)).Result()
	if err != nil {
		return nil, nil, errors.NewErrUnexpected("failed to fetch operation").WithError(err)
	}

	if len(res) == 0 {
		return nil, nil, errors.NewErrNotFound("operation %s not found in scheduler %s", operationID, schedulerName)
	}

	statusInt, err := strconv.Atoi(res[statusRedisKey])
	if err != nil {
		return nil, nil, errors.NewErrEncoding("failed to parse operation status").WithError(err)
	}

	op := &operation.Operation{
		ID:             res[idRedisKey],
		SchedulerName:  res[schedulerNameRedisKey],
		DefinitionName: res[definitionNameRedisKey],
		Status:         operation.Status(statusInt),
	}

	return op, []byte(res[definitionContentsRedisKey]), nil
}

func (r *redisOperationStorage) UpdateOperationStatus(ctx context.Context, schedulerName, operationID string, status operation.Status) error {
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

	if _, err := pipe.Exec(ctx); err != nil {
		return errors.NewErrUnexpected("failed to update operations").WithError(err)
	}

	return nil
}

func (r *redisOperationStorage) ListSchedulerActiveOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error) {
	operationsIDs, err := r.client.ZRange(ctx, r.buildSchedulerActiveOperationsKey(schedulerName), 0, -1).Result()
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to list active operations for \"%s\"", schedulerName).WithError(err)
	}

	operations := make([]*operation.Operation, len(operationsIDs))
	for i, operationID := range operationsIDs {
		op, _, err := r.GetOperation(ctx, schedulerName, operationID)
		if err != nil {
			return nil, err
		}

		operations[i] = op
	}

	return operations, nil
}

func (r *redisOperationStorage) ListSchedulerFinishedOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error) {
	operationsIDs, err := r.client.ZRange(ctx, r.buildSchedulerHistoryOperationsKey(schedulerName), 0, -1).Result()
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to list finished operations for \"%s\"", schedulerName).WithError(err)
	}

	operations := make([]*operation.Operation, len(operationsIDs))
	for i, operationID := range operationsIDs {
		op, _, err := r.GetOperation(ctx, schedulerName, operationID)
		if err != nil {
			return nil, err
		}

		operations[i] = op
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
