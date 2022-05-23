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

	_struct "github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/protobuf/types/known/structpb"

	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-redis/redis/v8"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework"
	"k8s.io/client-go/kubernetes"
)

func TestSwitchActiveVersion(t *testing.T) {
	t.Parallel()

	game := "switch-active-version-game"

	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		t.Run("Succeed - create minor version, rollback version", func(t *testing.T) {
			t.Parallel()

			roomsApiAddress := maestro.RoomsApiServer.ContainerInternalAddress
			scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, game, kubeClient)

			// Update scheduler
			updateRequest := &maestroApiV1.NewSchedulerVersionRequest{
				Name:     scheduler.Name,
				Game:     "test",
				MaxSurge: "10%",
				Spec: &maestroApiV1.Spec{
					TerminationGracePeriod: 15,
					Containers: []*maestroApiV1.Container{
						{
							Name:  "example",
							Image: "alpine:3.15.0",
							Command: []string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request PUT " +
								"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
								"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 1; done"},
							ImagePullPolicy: "Always",
							Environment: []*maestroApiV1.ContainerEnvironment{
								{
									Name:  "ROOMS_API_ADDRESS",
									Value: &roomsApiAddress,
								},
								{
									Name: "HOST_IP",
									ValueFrom: &maestroApiV1.ContainerEnvironmentValueFrom{
										FieldRef: &maestroApiV1.ContainerEnvironmentValueFromFieldRef{FieldPath: "status.hostIP"},
									},
								},
								{
									Name: "SECRET_ENV_VAR",
									ValueFrom: &maestroApiV1.ContainerEnvironmentValueFrom{
										SecretKeyRef: &maestroApiV1.ContainerEnvironmentValueFromSecretKeyRef{Name: "namespace-secret", Key: "secret_key"},
									},
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
				RoomsReplicas: 2,
				PortRange: &maestroApiV1.PortRange{
					Start: 40000,
					End:   60000,
				},
			}
			updateResponse := &maestroApiV1.NewSchedulerVersionResponse{}

			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s", scheduler.Name), updateRequest, updateResponse)
			require.NoError(t, err)
			require.NotNil(t, updateResponse.OperationId, scheduler.Name)

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			require.Eventually(t, func() bool {

				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)
				require.NoError(t, err)
				if getSchedulerResponse.Scheduler.Spec.Version != "v1.2.0" {
					return false
				}

				return true
			}, 1*time.Minute, 10*time.Millisecond)

			require.Eventually(t, func() bool {
				podsAfterUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				require.NotEmpty(t, podsAfterUpdate.Items)

				if len(podsAfterUpdate.Items) == 2 {
					return true
				}

				return false
			}, 1*time.Minute, 100*time.Millisecond)

			podsAfterUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, podsAfterUpdate.Items)

			podsBeforeSwitch, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.Len(t, podsBeforeSwitch.Items, 2)

			switchActiveVersionRequest := &maestroApiV1.SwitchActiveVersionRequest{
				SchedulerName: scheduler.Name,
				Version:       "v1.1.0",
			}
			switchActiveVersionResponse := &maestroApiV1.SwitchActiveVersionResponse{}
			// Switch to v1.1.0
			err = managementApiClient.Do("PUT", fmt.Sprintf("/schedulers/%s", scheduler.Name), switchActiveVersionRequest, switchActiveVersionResponse)
			require.NoError(t, err)

			// New Switch Active Version
			waitForOperationToFinishByOperationId(t, managementApiClient, scheduler.Name, switchActiveVersionResponse.OperationId)

			getSchedulerAfterSwitchResponse := &maestroApiV1.GetSchedulerResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerAfterSwitchResponse)
			require.NoError(t, err)
			require.NotEqual(t, getSchedulerAfterSwitchResponse.Scheduler.Spec.Version, getSchedulerResponse.Scheduler.Spec.Version)

			podsAfterSwitch, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, podsAfterSwitch.Items)
			require.Len(t, podsAfterSwitch.Items, 2)
			// Pods don't change since it's a minor rollback
			for i := 0; i < 2; i++ {
				require.Equal(t, podsBeforeSwitch.Items[i].Name, podsAfterSwitch.Items[i].Name)
			}

			podsBeforeSwitch, err = kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.Len(t, podsBeforeSwitch.Items, 2)

			switchActiveVersionRequest = &maestroApiV1.SwitchActiveVersionRequest{
				SchedulerName: scheduler.Name,
				Version:       "v1.2.0",
			}
			switchActiveVersionResponse = &maestroApiV1.SwitchActiveVersionResponse{}

			// Switch to v1.2.0
			err = managementApiClient.Do("PUT", fmt.Sprintf("/schedulers/%s", scheduler.Name), switchActiveVersionRequest, switchActiveVersionResponse)
			require.NoError(t, err)

			// New Switch Active Version
			waitForOperationToFinishByOperationId(t, managementApiClient, scheduler.Name, switchActiveVersionResponse.OperationId)

			getSchedulerResponsePreviousVersion := getSchedulerAfterSwitchResponse
			getSchedulerAfterSwitchResponse = &maestroApiV1.GetSchedulerResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerAfterSwitchResponse)
			require.NoError(t, err)
			require.NotEqual(t, getSchedulerAfterSwitchResponse.Scheduler.Spec.Version, getSchedulerResponsePreviousVersion.Scheduler.Spec.Version)

			podsAfterSwitch, err = kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})

			require.NoError(t, err)
			require.NotEmpty(t, podsAfterSwitch.Items)
			require.Len(t, podsAfterSwitch.Items, 2)
			// Pods don't change since it's a minor rollback
			for i := 0; i < 2; i++ {
				require.Equal(t, podsBeforeSwitch.Items[i].Name, podsAfterSwitch.Items[i].Name)
			}
		})

		t.Run("Succeed - create major change, rollback version", func(t *testing.T) {
			t.Parallel()

			roomsApiAddress := maestro.RoomsApiServer.ContainerInternalAddress
			scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, game, kubeClient)

			podsBeforeUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)

			updateRequest := &maestroApiV1.NewSchedulerVersionRequest{
				Name:          scheduler.Name,
				Game:          "test",
				MaxSurge:      "10%",
				RoomsReplicas: 2,
				Spec: &maestroApiV1.Spec{
					TerminationGracePeriod: 15,
					Containers: []*maestroApiV1.Container{
						{
							Name:  "example-update",
							Image: "alpine:3.15.0",
							Command: []string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request PUT " +
								"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
								"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 2; done"},
							ImagePullPolicy: "Always",
							Environment: []*maestroApiV1.ContainerEnvironment{
								{
									Name:  "ROOMS_API_ADDRESS",
									Value: &roomsApiAddress,
								},
								{
									Name: "HOST_IP",
									ValueFrom: &maestroApiV1.ContainerEnvironmentValueFrom{
										FieldRef: &maestroApiV1.ContainerEnvironmentValueFromFieldRef{FieldPath: "status.hostIP"},
									},
								},
								{
									Name: "SECRET_ENV_VAR",
									ValueFrom: &maestroApiV1.ContainerEnvironmentValueFrom{
										SecretKeyRef: &maestroApiV1.ContainerEnvironmentValueFromSecretKeyRef{Name: "namespace-secret", Key: "secret_key"},
									},
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
					Start: 4000,
					End:   6000,
				},
			}
			updateResponse := &maestroApiV1.NewSchedulerVersionResponse{}

			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s", scheduler.Name), updateRequest, updateResponse)
			require.NoError(t, err)
			require.NotNil(t, updateResponse.OperationId, scheduler.Name)

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			require.Eventually(t, func() bool {
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)
				require.NoError(t, err)
				if getSchedulerResponse.Scheduler.Spec.Version != "v2.0.0" {
					return false
				}

				return true
			}, 1*time.Minute, 10*time.Millisecond)

			require.Eventually(t, func() bool {
				podsAfterUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				require.NotEmpty(t, podsAfterUpdate.Items)

				if len(podsAfterUpdate.Items) == 2 && podsAfterUpdate.Items[0].Spec.Containers[0].Name == "example-update" {
					return true
				}

				return false
			}, 1*time.Minute, 10*time.Millisecond)

			podsAfterUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, podsAfterUpdate.Items)

			for i := 0; i < 2; i++ {
				require.NotEqual(t, podsAfterUpdate.Items[i].Spec, podsBeforeUpdate.Items[i].Spec)
				require.Equal(t, "example-update", podsAfterUpdate.Items[i].Spec.Containers[0].Name)
			}

			// Rollback scheduler version
			switchActiveVersionRequest := &maestroApiV1.SwitchActiveVersionRequest{
				SchedulerName: scheduler.Name,
				Version:       "v1.1.0",
			}
			switchActiveVersionResponse := &maestroApiV1.SwitchActiveVersionResponse{}

			err = managementApiClient.Do("PUT", fmt.Sprintf("/schedulers/%s", scheduler.Name), switchActiveVersionRequest, switchActiveVersionResponse)
			require.NoError(t, err)

			// New Switch Active Version
			waitForOperationToFinishByOperationId(t, managementApiClient, scheduler.Name, switchActiveVersionResponse.OperationId)

			getSchedulerAfterSwitchResponse := &maestroApiV1.GetSchedulerResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerAfterSwitchResponse)
			require.NoError(t, err)
			require.NotEqual(t, getSchedulerAfterSwitchResponse.Scheduler.Spec.Version, getSchedulerResponse.Scheduler.Spec.Version)

			require.Eventually(t, func() bool {
				podsAfterRollback, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				require.NotEmpty(t, podsAfterUpdate.Items)

				if len(podsAfterRollback.Items) == 2 {
					return true
				}

				return false
			}, 1*time.Minute, 10*time.Millisecond)

			podsAfterRollback, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, podsAfterRollback.Items)

			// Pods change since it's a major rollback
			for i := 0; i < 2; i++ {
				require.NotEqual(t, podsAfterUpdate.Items[i].Name, podsAfterRollback.Items[i].Name)
			}
		})

		t.Run("Fail - version does not exist", func(t *testing.T) {
			t.Parallel()

			roomsApiAddress := maestro.RoomsApiServer.ContainerInternalAddress
			scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, game, kubeClient)

			podsBeforeUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)

			updateRequest := &maestroApiV1.NewSchedulerVersionRequest{
				Name:     scheduler.Name,
				Game:     "test",
				MaxSurge: "10%",
				Spec: &maestroApiV1.Spec{
					TerminationGracePeriod: 15,
					Containers: []*maestroApiV1.Container{
						{
							Name:  "example-update",
							Image: "alpine:3.15.0",
							Command: []string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request PUT " +
								"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
								"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 1; done"},
							ImagePullPolicy: "Always",
							Environment: []*maestroApiV1.ContainerEnvironment{
								{
									Name:  "ROOMS_API_ADDRESS",
									Value: &roomsApiAddress,
								},
								{
									Name: "HOST_IP",
									ValueFrom: &maestroApiV1.ContainerEnvironmentValueFrom{
										FieldRef: &maestroApiV1.ContainerEnvironmentValueFromFieldRef{FieldPath: "status.hostIP"},
									},
								},
								{
									Name: "SECRET_ENV_VAR",
									ValueFrom: &maestroApiV1.ContainerEnvironmentValueFrom{
										SecretKeyRef: &maestroApiV1.ContainerEnvironmentValueFromSecretKeyRef{Name: "namespace-secret", Key: "secret_key"},
									},
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
					Start: 40000,
					End:   60000,
				},
			}
			updateResponse := &maestroApiV1.NewSchedulerVersionResponse{}

			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s", scheduler.Name), updateRequest, updateResponse)
			require.NoError(t, err)
			require.NotNil(t, updateResponse.OperationId, scheduler.Name)

			require.Eventually(t, func() bool {
				getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{}
				getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)
				require.NoError(t, err)
				if getSchedulerResponse.Scheduler.Spec.Version != "v2.0.0" {
					return false
				}

				return true
			}, 2*time.Minute, 10*time.Millisecond)

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
			}, 2*time.Minute, 10*time.Millisecond)

			podsAfterUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, podsAfterUpdate.Items)

			for i := 0; i < 2; i++ {
				require.NotEqual(t, podsAfterUpdate.Items[i].Spec, podsBeforeUpdate.Items[i].Spec)
				require.Equal(t, "example-update", podsAfterUpdate.Items[i].Spec.Containers[0].Name)
			}

			// Rollback scheduler version
			switchActiveVersionRequest := &maestroApiV1.SwitchActiveVersionRequest{
				SchedulerName: scheduler.Name,
				Version:       "DOES_NOT_EXIST",
			}
			switchActiveVersionResponse := &maestroApiV1.SwitchActiveVersionResponse{}

			err = managementApiClient.Do("PUT", fmt.Sprintf("/schedulers/%s", scheduler.Name), switchActiveVersionRequest, switchActiveVersionResponse)
			require.NoError(t, err)

			waitForOperationToFailById(t, managementApiClient, scheduler.Name, switchActiveVersionResponse.OperationId)
		})

		t.Run("Fail - adding forwarders crashes, testing scheduler cache", func(t *testing.T) {
			t.Parallel()
			roomsApiAddress := maestro.RoomsApiServer.ContainerInternalAddress
			forwarders := []*maestroApiV1.Forwarder{
				{
					Name:    "matchmaker-grpc",
					Enable:  true,
					Type:    "gRPC",
					Address: maestro.ServerMocks.GrpcForwarderAddress,
					Options: &maestroApiV1.ForwarderOptions{
						Timeout: 5000,
						Metadata: &_struct.Struct{
							Fields: map[string]*structpb.Value{
								"roomType": {
									Kind: &structpb.Value_StringValue{
										StringValue: "green",
									},
								},
								"forwarderMetadata1": {
									Kind: &structpb.Value_StringValue{
										StringValue: "value1",
									},
								},
								"forwarderMetadata2": {
									Kind: &structpb.Value_NumberValue{
										NumberValue: 245,
									},
								},
							},
						},
					},
				},
			}

			scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, game, kubeClient)
			firstPods, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			firstRoomName := firstPods.Items[0].ObjectMeta.Name

			// Forward player event true return true since there is no forwarder configured
			require.True(t, canForwardPlayerEvent(roomsApiClient, scheduler.Name, firstRoomName))

			// Update scheduler
			updateRequest := &maestroApiV1.NewSchedulerVersionRequest{
				Name:     scheduler.Name,
				Game:     "test",
				MaxSurge: "10%",
				Spec: &maestroApiV1.Spec{
					TerminationGracePeriod: 15,
					Containers: []*maestroApiV1.Container{
						{
							Name:  "example",
							Image: "alpine:3.15.0",
							Command: []string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request PUT " +
								"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
								"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 1; done"},
							ImagePullPolicy: "Always",
							Environment: []*maestroApiV1.ContainerEnvironment{
								{
									Name:  "ROOMS_API_ADDRESS",
									Value: &roomsApiAddress,
								},
								{
									Name: "HOST_IP",
									ValueFrom: &maestroApiV1.ContainerEnvironmentValueFrom{
										FieldRef: &maestroApiV1.ContainerEnvironmentValueFromFieldRef{FieldPath: "status.hostIP"},
									},
								},
								{
									Name: "SECRET_ENV_VAR",
									ValueFrom: &maestroApiV1.ContainerEnvironmentValueFrom{
										SecretKeyRef: &maestroApiV1.ContainerEnvironmentValueFromSecretKeyRef{Name: "namespace-secret", Key: "secret_key"},
									},
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
				RoomsReplicas: 2,
				PortRange: &maestroApiV1.PortRange{
					Start: 40000,
					End:   60000,
				},
				Forwarders: forwarders,
			}
			updateResponse := &maestroApiV1.NewSchedulerVersionResponse{}

			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s", scheduler.Name), updateRequest, updateResponse)
			require.NoError(t, err)
			require.NotNil(t, updateResponse.OperationId, scheduler.Name)

			require.Eventually(t, func() bool {
				getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{}
				getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)
				require.NoError(t, err)
				if getSchedulerResponse.Scheduler.Spec.Version != "v2.0.0" {
					return false
				}

				return true
			}, 1*time.Minute, 10*time.Millisecond)

			require.Eventually(t, func() bool {
				podsAfterUpdate, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				require.NotEmpty(t, podsAfterUpdate.Items)

				if len(podsAfterUpdate.Items) == 2 {
					return true
				}

				return false
			}, 1*time.Minute, 100*time.Millisecond)

			lastPods, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			lastRoomName := lastPods.Items[0].ObjectMeta.Name

			// Forward player event return false since there is no mock for grpc, and now we have forwarders
			require.False(t, canForwardPlayerEvent(roomsApiClient, scheduler.Name, lastRoomName))
		})
	})
}

func canForwardPlayerEvent(roomsApiClient *framework.APIClient, schedulerName, roomName string) bool {
	playerEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
		RoomName:  roomName,
		Event:     "playerLeft",
		Timestamp: time.Now().Unix(),
		Metadata: &_struct.Struct{
			Fields: map[string]*structpb.Value{
				"playerId": {
					Kind: &structpb.Value_StringValue{
						StringValue: "invalid-id",
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
	_ = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/playerevent", schedulerName, roomName), playerEventRequest, playerEventResponse)
	return playerEventResponse.Success
}
