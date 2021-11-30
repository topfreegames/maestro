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

	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-redis/redis/v8"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework"
	"k8s.io/client-go/kubernetes"
)

func TestUpdateScheduler(t *testing.T) {
	framework.WithClients(t, func(apiClient *framework.APIClient, kubeclient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		t.Run("Should Succeed - When scheduler spec don't change it updates the scheduler with minor version and don't replace any pod", func(t *testing.T) {
			t.Parallel()

			schedulerName, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, apiClient, kubeclient)

			podsBeforeUpdate, err := kubeclient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)

			// Update scheduler
			updateRequest := &maestroApiV1.UpdateSchedulerRequest{
				Name:                   schedulerName,
				Game:                   "test",
				MaxSurge:               "10%",
				TerminationGracePeriod: 15,
				Containers: []*maestroApiV1.Container{
					{
						Name:  "example",
						Image: "alpine",
						Command: []string{"/bin/sh", "-c", "apk add curl && curl --request POST " +
							"$ROOMS_API_ADDRESS:9097/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
							"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && tail -f /dev/null"},
						ImagePullPolicy: "Always",
						Environment: []*maestroApiV1.ContainerEnvironment{
							{
								Name:  "ROOMS_API_ADDRESS",
								Value: maestro.RoomsApiServer.ContainerInternalAddress,
							},
						},
						Requests: &maestroApiV1.ContainerResources{
							Memory: "20Mi",
							Cpu:    "10m",
						},
						Limits: &maestroApiV1.ContainerResources{
							Memory: "20Mi",
							Cpu:    "10m",
						},
						Ports: []*maestroApiV1.ContainerPort{
							{
								Name:     "default",
								Protocol: "tcp",
								Port:     80,
							},
						},
					},
				},
			}
			updateResponse := &maestroApiV1.UpdateSchedulerResponse{}

			err = apiClient.Do("PUT", fmt.Sprintf("/schedulers/%s", schedulerName), updateRequest, updateResponse)
			require.NoError(t, err)
			require.NotNil(t, updateResponse.OperationId, schedulerName)

			waitForOperationToFinish(t, apiClient, schedulerName, "update_scheduler")

			podsAfterUpdate, err := kubeclient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, podsAfterUpdate.Items)

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: schedulerName}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}
			err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s", schedulerName), getSchedulerRequest, getSchedulerResponse)
			require.NoError(t, err)

			for i := 0; i < 2; i++ {
				require.Equal(t, podsAfterUpdate.Items[i].Name, podsBeforeUpdate.Items[i].Name)
			}
			require.Equal(t, "v1.2.0", getSchedulerResponse.Scheduler.Version)
		})

		t.Run("Should Succeed - When scheduler spec changes it updates the scheduler with major version and replace all pods", func(t *testing.T) {
			t.Parallel()

			schedulerName, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, apiClient, kubeclient)

			podsBeforeUpdate, err := kubeclient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)

			updateRequest := &maestroApiV1.UpdateSchedulerRequest{
				Name:                   schedulerName,
				Game:                   "test",
				MaxSurge:               "10%",
				TerminationGracePeriod: 15,
				Containers: []*maestroApiV1.Container{
					{
						Name:  "example-update",
						Image: "alpine",
						Command: []string{"/bin/sh", "-c", "apk add curl && curl --request POST " +
							"$ROOMS_API_ADDRESS:9097/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
							"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && tail -f /dev/null"},
						ImagePullPolicy: "Always",
						Environment: []*maestroApiV1.ContainerEnvironment{
							{
								Name:  "ROOMS_API_ADDRESS",
								Value: maestro.RoomsApiServer.ContainerInternalAddress,
							},
						},
						Requests: &maestroApiV1.ContainerResources{
							Memory: "20Mi",
							Cpu:    "10m",
						},
						Limits: &maestroApiV1.ContainerResources{
							Memory: "20Mi",
							Cpu:    "10m",
						},
						Ports: []*maestroApiV1.ContainerPort{
							{
								Name:     "default",
								Protocol: "tcp",
								Port:     80,
							},
						},
					},
				},
			}
			updateResponse := &maestroApiV1.UpdateSchedulerResponse{}

			err = apiClient.Do("PUT", fmt.Sprintf("/schedulers/%s", schedulerName), updateRequest, updateResponse)
			require.NoError(t, err)
			require.NotNil(t, updateResponse.OperationId, schedulerName)

			waitForOperationToFinish(t, apiClient, schedulerName, "update_scheduler")

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: schedulerName}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s", schedulerName), getSchedulerRequest, getSchedulerResponse)
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				podsAfterUpdate, err := kubeclient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				require.NotEmpty(t, podsAfterUpdate.Items)

				if len(podsAfterUpdate.Items) == 2 {
					return true
				}

				return false
			}, 2*time.Minute, time.Second)

			podsAfterUpdate, err := kubeclient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, podsAfterUpdate.Items)

			for i := 0; i < 2; i++ {
				require.NotEqual(t, podsAfterUpdate.Items[i].Spec, podsBeforeUpdate.Items[i].Spec)
				require.Equal(t, "example-update", podsAfterUpdate.Items[i].Spec.Containers[0].Name)
			}
			require.Equal(t, "v2.0.0", getSchedulerResponse.Scheduler.Version)
		})

		t.Run("Should Fail - When scheduler when sending invalid request to update endpoint it fails fast", func(t *testing.T) {
			t.Parallel()

			schedulerName, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, apiClient, kubeclient)

			invalidUpdateRequest := &maestroApiV1.UpdateSchedulerRequest{}
			updateResponse := &maestroApiV1.UpdateSchedulerResponse{}

			err = apiClient.Do("PUT", fmt.Sprintf("/schedulers/%s", schedulerName), invalidUpdateRequest, updateResponse)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed with status 500")

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: schedulerName}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s", schedulerName), getSchedulerRequest, getSchedulerResponse)
			require.NoError(t, err)
			require.Equal(t, "v1.1", getSchedulerResponse.Scheduler.Version)
		})

		// TODO(guilhermecarvalho): when update failed flow is implemented, we should add extra test cases here

	})
}

func createSchedulerWithRoomsAndWaitForIt(t *testing.T, maestro *maestro.MaestroInstance, apiClient *framework.APIClient, kubeclient kubernetes.Interface) (string, error) {
	// Create scheduler
	schedulerName, err := createSchedulerAndWaitForIt(
		t,
		maestro,
		apiClient,
		kubeclient,
		[]string{"/bin/sh", "-c", "apk add curl && curl --request POST " +
			"$ROOMS_API_ADDRESS:9097/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
			"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && tail -f /dev/null"},
	)

	// Add rooms to the created scheduler
	// TODO(guilhermecarvalho): when autoscaling is implemented, this part of the test can be deleted
	addRoomsRequest := &maestroApiV1.AddRoomsRequest{SchedulerName: schedulerName, Amount: 2}
	addRoomsResponse := &maestroApiV1.AddRoomsResponse{}
	err = apiClient.Do("POST", fmt.Sprintf("/schedulers/%s/add-rooms", schedulerName), addRoomsRequest, addRoomsResponse)
	require.NoError(t, err)

	waitForOperationToFinish(t, apiClient, schedulerName, "add_rooms")
	return schedulerName, err
}

func waitForOperationToFinish(t *testing.T, apiClient *framework.APIClient, schedulerName, operation string) {
	require.Eventually(t, func() bool {
		listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
		listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
		err := apiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
		require.NoError(t, err)

		if len(listOperationsResponse.FinishedOperations) > 1 && listOperationsResponse.FinishedOperations[0].DefinitionName == operation {
			return true
		}

		return false
	}, 2*time.Minute, time.Second)
}
