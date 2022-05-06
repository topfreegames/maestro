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

//go:build integration
// +build integration

package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/test"
)

func TestInsertOperationID(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		flow := NewRedisOperationFlow(client)
		schedulerName := "test-scheduler"
		expectedOperationID := "some-op-id"

		err := flow.InsertOperationID(context.Background(), schedulerName, expectedOperationID)
		require.NoError(t, err)

		opID, err := client.LPop(context.Background(), flow.buildSchedulerPendingOperationsKey(schedulerName)).Result()
		require.NoError(t, err)
		require.Equal(t, expectedOperationID, opID)
	})

	t.Run("fails on redis", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		flow := NewRedisOperationFlow(client)

		// "drop" redis connection
		client.Close()

		err := flow.InsertOperationID(context.Background(), "", "")
		require.ErrorIs(t, errors.ErrUnexpected, err)
	})
}

func TestInsertPriorityOperationID(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		flow := NewRedisOperationFlow(client)

		schedulerName := "test-scheduler"

		lowPriorityOperationID := "low-priority-operation"
		highPriorityOperationID := "high-priority-operation"

		err := flow.InsertPriorityOperationID(context.Background(), schedulerName, lowPriorityOperationID)
		require.NoError(t, err)

		err = flow.InsertPriorityOperationID(context.Background(), schedulerName, highPriorityOperationID)
		require.NoError(t, err)

		opID, err := client.LPop(context.Background(), flow.buildSchedulerPendingOperationsKey(schedulerName)).Result()
		require.NoError(t, err)
		require.Equal(t, highPriorityOperationID, opID)
	})

	t.Run("fails on redis", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		flow := NewRedisOperationFlow(client)

		// "drop" redis connection
		client.Close()

		err := flow.InsertPriorityOperationID(context.Background(), "", "")
		require.ErrorIs(t, errors.ErrUnexpected, err)
	})
}

func TestNextOperationID(t *testing.T) {
	t.Run("successfully receives the operation ID", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		flow := NewRedisOperationFlow(client)

		schedulerName := "test-scheduler"
		expectedOperationID := "some-op-id"

		err := flow.InsertOperationID(context.Background(), schedulerName, expectedOperationID)
		require.NoError(t, err)

		opID, err := flow.NextOperationID(context.Background(), schedulerName)
		require.NoError(t, err)
		require.Equal(t, expectedOperationID, opID)
	})

	t.Run("failed with context canceled", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		flow := NewRedisOperationFlow(client)

		ctx, cancel := context.WithCancel(context.Background())
		nextWait := make(chan error)

		go func() {
			_, err := flow.NextOperationID(ctx, "some-random-scheduler")
			nextWait <- err
		}()

		cancel()

		require.Eventually(t, func() bool {
			select {
			case err := <-nextWait:
				require.Error(t, err)
				return true
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("failed redis connection", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		storage := NewRedisOperationFlow(client)

		nextWait := make(chan error)
		go func() {
			_, err := storage.NextOperationID(context.Background(), "some-random-scheduler")
			nextWait <- err
		}()

		client.Close()

		require.Eventually(t, func() bool {
			select {
			case err := <-nextWait:
				require.Error(t, err)
				return true
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)
	})
}

func TestEnqueueOperationCancellationRequest(t *testing.T) {
	schedulerName := uuid.New().String()
	operationID := uuid.New().String()

	t.Run("successfully publishes the request to cancel", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		flow := NewRedisOperationFlow(client)
		ctx := context.Background()

		cancelChan := flow.WatchOperationCancellationRequests(ctx)
		err := flow.EnqueueOperationCancellationRequest(ctx, ports.OperationCancellationRequest{
			SchedulerName: schedulerName,
			OperationID:   operationID,
		})

		require.NoError(t, err)

		require.Eventually(t, func() bool {
			select {
			case request := <-cancelChan:
				require.Equal(t, request.SchedulerName, schedulerName)
				require.Equal(t, request.OperationID, operationID)
				return true
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)
	})
}

func TestWatchOperationCancellationRequests(t *testing.T) {
	schedulerName := uuid.New().String()
	operationID := uuid.New().String()

	t.Run("successfully receives the scheduler name and operation ID to cancel", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		flow := NewRedisOperationFlow(client)
		ctx, ctxCancelFn := context.WithCancel(context.Background())

		cancelChan := flow.WatchOperationCancellationRequests(ctx)

		requestAsString, _ := json.Marshal(ports.OperationCancellationRequest{
			SchedulerName: schedulerName,
			OperationID:   operationID,
		})

		err := client.Publish(ctx, watchOperationCancellationRequestKey, string(requestAsString)).Err()
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			select {
			case request := <-cancelChan:
				require.Equal(t, request.SchedulerName, schedulerName)
				require.Equal(t, request.OperationID, operationID)
				return true
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)

		ctxCancelFn()
	})

	t.Run("when parent context is canceled, stops to watch requests", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		flow := NewRedisOperationFlow(client)
		ctx, ctxCancelFn := context.WithCancel(context.Background())

		cancelChan := flow.WatchOperationCancellationRequests(ctx)
		ctxCancelFn()

		require.Eventually(t, func() bool {
			select {
			case _, ok := <-cancelChan:
				return !ok
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("when redis connection fails, stops to watch requests", func(t *testing.T) {
		client := test.GetRedisConnection(t, redisAddress)
		flow := NewRedisOperationFlow(client)
		ctx := context.Background()

		cancelChan := flow.WatchOperationCancellationRequests(ctx)

		client.Close()

		require.Eventually(t, func() bool {
			select {
			case _, ok := <-cancelChan:
				return !ok
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)
	})
}

func TestListSchedulerPendingOperationIDs(t *testing.T) {
	type args struct {
		ctx           context.Context
		schedulerName string
	}
	type environmentSetup struct {
		prepareDatabase  func(schedulerName string, client *redis.Client)
		forceClientError bool
	}
	tests := []struct {
		name              string
		args              args
		environmentSetup  environmentSetup
		wantOperationsIDs []string
		wantErr           error
	}{
		{"return no error and the list of pending operations from main queue",
			args{
				ctx:           context.Background(),
				schedulerName: "test-scheduler",
			},
			environmentSetup{
				prepareDatabase: func(schedulerName string, client *redis.Client) {
					err := client.RPush(context.Background(), fmt.Sprintf("pending_operations:%s", schedulerName), "some-op-id").Err()
					require.NoError(t, err)
				},
				forceClientError: false,
			},
			[]string{"some-op-id"},
			nil,
		},
		{"return no error and the list of pending operations from aux queue",
			args{
				ctx:           context.Background(),
				schedulerName: "test-scheduler",
			},
			environmentSetup{
				prepareDatabase: func(schedulerName string, client *redis.Client) {
					err := client.RPush(context.Background(), fmt.Sprintf("pending_operations:%s", schedulerName), "some-op-id1").Err()
					require.NoError(t, err)
					err = client.RPush(context.Background(), fmt.Sprintf("pending_operations:%s:auxiliary", schedulerName), "some-op-id2").Err()
					require.NoError(t, err)
				},
				forceClientError: false,
			},
			[]string{"some-op-id1", "some-op-id2"},
			nil,
		},
		{"return error when some error occurs with redis client",
			args{
				ctx:           context.Background(),
				schedulerName: "test-scheduler",
			},
			environmentSetup{
				prepareDatabase:  func(schedulerName string, client *redis.Client) {},
				forceClientError: true,
			},
			[]string{"some-op-id1", "some-op-id2"},
			errors.NewErrUnexpected("failed to list pending operations for \"test-scheduler\": redis: client is closed"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)
			flow := NewRedisOperationFlow(client)

			ctx := tt.args.ctx
			schedulerName := tt.args.schedulerName

			tt.environmentSetup.prepareDatabase(schedulerName, client)

			if tt.environmentSetup.forceClientError {
				go func() {
					_, err := flow.ListSchedulerPendingOperationIDs(ctx, schedulerName)
					assert.EqualError(t, err, tt.wantErr.Error())
				}()

				client.Close()
			} else {
				gotOperationsIDs, err := flow.ListSchedulerPendingOperationIDs(ctx, schedulerName)
				if tt.wantErr != nil {
					assert.EqualError(t, err, tt.wantErr.Error())
				}
				assert.Equal(t, tt.wantOperationsIDs, gotOperationsIDs)
			}
		})
	}
}

func TestRemoveOperation(t *testing.T) {

	type args struct {
		ctx           context.Context
		schedulerName string
	}
	type environmentSetup struct {
		numberOfOps      int
		forceClientError bool
	}
	tests := []struct {
		name             string
		args             args
		environmentSetup environmentSetup
	}{
		{"return no error and pops the operation from de auxiliary operations queue if there is some", args{
			ctx:           context.Background(),
			schedulerName: "test-scheduler",
		},
			environmentSetup{numberOfOps: 1, forceClientError: false},
		},
		{"return no error and does nothing if the auxiliary operations queue is empty", args{
			ctx:           context.Background(),
			schedulerName: "test-scheduler",
		},
			environmentSetup{numberOfOps: 0, forceClientError: false},
		},

		{"return error when some error occurs with redis client", args{
			ctx:           context.Background(),
			schedulerName: "test-scheduler",
		},
			environmentSetup{numberOfOps: 0, forceClientError: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := test.GetRedisConnection(t, redisAddress)

			flow := &redisOperationFlow{
				client: client,
			}

			schedulerName := tt.args.schedulerName
			expectedOperationID := "some-op-id"

			for i := 0; i < tt.environmentSetup.numberOfOps; i++ {
				err := client.RPush(context.Background(), flow.buildSchedulerAuxiliaryPendingOperationsKey(schedulerName), expectedOperationID).Err()
				require.NoError(t, err)
			}

			if tt.environmentSetup.forceClientError {
				go func() {
					err := flow.RemoveNextOperation(tt.args.ctx, tt.args.schedulerName)
					assert.Error(t, err)
				}()

				client.Close()
			} else {
				err := flow.RemoveNextOperation(tt.args.ctx, tt.args.schedulerName)
				assert.NoError(t, err)

				opIdBufferedQueue, err := client.LIndex(context.Background(), flow.buildSchedulerAuxiliaryPendingOperationsKey(schedulerName), 0).Result()
				require.EqualError(t, err, "redis: nil")
				require.Equal(t, "", opIdBufferedQueue)
			}
		})
	}
}
