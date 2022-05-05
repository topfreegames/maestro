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
func (r *redisOperationFlow) NextOperationID(ctx context.Context, schedulerName string) (opId string, err error) {
	var opIDs []string
	metrics.RunWithMetrics(operationFlowStorageMetricLabel, func() error {
		opIDs, err = r.client.BLPop(ctx, 0, r.buildSchedulerPendingOperationsKey(schedulerName)).Result()
		return err
	})
	if err != nil {
		return "", errors.NewErrUnexpected("failed to fetch next operation ID").WithError(err)
	}

	if len(opIDs) == 0 {
		return "", errors.NewErrNotFound("scheduler \"%s\" has no operations", schedulerName)
	}

	// Once redis finds any operation ID, it returns an array using:
	// - the first position to indicate the list name;
	// - the second position with the found value.
	opId = opIDs[1]
	return opId, nil
}

func (r *redisOperationFlow) ListSchedulerPendingOperationIDs(ctx context.Context, schedulerName string) (operationsIDs []string, err error) {
	metrics.RunWithMetrics(operationFlowStorageMetricLabel, func() error {
		operationsIDs, err = r.client.LRange(ctx, r.buildSchedulerPendingOperationsKey(schedulerName), 0, -1).Result()
		return err
	})
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to list pending operations for \"%s\"", schedulerName).WithError(err)
	}

	return operationsIDs, nil
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
