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

	"github.com/go-redis/redis"
	roomStorageRedis "github.com/topfreegames/maestro/internal/adapters/room_storage/redis"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/stretchr/testify/require"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	maestrov1 "github.com/topfreegames/maestro/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func TestAddRooms(t *testing.T) {
	framework.WithClients(t, func(apiClient *framework.APIClient, kubeclient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {

		roomsStorage := roomStorageRedis.NewRedisStateStorage(redisClient)

		t.Run("when created rooms does not reply its state back then it finishes the operation successfully", func(t *testing.T) {
			schedulerName := framework.GenerateSchedulerName()
			createRequest := &maestrov1.CreateSchedulerRequest{
				Name:                   schedulerName,
				Game:                   "test",
				Version:                "v1.1",
				TerminationGracePeriod: 15,
				Containers: []*maestrov1.Container{
					{
						Name:            "example",
						Image:           "alpine",
						Command:         []string{"sh", "-c", "tail -f /dev/null"},
						ImagePullPolicy: "Always",
						Requests: &maestrov1.ContainerResources{
							Memory: "20Mi",
							Cpu:    "10m",
						},
						Limits: &maestrov1.ContainerResources{
							Memory: "20Mi",
							Cpu:    "10m",
						},
						Ports: []*maestrov1.ContainerPort{
							{
								Name:     "default",
								Protocol: "tcp",
								Port:     80,
							},
						},
					},
				},
			}

			createResponse := &maestrov1.CreateSchedulerResponse{}
			err := apiClient.Do("POST", "/schedulers", createRequest, createResponse)
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestrov1.ListOperationsRequest{}
				listOperationsResponse := &maestrov1.ListOperationsResponse{}
				err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.FinishedOperations) == 0 {
					return false
				}

				require.Equal(t, "create_scheduler", listOperationsResponse.FinishedOperations[0].DefinitionName)
				require.Equal(t, "finished", listOperationsResponse.FinishedOperations[0].Status)
				return true
			}, 30*time.Second, time.Second)

			_, err = kubeclient.CoreV1().Namespaces().Get(context.Background(), schedulerName, metav1.GetOptions{})
			require.NoError(t, err)

			// wait for service account to be created
			// TODO: check if we need to wait the service account to be created on internal/adapters/runtime/kubernetes/scheduler.go
			// we were having errors when not waiting for this in this test, reported in this issue https://github.com/kubernetes/kubernetes/issues/66689
			require.Eventually(t, func() bool {
				svcAccs, err := kubeclient.CoreV1().ServiceAccounts(schedulerName).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)

				return len(svcAccs.Items) > 0
			}, 5*time.Second, time.Second)

			addRoomsRequest := &maestrov1.AddRoomsRequest{
				SchedulerName: schedulerName,
				Amount:        1,
			}
			addRoomsResponse := &maestrov1.AddRoomsResponse{}
			err = apiClient.Do("POST", fmt.Sprintf("/schedulers/%s/add-rooms", schedulerName), addRoomsRequest, addRoomsResponse)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestrov1.ListOperationsRequest{}
				listOperationsResponse := &maestrov1.ListOperationsResponse{}
				err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.FinishedOperations) < 2 {
					return false
				}

				require.Equal(t, "add_rooms", listOperationsResponse.FinishedOperations[1].DefinitionName)
				return true
			}, 240*time.Second, time.Second)

			pods, err := kubeclient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, pods.Items)

			room, err := roomsStorage.GetRoom(context.Background(), schedulerName, pods.Items[0].ObjectMeta.Name)
			require.NoError(t, err)
			require.Equal(t, room.Status, game_room.GameStatusPending)
		})

		t.Run("when created rooms replies its state back then it finishes the operation successfully", func(t *testing.T) {

			schedulerName := framework.GenerateSchedulerName()
			createRequest := &maestrov1.CreateSchedulerRequest{
				Name:                   schedulerName,
				Game:                   "test",
				Version:                "v1.1",
				TerminationGracePeriod: 15,
				Containers: []*maestrov1.Container{
					{
						Name:  "example",
						Image: "alpine",
						Command: []string{"/bin/sh", "-c", "apk add curl && curl --request POST " +
							"$ROOMS_API_ADDRESS:9097/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
							"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}'"},
						ImagePullPolicy: "Always",
						Environment: []*maestrov1.ContainerEnvironment{
							{
								Name:  "ROOMS_API_ADDRESS",
								Value: maestro.RoomsApiServer.ContainerInternalAddress,
							},
						},
						Requests: &maestrov1.ContainerResources{
							Memory: "20Mi",
							Cpu:    "10m",
						},
						Limits: &maestrov1.ContainerResources{
							Memory: "20Mi",
							Cpu:    "10m",
						},
						Ports: []*maestrov1.ContainerPort{
							{
								Name:     "default",
								Protocol: "tcp",
								Port:     80,
							},
						},
					},
				},
			}

			createResponse := &maestrov1.CreateSchedulerResponse{}
			err := apiClient.Do("POST", "/schedulers", createRequest, createResponse)
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestrov1.ListOperationsRequest{}
				listOperationsResponse := &maestrov1.ListOperationsResponse{}
				err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.FinishedOperations) == 0 {
					return false
				}

				require.Equal(t, "create_scheduler", listOperationsResponse.FinishedOperations[0].DefinitionName)
				require.Equal(t, "finished", listOperationsResponse.FinishedOperations[0].Status)
				return true
			}, 30*time.Second, time.Second)

			// Check on kubernetes that the scheduler namespace was created.
			_, err = kubeclient.CoreV1().Namespaces().Get(context.Background(), schedulerName, metav1.GetOptions{})
			require.NoError(t, err)

			// wait for service account to be created
			// TODO: check if we need to wait the service account to be created on internal/adapters/runtime/kubernetes/scheduler.go
			// we were having errors when not waiting for this in this test, reported in this issue https://github.com/kubernetes/kubernetes/issues/66689
			require.Eventually(t, func() bool {
				svcAccs, err := kubeclient.CoreV1().ServiceAccounts(schedulerName).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)

				return len(svcAccs.Items) > 0
			}, 5*time.Second, time.Second)

			addRoomsRequest := &maestrov1.AddRoomsRequest{
				SchedulerName: schedulerName,
				Amount:        1,
			}
			addRoomsResponse := &maestrov1.AddRoomsResponse{}
			err = apiClient.Do("POST", fmt.Sprintf("/schedulers/%s/add-rooms", schedulerName), addRoomsRequest, addRoomsResponse)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestrov1.ListOperationsRequest{}
				listOperationsResponse := &maestrov1.ListOperationsResponse{}
				err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.FinishedOperations) < 2 {
					return false
				}

				require.Equal(t, "add_rooms", listOperationsResponse.FinishedOperations[1].DefinitionName)
				return true
			}, 240*time.Second, time.Second)

			pods, err := kubeclient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, pods.Items)

			room, err := roomsStorage.GetRoom(context.Background(), schedulerName, pods.Items[0].ObjectMeta.Name)
			require.NoError(t, err)
			require.Equal(t, room.Status, game_room.GameStatusReady)
		})
	})

}
