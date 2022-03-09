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
	_struct "github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/protobuf/types/known/structpb"
	"time"

	"testing"

	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	maestrov1 "github.com/topfreegames/maestro/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-redis/redis/v8"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework"
	"k8s.io/client-go/kubernetes"
)

func TestCreateNewSchedulerVersion(t *testing.T) {
	game := "create-new-scheduler-version-game"
	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		t.Run("Should Succeed - create minor version, no pods replaces", func(t *testing.T) {
			t.Parallel()

			scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, game, kubeClient)

			podsBeforeUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)

			// Update scheduler
			updateRequest := &maestroApiV1.NewSchedulerVersionRequest{
				Name:     scheduler.Name,
				Game:     "test",
				MaxSurge: "10%",
				Spec: &maestrov1.Spec{
					TerminationGracePeriod: 15,
					Containers: []*maestroApiV1.Container{
						{
							Name:  "example",
							Image: "alpine:3.15.0",
							Command: []string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request POST " +
								"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
								"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 1; done"},
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
				},
				PortRange: &maestroApiV1.PortRange{
					Start: 80,
					End:   8000,
				},
			}
			updateResponse := &maestroApiV1.NewSchedulerVersionResponse{}

			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s", scheduler.Name), updateRequest, updateResponse)
			require.NoError(t, err)
			require.NotNil(t, updateResponse.OperationId, scheduler.Name)

			waitForOperationToFinish(t, managementApiClient, scheduler.Name, "create_new_scheduler_version")
			waitForOperationToFinish(t, managementApiClient, scheduler.Name, "switch_active_version")

			podsAfterUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, podsAfterUpdate.Items)

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: scheduler.Name}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)
			require.NoError(t, err)

			// Don't replace pods since is a minor change
			for i := 0; i < 2; i++ {
				require.Equal(t, podsAfterUpdate.Items[i].Name, podsBeforeUpdate.Items[i].Name)
			}

			// Switches to version v1.1.0
			require.Equal(t, "v1.1.0", getSchedulerResponse.Scheduler.Spec.Version)
		})

		t.Run("Should Succeed - create major change, all pods are changed", func(t *testing.T) {
			t.Parallel()

			scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, game, kubeClient)

			podsBeforeUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)

			updateRequest := &maestroApiV1.NewSchedulerVersionRequest{
				Name:     scheduler.Name,
				Game:     "test",
				MaxSurge: "10%",
				Spec: &maestrov1.Spec{
					TerminationGracePeriod: 15,
					Containers: []*maestroApiV1.Container{
						{
							Name:  "example-update",
							Image: "alpine:3.15.0",
							Command: []string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request POST " +
								"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
								"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 1; done"},
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
				},
				PortRange: &maestroApiV1.PortRange{
					Start: 80,
					End:   8000,
				},
			}
			updateResponse := &maestroApiV1.NewSchedulerVersionResponse{}

			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s", scheduler.Name), updateRequest, updateResponse)
			require.NoError(t, err)
			require.NotNil(t, updateResponse.OperationId, scheduler.Name)

			// Wait till we have 3 pods (the third one is the validation one). Then, try to forward event.
			// Since we don't forward event from validation room, it should return true even though we're not
			// mocking the gRPC call.
			require.Eventually(t, func() bool {
				pods, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				require.NotEmpty(t, pods.Items)

				if len(pods.Items) == 3 {
					validationRoom := pods.Items[2].ObjectMeta.Name

					playerEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
						RoomName: validationRoom,

						Event:     "playerLeft",
						Timestamp: time.Now().Unix(),
						Metadata: &_struct.Struct{
							Fields: map[string]*structpb.Value{
								"playerId": {
									Kind: &structpb.Value_StringValue{
										StringValue: "c50acc91-4d88-46fa-aa56-48d63c5b5311",
									},
								},
								"eventMetadata1": {
									Kind: &structpb.Value_StringValue{
										StringValue: "value1",
									},
								},
								"eventMetadata2": {
									Kind: &structpb.Value_BoolValue{
										BoolValue: true,
									},
								},
							},
						},
					}
					playerEventResponse := &maestroApiV1.ForwardPlayerEventResponse{}
					err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/playerevent", scheduler.Name, validationRoom), playerEventRequest, playerEventResponse)
					require.NoError(t, err)
					require.Equal(t, true, playerEventResponse.Success)

					return true
				}

				return false
			}, 2*time.Minute, time.Second)

			waitForOperationToFinish(t, managementApiClient, scheduler.Name, "create_new_scheduler_version")
			waitForOperationToFinish(t, managementApiClient, scheduler.Name, "switch_active_version")

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: scheduler.Name}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				podsAfterUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				require.NotEmpty(t, podsAfterUpdate.Items)

				if len(podsAfterUpdate.Items) == 2 {
					return true
				}

				return false
			}, 2*time.Minute, time.Second)

			podsAfterUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, podsAfterUpdate.Items)

			for i := 0; i < 2; i++ {
				require.NotEqual(t, podsAfterUpdate.Items[i].Spec, podsBeforeUpdate.Items[i].Spec)
				require.Equal(t, "example-update", podsAfterUpdate.Items[i].Spec.Containers[0].Name)
			}
			require.Equal(t, "v2.0.0", getSchedulerResponse.Scheduler.Spec.Version)
		})

		t.Run("Should Fail - When scheduler when sending invalid request to update endpoint it fails fast", func(t *testing.T) {
			t.Parallel()

			scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, game, kubeClient)

			invalidUpdateRequest := &maestroApiV1.NewSchedulerVersionRequest{}
			updateResponse := &maestroApiV1.NewSchedulerVersionResponse{}

			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s", scheduler.Name), invalidUpdateRequest, updateResponse)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed with status 400")

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: scheduler.Name}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)
			require.NoError(t, err)
			require.Equal(t, "v1.0.0", getSchedulerResponse.Scheduler.Spec.Version)
		})

		t.Run("Should Fail - image of GRU is invalid. Operation fails, version and pods are unchanged", func(t *testing.T) {
			t.Parallel()

			scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, game, kubeClient)

			podsBeforeUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)

			updateRequest := &maestroApiV1.NewSchedulerVersionRequest{
				Name:     scheduler.Name,
				Game:     "test",
				MaxSurge: "10%",
				Spec: &maestrov1.Spec{
					TerminationGracePeriod: 15,
					Containers: []*maestroApiV1.Container{
						{
							Name:            "example-update",
							Image:           "alpine:3.15.0",
							Command:         []string{"while true; do sleep 1; done"},
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
				},
				PortRange: &maestroApiV1.PortRange{
					Start: 80,
					End:   8000,
				},
			}
			updateResponse := &maestroApiV1.NewSchedulerVersionResponse{}

			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s", scheduler.Name), updateRequest, updateResponse)
			require.NoError(t, err)
			require.NotNil(t, updateResponse.OperationId, scheduler.Name)

			waitForOperationToFail(t, managementApiClient, scheduler.Name, "create_new_scheduler_version")

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: scheduler.Name}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				podsAfterUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				require.NotEmpty(t, podsAfterUpdate.Items)

				if len(podsAfterUpdate.Items) == 2 {
					return true
				}

				return false
			}, 2*time.Minute, time.Second)

			podsAfterUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.Len(t, podsAfterUpdate.Items, 2)

			// Pod's haven't change
			for i := 0; i < 2; i++ {
				require.Equal(t, podsAfterUpdate.Items[i].Name, podsBeforeUpdate.Items[i].Name)
			}
			// version didn't change
			require.Equal(t, "v1.0.0", getSchedulerResponse.Scheduler.Spec.Version)

			getVersionsRequest := &maestroApiV1.GetSchedulerVersionsRequest{SchedulerName: scheduler.Name}
			getVersionsResponse := &maestroApiV1.GetSchedulerVersionsResponse{}

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/versions", scheduler.Name), getVersionsRequest, getVersionsResponse)
			require.NoError(t, err)

			// No version was created
			require.Len(t, getVersionsResponse.Versions, 1)
		})
	})
}
