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

package management

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	timeClock "github.com/topfreegames/maestro/internal/adapters/clock/time"
	operationFlowRedis "github.com/topfreegames/maestro/internal/adapters/operation_flow/redis"
	operationStorageRedis "github.com/topfreegames/maestro/internal/adapters/operation_storage/redis"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations/test_operation"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
)

func TestCancelOperation(t *testing.T) {
	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		operationStorage := operationStorageRedis.NewRedisOperationStorage(redisClient, timeClock.NewClock())
		operationFlow := operationFlowRedis.NewRedisOperationFlow(redisClient)

		t.Run("cancel pending and in-progress operations successfully", func(t *testing.T) {
			ctx := context.Background()
			scheduler, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				managementApiClient,
				kubeClient,
				[]string{"sh", "-c", "tail -f /dev/null"},
			)

			firstSlowOp := createTestOperation(ctx, t, operationStorage, operationFlow, scheduler.Name, 100000)
			secondSlowOp := createTestOperation(ctx, t, operationStorage, operationFlow, scheduler.Name, 100000)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", scheduler.Name), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.PendingOperations) < 1 || len(listOperationsResponse.ActiveOperations) < 1 {
					return false
				}

				require.Equal(t, firstSlowOp.ID, listOperationsResponse.ActiveOperations[0].Id)
				require.Equal(t, secondSlowOp.ID, listOperationsResponse.PendingOperations[0].Id)
				return true
			}, 240*time.Second, time.Second)

			secondOpCancelRequest := &maestroApiV1.CancelOperationRequest{SchedulerName: scheduler.Name, OperationId: secondSlowOp.ID}
			secondOpCancelResponse := &maestroApiV1.CancelOperationResponse{}
			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s/operations/%s/cancel", scheduler.Name, secondSlowOp.ID), secondOpCancelRequest, secondOpCancelResponse)
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", scheduler.Name), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.FinishedOperations) < 2 {
					return false
				}

				require.Equal(t, secondSlowOp.ID, listOperationsResponse.FinishedOperations[0].Id)
				require.Equal(t, firstSlowOp.ID, listOperationsResponse.ActiveOperations[0].Id)
				return true
			}, 240*time.Second, time.Second)

			firstOpCancelRequest := &maestroApiV1.CancelOperationRequest{SchedulerName: scheduler.Name, OperationId: firstSlowOp.ID}
			firstOpCancelResponse := &maestroApiV1.CancelOperationResponse{}
			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s/operations/%s/cancel", scheduler.Name, firstSlowOp.ID), firstOpCancelRequest, firstOpCancelResponse)
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", scheduler.Name), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.FinishedOperations) < 3 {
					return false
				}

				require.Equal(t, secondSlowOp.ID, listOperationsResponse.FinishedOperations[0].Id)
				require.Equal(t, firstSlowOp.ID, listOperationsResponse.FinishedOperations[1].Id)
				return true
			}, 240*time.Second, time.Second)
		})
	})
}

func createTestOperation(ctx context.Context, t *testing.T, operationStorage ports.OperationStorage, operationFlow ports.OperationFlow, schedulerName string, sleepSeconds int) *operation.Operation {

	definition := test_operation.TestOperationDefinition{
		SleepSeconds: sleepSeconds,
	}

	op := &operation.Operation{
		ID:             uuid.NewString(),
		Status:         operation.StatusPending,
		DefinitionName: definition.Name(),
		SchedulerName:  schedulerName,
		CreatedAt:      time.Now(),
	}

	err := operationStorage.CreateOperation(ctx, op, definition.Marshal())
	require.NoError(t, err)
	err = operationFlow.InsertOperationID(ctx, op.SchedulerName, op.ID)
	require.NoError(t, err)

	return op
}
