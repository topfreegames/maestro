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

	timeClock "github.com/topfreegames/maestro/internal/adapters/clock/time"
	operationleasestorage "github.com/topfreegames/maestro/internal/adapters/operation_lease/redis"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
)

func TestOperationLease(t *testing.T) {
	framework.WithClients(t, func(apiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		operationLeaseStorage := operationleasestorage.NewRedisOperationLeaseStorage(redisClient, timeClock.NewClock())

		t.Run("When the operation executes with success, then the lease keeps being renewed while it executes", func(t *testing.T) {
			t.Parallel()

			schedulerName, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				apiClient,
				kubeClient,
				[]string{"sh", "-c", "tail -f /dev/null"},
			)

			addRoomsRequest := &maestroApiV1.AddRoomsRequest{SchedulerName: schedulerName, Amount: 1}
			addRoomsResponse := &maestroApiV1.AddRoomsResponse{}
			err = apiClient.Do("POST", fmt.Sprintf("/schedulers/%s/add-rooms", schedulerName), addRoomsRequest, addRoomsResponse)
			require.NoError(t, err)
			addRoomsOpID := addRoomsResponse.OperationId
			var previousTtl = &time.Time{}
			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				// Only exit the loop when the operation finishes
				if len(listOperationsResponse.FinishedOperations) >= 2 {
					return true
				}

				// Make assertions while the operation is being executed
				if len(listOperationsResponse.ActiveOperations) >= 1 {
					require.Equal(t, "add_rooms", listOperationsResponse.ActiveOperations[0].DefinitionName)
					require.NotNil(t, listOperationsResponse.ActiveOperations[0].Lease)
					require.NotNil(t, listOperationsResponse.ActiveOperations[0].Lease.Ttl)
					renewedTtl, err := time.Parse(time.RFC3339, listOperationsResponse.ActiveOperations[0].Lease.Ttl)
					require.NoError(t, err)
					// Assert that the lease is not expired
					require.True(t, renewedTtl.After(time.Now()))
					// Assert that the lease is being renewed
					require.True(t, previousTtl.Before(renewedTtl))
					previousTtl = &renewedTtl
				}

				return false
				// 5 seconds is the operation lease ttl renew default factor (worker.yaml)
			}, 240*time.Second, 5*time.Second)

			// Asserting that the lease is deleted after the operation finishes
			_, err = operationLeaseStorage.FetchLeaseTTL(context.Background(), schedulerName, addRoomsOpID)
			require.Error(t, err, fmt.Sprintf("lease of scheduler \"%s\" and operationId \"%s\" does not exist", schedulerName, addRoomsOpID))
		})

		t.Run("When the operation is canceled, then the lease keeps being renewed while it executes", func(t *testing.T) {
			t.Parallel()

			schedulerName, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				apiClient,
				kubeClient,
				[]string{"sh", "-c", "tail -f /dev/null"},
			)

			addRoomsRequest := &maestroApiV1.AddRoomsRequest{SchedulerName: schedulerName, Amount: 1}
			addRoomsResponse := &maestroApiV1.AddRoomsResponse{}
			err = apiClient.Do("POST", fmt.Sprintf("/schedulers/%s/add-rooms", schedulerName), addRoomsRequest, addRoomsResponse)
			addRoomsOpID := addRoomsResponse.OperationId

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				// Don't make assertions while the operation hasn't started
				if len(listOperationsResponse.ActiveOperations) < 1 {
					return false
				}

				require.Equal(t, addRoomsOpID, listOperationsResponse.ActiveOperations[0].Id)
				require.NotNil(t, listOperationsResponse.ActiveOperations[0].Lease)
				require.NotNil(t, listOperationsResponse.ActiveOperations[0].Lease.Ttl)
				renewedTtl, err := time.Parse(time.RFC3339, listOperationsResponse.ActiveOperations[0].Lease.Ttl)
				require.NoError(t, err)
				// Assert that the lease is not expired
				require.True(t, renewedTtl.After(time.Now()))

				cancelRequest := &maestroApiV1.CancelOperationRequest{SchedulerName: schedulerName, OperationId: addRoomsOpID}
				cancelResponse := &maestroApiV1.CancelOperationResponse{}
				err = apiClient.Do("POST", fmt.Sprintf("/schedulers/%s/operations/%s/cancel", schedulerName, addRoomsOpID), cancelRequest, cancelResponse)
				require.NoError(t, err)

				return true
			}, 240*time.Second, 1*time.Second)

			// Asserting that the lease is deleted after the operation finishes
			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.FinishedOperations) < 2 {
					return false
				}

				lease, err := operationLeaseStorage.FetchLeaseTTL(context.Background(), schedulerName, addRoomsOpID)
				print(lease.String())
				require.Error(t, err, fmt.Sprintf("lease of scheduler \"%s\" and operationId \"%s\" does not exist", schedulerName, addRoomsOpID))

				return true
			}, 10*time.Second, 1*time.Second)
		})
	})

}