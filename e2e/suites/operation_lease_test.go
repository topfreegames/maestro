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
	operation3 "github.com/topfreegames/maestro/internal/adapters/lease/redis/operation"
	operationredis "github.com/topfreegames/maestro/internal/adapters/storage/redis/operation"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"
	operationsproviders "github.com/topfreegames/maestro/internal/core/operations/providers"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework/maestro"
	timeClock "github.com/topfreegames/maestro/internal/adapters/clock/time"

	"github.com/topfreegames/maestro/e2e/framework"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
)

func TestOperationLease(t *testing.T) {
	t.Parallel()

	canceledStatus, err := operation.StatusCanceled.String()
	require.NoError(t, err)

	duration, err := time.ParseDuration("24h")
	require.NoError(t, err)

	operationsTTLMap := map[operationredis.Definition]time.Duration{
		healthcontroller.OperationName: duration,
	}

	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		operationStorage := operationredis.NewRedisOperationStorage(redisClient, timeClock.NewClock(), operationsTTLMap, operationsproviders.ProvideDefinitionConstructors())
		operationFlow := operation2.NewRedisOperationFlow(redisClient)
		operationLeaseStorage := operation3.NewRedisOperationLeaseStorage(redisClient, timeClock.NewClock())

		t.Run("When the operation executes with success, then the lease keeps being renewed while it executes", func(t *testing.T) {
			t.Parallel()

			scheduler, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				managementApiClient,
				kubeClient,
				"test",
				[]string{"sh", "-c", "while true; do sleep 1; done"},
			)

			slowOp := createTestOperation(context.Background(), t, operationStorage, operationFlow, scheduler.Name, 1)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations?stage=active", scheduler.Name), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				activeOperation := listOperationsResponse.Operations[0]
				if activeOperation.Id != slowOp.ID {
					return false
				}

				renewedTtl, err := time.Parse(time.RFC3339, activeOperation.Lease.Ttl)
				require.NoError(t, err)
				// Assert that the lease is not expired
				require.True(t, renewedTtl.After(time.Now()))

				return true
				// 5 seconds is the operation lease ttl renew default factor (worker.yaml)
			}, 5*time.Second, 10*time.Millisecond)

			// Wait operation finished
			time.Sleep(2 * time.Second)

			// Asserting that the lease is deleted after the operation finishes
			_, err = operationLeaseStorage.FetchLeaseTTL(context.Background(), scheduler.Name, slowOp.ID)
			require.Error(t, err, fmt.Sprintf("lease of scheduler \"%s\" and operationId \"%s\" does not exist", scheduler.Name, slowOp.ID))
		})

		t.Run("When the operation is canceled, then the lease keeps being renewed while it executes", func(t *testing.T) {
			t.Parallel()

			scheduler, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				managementApiClient,
				kubeClient,
				"test",
				[]string{"sh", "-c", "while true; do sleep 1; done"},
			)

			slowOp := createTestOperation(context.Background(), t, operationStorage, operationFlow, scheduler.Name, 100000)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations?stage=active", scheduler.Name), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.Operations) <= 0 {
					return false
				}

				activeOperation := listOperationsResponse.Operations[0]
				if activeOperation.Id != slowOp.ID {
					return false
				}

				renewedTtl, err := time.Parse(time.RFC3339, activeOperation.Lease.Ttl)
				require.NoError(t, err)
				// Assert that the lease is not expired
				require.True(t, renewedTtl.After(time.Now()))

				return true
				// 5 seconds is the operation lease ttl renew default factor (worker.yaml)
			}, 100*time.Second, 10*time.Millisecond)

			cancelRequest := &maestroApiV1.CancelOperationRequest{SchedulerName: scheduler.Name, OperationId: slowOp.ID}
			cancelResponse := &maestroApiV1.CancelOperationResponse{}
			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s/operations/%s/cancel", scheduler.Name, slowOp.ID), cancelRequest, cancelResponse)
			require.NoError(t, err)

			// Asserting that the lease is deleted after the operation finishes
			require.Eventually(t, func() bool {
				getOperationRequest := &maestroApiV1.GetOperationRequest{}
				getOperationResponse := &maestroApiV1.GetOperationResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations/%s", scheduler.Name, slowOp.ID), getOperationRequest, getOperationResponse)
				require.NoError(t, err)

				if getOperationResponse.Operation.Status != canceledStatus {
					return false
				}

				return true
			}, 100*time.Second, 10*time.Millisecond)

			_, err = operationLeaseStorage.FetchLeaseTTL(context.Background(), scheduler.Name, slowOp.ID)
			require.Error(t, err, fmt.Sprintf("lease of scheduler \"%s\" and operationId \"%s\" does not exist", scheduler.Name, slowOp.ID))
		})
	})
}
