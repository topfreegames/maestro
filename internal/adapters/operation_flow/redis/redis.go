package redis

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
)

var _ ports.OperationFlow = (*redisOperationFlow)(nil)

// redisOperationFlow adapter of the OperationStorage port. It store store
// the operations in lists to keep their creation/update order.
type redisOperationFlow struct {
	client *redis.Client
}

func NewRedisOperationFlow(client *redis.Client) *redisOperationFlow {
	return &redisOperationFlow{client}
}

// InsertOperationID pushes the operation ID to the scheduler pending
// operations list.
func (r *redisOperationFlow) InsertOperationID(ctx context.Context, schedulerName string, operationID string) error {
	err := r.client.RPush(ctx, r.buildSchedulerPendingOperationsKey(schedulerName), operationID).Err()
	if err != nil {
		return errors.NewErrUnexpected("failed to insert operation ID on redis").WithError(err)
	}

	return nil
}

// NextOperationID fetches the next scheduler operation ID from the
// pending_operations list.
func (r *redisOperationFlow) NextOperationID(ctx context.Context, schedulerName string) (string, error) {
	opIDs, err := r.client.BLPop(ctx, 0, r.buildSchedulerPendingOperationsKey(schedulerName)).Result()
	if err != nil {
		return "", errors.NewErrUnexpected("failed to fetch next operation ID").WithError(err)
	}

	if len(opIDs) == 0 {
		return "", errors.NewErrNotFound("scheduler \"%s\" has no operations", schedulerName)
	}

	// Once redis finds any operation ID, it returns an array using:
	// - the first position to indicate the list name;
	// - the second position with the found value.
	return opIDs[1], nil
}

func (r *redisOperationFlow) buildSchedulerPendingOperationsKey(schedulerName string) string {
	return fmt.Sprintf("pending_operations:%s", schedulerName)
}
