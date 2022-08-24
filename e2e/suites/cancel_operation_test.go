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

package suites

import (
	"context"
	"fmt"
	"testing"
	"time"

	operation2 "github.com/topfreegames/maestro/internal/adapters/flow/redis/operation"
	operationredis "github.com/topfreegames/maestro/internal/adapters/storage/redis/operation"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	timeClock "github.com/topfreegames/maestro/internal/adapters/clock/time"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
)

func TestCancelOperation(t *testing.T) {
	t.Parallel()

	duration, err := time.ParseDuration("24h")
	require.NoError(t, err)

	operationsTTLMap := map[operationredis.Definition]time.Duration{
		healthcontroller.OperationName: duration,
	}

	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		operationStorage := operationredis.NewRedisOperationStorage(redisClient, timeClock.NewClock(), operationsTTLMap)
		operationFlow := operation2.NewRedisOperationFlow(redisClient)

		t.Run("cancel pending and in-progress operations successfully", func(t *testing.T) {
			ctx := context.Background()
			scheduler, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				managementApiClient,
				kubeClient,
				"test",
				[]string{"sh", "-c", "while true; do sleep 100; done"},
			)

			firstSlowOp := createTestOperation(ctx, t, operationStorage, operationFlow, scheduler.Name, 100000)
			secondSlowOp := createTestOperation(ctx, t, operationStorage, operationFlow, scheduler.Name, 100000)

			inProgressStatus, err := operation.StatusInProgress.String()
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				getOperationRequest := &maestroApiV1.GetOperationRequest{}
				getOperationResponse := &maestroApiV1.GetOperationResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations/%s", scheduler.Name, firstSlowOp.ID), getOperationRequest, getOperationResponse)
				require.NoError(t, err)

				if getOperationResponse.Operation.Status != inProgressStatus {
					return false
				}

				return true
			}, 240*time.Second, 10*time.Millisecond)

			secondOpCancelRequest := &maestroApiV1.CancelOperationRequest{SchedulerName: scheduler.Name, OperationId: secondSlowOp.ID}
			secondOpCancelResponse := &maestroApiV1.CancelOperationResponse{}
			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s/operations/%s/cancel", scheduler.Name, secondSlowOp.ID), secondOpCancelRequest, secondOpCancelResponse)
			require.NoError(t, err)

			firstOpCancelRequest := &maestroApiV1.CancelOperationRequest{SchedulerName: scheduler.Name, OperationId: firstSlowOp.ID}
			firstOpCancelResponse := &maestroApiV1.CancelOperationResponse{}
			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s/operations/%s/cancel", scheduler.Name, firstSlowOp.ID), firstOpCancelRequest, firstOpCancelResponse)
			require.NoError(t, err)

			canceledStatus, err := operation.StatusCanceled.String()
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				firstOperationRequest := &maestroApiV1.GetOperationRequest{}
				firstOperationResponse := &maestroApiV1.GetOperationResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations/%s", scheduler.Name, firstSlowOp.ID), firstOperationRequest, firstOperationResponse)
				require.NoError(t, err)
				if firstOperationResponse.Operation.Status != canceledStatus {
					return false
				}

				return true
			}, 1*time.Minute, 10*time.Millisecond)

			require.Eventually(t, func() bool {
				secondOperationRequest := &maestroApiV1.GetOperationRequest{}
				secondOperationResponse := &maestroApiV1.GetOperationResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations/%s", scheduler.Name, secondSlowOp.ID), secondOperationRequest, secondOperationResponse)
				require.NoError(t, err)

				if secondOperationResponse.Operation.Status != canceledStatus {
					return false
				}

				return true
			}, 1*time.Minute, 10*time.Millisecond)
		})

		t.Run("error when try to cancel operations on final state", func(t *testing.T) {
			scheduler, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				managementApiClient,
				kubeClient,
				"test",
				[]string{"sh", "-c", "while true; do sleep 1; done"},
			)

			listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
			listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations?stage=final", scheduler.Name), listOperationsRequest, listOperationsResponse)
			require.NoError(t, err)

			finishedOpId := listOperationsResponse.Operations[0].Id

			finishedOpCancelRequest := &maestroApiV1.CancelOperationRequest{SchedulerName: scheduler.Name, OperationId: finishedOpId}
			finishedOpCancelResponse := &maestroApiV1.CancelOperationResponse{}
			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s/operations/%s/cancel", scheduler.Name, finishedOpId), finishedOpCancelRequest, finishedOpCancelResponse)
			require.Error(t, err)
			require.Contains(t, err.Error(), "status 409")
		})
	})
}
