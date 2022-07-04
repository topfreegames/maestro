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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"testing"

	"github.com/go-redis/redis/v8"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	"k8s.io/client-go/kubernetes"
)

func TestDeleteScheduler(t *testing.T) {
	t.Parallel()

	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		t.Run("Should Succeed - check pods and namespace are deleted", func(t *testing.T) {
			t.Parallel()

			game := "delete-game"
			scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, game, kubeClient)
			require.NoError(t, err)

			deleteSchedulerRequest := &maestroApiV1.DeleteSchedulerRequest{SchedulerName: scheduler.Name}
			deleteSchedulerResponse := &maestroApiV1.DeleteSchedulerResponse{}
			err = managementApiClient.Do("DELETE", fmt.Sprintf("/schedulers/%s", scheduler.Name), deleteSchedulerRequest, deleteSchedulerResponse)
			assert.NoError(t, err)
			assert.NotNil(t, deleteSchedulerResponse.OperationId, scheduler.Name)

			// Assert every pod is deleted
			assert.Eventually(t, func() bool {
				pods, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
				assert.NoError(t, err)
				if len(pods.Items) == 0 {
					return true
				}
				return false
			}, time.Second*60, time.Second)

			// Assert namespace is deleted
			assert.Eventually(t, func() bool {
				ns, err := kubeClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
				assert.NoError(t, err)

				for _, n := range ns.Items {
					if scheduler.Name == n.GetName() {
						return false
					}
				}
				return true
			}, time.Second*60, time.Second)

			// Assert scheduler is deleted
			assert.Eventually(t, func() bool {
				getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: scheduler.Name}
				getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)

				if getSchedulerResponse.GetScheduler() == nil {
					return true
				}
				return false
			}, time.Second*60, time.Second*5)

			// Assert scheduler operations are deleted
			assert.Eventually(t, func() bool {
				getOperationsRequest := &maestroApiV1.ListOperationsRequest{SchedulerName: scheduler.Name}
				getOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/scheduler/%s/operations", scheduler.Name), getOperationsRequest, getOperationsResponse)

				if len(getOperationsResponse.GetFinishedOperations()) == 0 &&
					len(getOperationsResponse.GetActiveOperations()) == 0 &&
					len(getOperationsResponse.GetPendingOperations()) == 0 {
					return true
				}
				return false
			}, time.Second*60, time.Second*5)

		})

		t.Run("Should Fail - Scheduler not found", func(t *testing.T) {
			t.Parallel()

			schedulerName := "not-found-scheduler"
			deleteSchedulerRequest := &maestroApiV1.DeleteSchedulerRequest{SchedulerName: schedulerName}
			deleteSchedulerResponse := &maestroApiV1.DeleteSchedulerResponse{}
			err := managementApiClient.Do("DELETE", fmt.Sprintf("/schedulers/%s", schedulerName), deleteSchedulerRequest, deleteSchedulerResponse)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed with status 404")
		})
	})
}
