package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
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

// storageOp operation representation used by the storage. It will be used to
// encode/decode operations that come from Redis.
type storageOp struct {
	ID             string           `json:"id"`
	Status         operation.Status `json:"status"`
	Contents       []byte           `json:"contents"`
	DefinitionName string           `json:"definition_name"`
}

// newStorageOp creates a storageOp from `operation.Operation`.
func newStorageOp(op *operation.Operation) *storageOp {
	return &storageOp{
		ID:             op.ID,
		Status:         op.Status,
		Contents:       op.Definition.Marshal(),
		DefinitionName: op.Definition.Name(),
	}
}

// CreateOperation marshal and pushes the operation to the scheduler pending
// operations list.
func (r *redisOperationStorage) CreateOperation(ctx context.Context, op *operation.Operation) error {
	opBytes, err := json.Marshal(newStorageOp(op))
	if err != nil {
		return errors.NewErrEncoding("failed to encode redis value: %s", err)
	}

	err = r.client.WithContext(ctx).RPush(r.buildSchedulerPendingOperationsKey(op.SchedulerName), opBytes).Err()
	if err != nil {
		return errors.NewErrUnexpected("failed to create operation on redis: %s", err)
	}

	return nil
}

func (r *redisOperationStorage) buildSchedulerPendingOperationsKey(schedulerName string) string {
	return fmt.Sprintf("operations:%s:pending", schedulerName)
}
