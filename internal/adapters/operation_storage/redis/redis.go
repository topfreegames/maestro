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
func (r *redisOperationStorage) CreateOperation(ctx context.Context, op *operation.Operation) error {
	pipe := r.client.WithContext(ctx).Pipeline()
	pipe.RPush(r.buildSchedulerPendingOperationsKey(op.SchedulerName), op.ID)
	// TODO(gabrielcorado): consider moving to go-redis v4 to avoid using HMSet.
	pipe.HMSet(r.buildSchedulerOperationKey(op.SchedulerName, op.ID), map[string]interface{}{
		idRedisKey:                 op.ID,
		schedulerNameRedisKey:      op.SchedulerName,
		statusRedisKey:             strconv.Itoa(int(op.Status)),
		definitionNameRedisKey:     op.Definition.Name(),
		definitionContentsRedisKey: op.Definition.Marshal(),
	})

	_, err := pipe.Exec()
	if err != nil {
		return errors.NewErrUnexpected("failed to create operation on redis: %s", err)
	}

	return nil
}

func (r *redisOperationStorage) buildSchedulerOperationKey(schedulerName, opID string) string {
	return fmt.Sprintf("operations:%s:%s", schedulerName, opID)
}

func (r *redisOperationStorage) buildSchedulerPendingOperationsKey(schedulerName string) string {
	return fmt.Sprintf("operations:%s:lists:pending", schedulerName)
}
