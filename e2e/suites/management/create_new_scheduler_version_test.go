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

	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/go-redis/redis/v8"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework"
	"k8s.io/client-go/kubernetes"
)

func TestCreateNewSchedulerVersion(t *testing.T) {
	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		t.Run("Should Succeed - When scheduler spec don't change it updates the scheduler with minor version and don't replace any pod", func(t *testing.T) {
			t.Parallel()

			schedulerName, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, kubeClient)

			podsBeforeUpdate, err := kubeClient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)

			// Update scheduler
			updateRequest := &maestroApiV1.NewSchedulerVersionRequest{
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
				PortRange: &maestroApiV1.PortRange{
					Start: 80,
					End:   8000,
				},
			}
			updateResponse := &maestroApiV1.NewSchedulerVersionResponse{}

			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s", schedulerName), updateRequest, updateResponse)
			require.NoError(t, err)
			require.NotNil(t, updateResponse.OperationId, schedulerName)

			waitForOperationToFinish(t, managementApiClient, schedulerName, "create_new_scheduler_version")
			waitForOperationToFinish(t, managementApiClient, schedulerName, "switch_active_version")

			podsAfterUpdate, err := kubeClient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, podsAfterUpdate.Items)

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: schedulerName}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", schedulerName), getSchedulerRequest, getSchedulerResponse)
			require.NoError(t, err)

			for i := 0; i < 2; i++ {
				require.Equal(t, podsAfterUpdate.Items[i].Name, podsBeforeUpdate.Items[i].Name)
			}
			require.Equal(t, "v1.2.0", getSchedulerResponse.Scheduler.Version)
		})

		//t.Run("Should Succeed - When scheduler spec changes it updates the scheduler with major version and replace all pods", func(t *testing.T) {
		//	t.Parallel()
		//
		//	schedulerName, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, kubeClient)
		//
		//	podsBeforeUpdate, err := kubeClient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
		//	require.NoError(t, err)
		//
		//	updateRequest := &maestroApiV1.NewSchedulerVersionRequest{
		//		Name:                   schedulerName,
		//		Game:                   "test",
		//		MaxSurge:               "10%",
		//		TerminationGracePeriod: 15,
		//		Containers: []*maestroApiV1.Container{
		//			{
		//				Name:  "example-update",
		//				Image: "alpine",
		//				Command: []string{"/bin/sh", "-c", "apk add curl && curl --request POST " +
		//					"$ROOMS_API_ADDRESS:9097/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
		//					"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && tail -f /dev/null"},
		//				ImagePullPolicy: "Always",
		//				Environment: []*maestroApiV1.ContainerEnvironment{
		//					{
		//						Name:  "ROOMS_API_ADDRESS",
		//						Value: maestro.RoomsApiServer.ContainerInternalAddress,
		//					},
		//				},
		//				Requests: &maestroApiV1.ContainerResources{
		//					Memory: "20Mi",
		//					Cpu:    "10m",
		//				},
		//				Limits: &maestroApiV1.ContainerResources{
		//					Memory: "20Mi",
		//					Cpu:    "10m",
		//				},
		//				Ports: []*maestroApiV1.ContainerPort{
		//					{
		//						Name:     "default",
		//						Protocol: "tcp",
		//						Port:     80,
		//					},
		//				},
		//			},
		//		},
		//		PortRange: &maestroApiV1.PortRange{
		//			Start: 80,
		//			End:   8000,
		//		},
		//	}
		//	updateResponse := &maestroApiV1.NewSchedulerVersionResponse{}
		//
		//	err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s", schedulerName), updateRequest, updateResponse)
		//	require.NoError(t, err)
		//	require.NotNil(t, updateResponse.OperationId, schedulerName)
		//
		//	waitForOperationToFinish(t, managementApiClient, schedulerName, "update_scheduler")
		//
		//	getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: schedulerName}
		//	getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}
		//
		//	err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", schedulerName), getSchedulerRequest, getSchedulerResponse)
		//	require.NoError(t, err)
		//
		//	require.Eventually(t, func() bool {
		//		podsAfterUpdate, err := kubeClient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
		//		require.NoError(t, err)
		//		require.NotEmpty(t, podsAfterUpdate.Items)
		//
		//		if len(podsAfterUpdate.Items) == 2 {
		//			return true
		//		}
		//
		//		return false
		//	}, 2*time.Minute, time.Second)
		//
		//	podsAfterUpdate, err := kubeClient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
		//	require.NoError(t, err)
		//	require.NotEmpty(t, podsAfterUpdate.Items)
		//
		//	for i := 0; i < 2; i++ {
		//		require.NotEqual(t, podsAfterUpdate.Items[i].Spec, podsBeforeUpdate.Items[i].Spec)
		//		require.Equal(t, "example-update", podsAfterUpdate.Items[i].Spec.Containers[0].Name)
		//	}
		//	require.Equal(t, "v2.0.0", getSchedulerResponse.Scheduler.Version)
		//})
		//
		//t.Run("Should Fail - When scheduler when sending invalid request to update endpoint it fails fast", func(t *testing.T) {
		//	t.Parallel()
		//
		//	schedulerName, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, kubeClient)
		//
		//	invalidUpdateRequest := &maestroApiV1.NewSchedulerVersionRequest{}
		//	updateResponse := &maestroApiV1.NewSchedulerVersionResponse{}
		//
		//	err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s", schedulerName), invalidUpdateRequest, updateResponse)
		//	require.Error(t, err)
		//	require.Contains(t, err.Error(), "failed with status 500")
		//
		//	getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: schedulerName}
		//	getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}
		//
		//	err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", schedulerName), getSchedulerRequest, getSchedulerResponse)
		//	require.NoError(t, err)
		//	require.Equal(t, "v1.1", getSchedulerResponse.Scheduler.Version)
		//})

		// TODO(guilhermecarvalho): when update failed flow is implemented, we should add extra test cases here

	})
}
