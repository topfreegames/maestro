package redis

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-redis/redis"
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
}

func NewRedisOperationStorage(client *redis.Client) *redisOperationStorage {
	return &redisOperationStorage{client}
}

// CreateOperation marshal and pushes the operation to the scheduler pending
// operations list.
func (r *redisOperationStorage) CreateOperation(ctx context.Context, op *operation.Operation, definitionContents []byte) error {
	pipe := r.client.WithContext(ctx).Pipeline()
	pipe.RPush(r.buildSchedulerPendingOperationsKey(op.SchedulerName), op.ID)
	// TODO(gabrielcorado): consider moving to go-redis v4 to avoid using HMSet.
	pipe.HMSet(r.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
		idRedisKey:                 op.ID,
		schedulerNameRedisKey:      op.SchedulerName,
		statusRedisKey:             strconv.Itoa(int(op.Status)),
		definitionNameRedisKey:     op.DefinitionName,
		definitionContentsRedisKey: definitionContents,
	})

	_, err := pipe.Exec()
	if err != nil {
		return errors.NewErrUnexpected("failed to create operation on redis: %s", err)
	}

	return nil
}

func (r *redisOperationStorage) GetOperation(ctx context.Context, schedulerName, operationID string) (*operation.Operation, []byte, error) {
	res, err := r.client.WithContext(ctx).HGetAll(fmt.Sprintf("operations:%s:%s", schedulerName, operationID)).Result()
	if err != nil {
		return nil, nil, errors.NewErrUnexpected("failed to fetch operation: %s", err)
	}

	if len(res) == 0 {
		return nil, nil, errors.NewErrNotFound("operation %s not found in scheduler %s", operationID, schedulerName)
	}

	statusInt, err := strconv.Atoi(res[statusRedisKey])
	if err != nil {
		return nil, nil, errors.NewErrEncoding("failed to parse operation status: %s", err)
	}

	op := &operation.Operation{
		ID: res[idRedisKey],
		SchedulerName: res[schedulerNameRedisKey],
		DefinitionName: res[definitionNameRedisKey],
		Status: operation.Status(statusInt),
	}

	return op, []byte(res[definitionContentsRedisKey]), nil
}

func (r *redisOperationStorage) buildSchedulerOperationKey(schedulerName, opID string) string {
	return fmt.Sprintf("operations:%s:%s", schedulerName, opID)
}

func (r *redisOperationStorage) buildSchedulerPendingOperationsKey(schedulerName string) string {
	return fmt.Sprintf("operations:%s:lists:pending", schedulerName)
}
