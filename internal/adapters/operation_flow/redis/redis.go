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

	"github.com/go-redis/redis/v8"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"go.uber.org/zap"
)

var _ ports.OperationFlow = (*redisOperationFlow)(nil)
var watchOperationCancelationRequestKey = "scheduler:operation_cancelation_requests"

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

func (r *redisOperationFlow) ListSchedulerPendingOperationIDs(ctx context.Context, schedulerName string) ([]string, error) {
	operationsIDs, err := r.client.LRange(ctx, r.buildSchedulerPendingOperationsKey(schedulerName), 0, -1).Result()
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to list pending operations for \"%s\"", schedulerName).WithError(err)
	}

	return operationsIDs, nil
}

func (r *redisOperationFlow) WatchOperationCancelationRequests(ctx context.Context) chan ports.OperationCancelationRequest {
	sub := r.client.Subscribe(ctx, watchOperationCancelationRequestKey)

	resultChan := make(chan ports.OperationCancelationRequest, 10000)

	go func() {
		defer sub.Close()

		for {
			select {
			case msg, ok := <-sub.Channel():
				if !ok {
					close(resultChan)
					return
				}

				var request ports.OperationCancelationRequest
				err := json.Unmarshal([]byte(msg.Payload), &request)
				if err != nil {
					zap.L().With(zap.Error(err)).Error("failed to parse operation cancelation request")
					continue
				}

				resultChan <- request
			case <-ctx.Done():
				close(resultChan)
				return
			}
		}
	}()

	return resultChan
}

func (r *redisOperationFlow) buildSchedulerPendingOperationsKey(schedulerName string) string {
	return fmt.Sprintf("pending_operations:%s", schedulerName)
}
