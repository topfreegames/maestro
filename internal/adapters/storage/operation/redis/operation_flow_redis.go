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

package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/topfreegames/maestro/internal/adapters/metrics"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/go-redis/redis/v8"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"go.uber.org/zap"
)

var _ ports.OperationFlow = (*redisOperationFlow)(nil)
var watchOperationCancellationRequestKey = "scheduler:operation_cancellation_requests"

const operationFlowStorageMetricLabel = "operation-flow-storage"

// redisOperationFlow adapter of the OperationStorage port. It stores
// the operations in lists to keep their creation/update order.
type redisOperationFlow struct {
	client *redis.Client
}

func NewRedisOperationFlow(client *redis.Client) *redisOperationFlow {
	return &redisOperationFlow{client}
}

// InsertOperationID pushes the operation ID to the scheduler pending
// operations list.
func (r *redisOperationFlow) InsertOperationID(ctx context.Context, schedulerName string, operationID string) (err error) {
	metrics.RunWithMetrics(operationFlowStorageMetricLabel, func() error {
		err = r.client.RPush(ctx, r.buildSchedulerPendingOperationsKey(schedulerName), operationID).Err()
		return err
	})
	if err != nil {
		return errors.NewErrUnexpected("failed to insert operation ID on redis").WithError(err)
	}

	return nil
}

// InsertPriorityOperationID inserts the operationID on the top of pending operations list.
func (r *redisOperationFlow) InsertPriorityOperationID(ctx context.Context, schedulerName string, operationID string) (err error) {
	metrics.RunWithMetrics(operationFlowStorageMetricLabel, func() error {
		err = r.client.LPush(ctx, r.buildSchedulerPendingOperationsKey(schedulerName), operationID).Err()
		return err
	})
	if err != nil {
		return errors.NewErrUnexpected("failed to insert operation ID on redis").WithError(err)
	}

	return nil
}

// NextOperationID fetches the next scheduler operation ID from the
// pending_operations list.
func (r *redisOperationFlow) NextOperationID(ctx context.Context, schedulerName string) (opID string, err error) {
	opID, err = r.fetchNextOpIDFromAuxiliaryQueue(ctx, schedulerName)

	if err == nil {
		return opID, nil
	}

	if err != nil && err != redis.Nil {
		return "", errors.NewErrUnexpected("failed to fetch next operation ID from auxiliary queue").WithError(err)
	}

	opID, err = r.waitAndFetchNextOpIDFromMainQueue(ctx, schedulerName)

	if err != nil {
		return "", errors.NewErrUnexpected("failed to fetch next operation ID from main queue").WithError(err)
	}

	return opID, nil
}

// RemoveNextOperation removes the next operation from the operation flow.
func (r *redisOperationFlow) RemoveNextOperation(ctx context.Context, schedulerName string) (err error) {
	metrics.RunWithMetrics(operationFlowStorageMetricLabel, func() error {
		_, err = r.client.LPop(ctx, r.buildSchedulerAuxiliaryPendingOperationsKey(schedulerName)).Result()

		return err
	})
	if err == redis.Nil {
		return errors.NewErrUnexpected("auxiliary pending operations queue is empty").WithError(err)
	}
	if err != nil {
		return errors.NewErrUnexpected("failed to pop from auxiliary pending operations list").WithError(err)
	}

	return nil
}

func (r *redisOperationFlow) ListSchedulerPendingOperationIDs(ctx context.Context, schedulerName string) (operationsIDs []string, err error) {
	operationsIDs, err = r.listPendingOperationsFromQueue(ctx, schedulerName, r.buildSchedulerPendingOperationsKey(schedulerName))

	if err != nil {
		return nil, errors.NewErrUnexpected("failed to list pending operations for \"%s\" from main queue", schedulerName).WithError(err)
	}

	operationsIDsFromAux, err := r.listPendingOperationsFromQueue(ctx, schedulerName, r.buildSchedulerAuxiliaryPendingOperationsKey(schedulerName))

	if err != nil && err != redis.Nil {
		return nil, errors.NewErrUnexpected("failed to list pending operations for \"%s\"", schedulerName).WithError(err)
	}

	return append(operationsIDsFromAux, operationsIDs...), nil
}

func (r *redisOperationFlow) EnqueueOperationCancellationRequest(ctx context.Context, request ports.OperationCancellationRequest) (err error) {
	requestAsString, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal operation cancellation request to string: %w", err)
	}

	metrics.RunWithMetrics(operationFlowStorageMetricLabel, func() error {
		err = r.client.Publish(ctx, watchOperationCancellationRequestKey, string(requestAsString)).Err()
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to publish operation cancellation request: %w", err)
	}

	return nil
}

func (r *redisOperationFlow) WatchOperationCancellationRequests(ctx context.Context) (resultChan chan ports.OperationCancellationRequest) {
	var sub *redis.PubSub
	metrics.RunWithMetrics(operationFlowStorageMetricLabel, func() error {
		sub = r.client.Subscribe(ctx, watchOperationCancellationRequestKey)
		return nil
	})

	resultChan = make(chan ports.OperationCancellationRequest, 100)

	go func() {
		defer sub.Close()
		defer close(resultChan)

		for {
			select {
			case msg, ok := <-sub.Channel():
				if !ok {
					return
				}

				var request ports.OperationCancellationRequest
				err := json.Unmarshal([]byte(msg.Payload), &request)
				if err != nil {
					zap.L().With(zap.Error(err)).Error("failed to parse operation cancellation request")
					continue
				}

				resultChan <- request
			case <-ctx.Done():
				return
			}
		}
	}()

	return resultChan
}

func (r *redisOperationFlow) buildSchedulerPendingOperationsKey(schedulerName string) string {
	return fmt.Sprintf("pending_operations:%s", schedulerName)
}

func (r *redisOperationFlow) buildSchedulerAuxiliaryPendingOperationsKey(schedulerName string) string {
	return fmt.Sprintf("pending_operations:%s:auxiliary", schedulerName)
}

func (r *redisOperationFlow) fetchNextOpIDFromAuxiliaryQueue(ctx context.Context, schedulerName string) (opID string, err error) {
	metrics.RunWithMetrics(operationFlowStorageMetricLabel, func() error {
		opID, err = r.client.LIndex(ctx, r.buildSchedulerAuxiliaryPendingOperationsKey(schedulerName), 0).Result()

		return err
	})
	return opID, err
}

func (r *redisOperationFlow) waitAndFetchNextOpIDFromMainQueue(ctx context.Context, schedulerName string) (opID string, err error) {
	metrics.RunWithMetrics(operationFlowStorageMetricLabel, func() error {
		opID, err = r.client.BLMove(ctx, r.buildSchedulerPendingOperationsKey(schedulerName), r.buildSchedulerAuxiliaryPendingOperationsKey(schedulerName), "LEFT", "RIGHT", 0).Result()

		return err
	})
	return opID, err
}

func (r *redisOperationFlow) listPendingOperationsFromQueue(ctx context.Context, schedulerName string, queueKey string) (operationsIDs []string, err error) {
	metrics.RunWithMetrics(operationFlowStorageMetricLabel, func() error {
		operationsIDs, err = r.client.LRange(ctx, queueKey, 0, -1).Result()
		return err
	})
	return operationsIDs, err

}
