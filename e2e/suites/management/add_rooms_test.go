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

	"github.com/go-redis/redis/v8"
	roomStorageRedis "github.com/topfreegames/maestro/internal/adapters/room_storage/redis"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/stretchr/testify/require"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func TestAddRooms(t *testing.T) {
	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		roomsStorage := roomStorageRedis.NewRedisStateStorage(redisClient)

		t.Run("when created rooms does not reply its state back then it finishes the operation with error and delete created pods", func(t *testing.T) {
			t.Parallel()

			scheduler, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				managementApiClient,
				kubeClient,
				"test",
				[]string{"sh", "-c", "tail -f /dev/null"},
			)

			addRoomsRequest := &maestroApiV1.AddRoomsRequest{SchedulerName: scheduler.Name, Amount: 1}
			addRoomsResponse := &maestroApiV1.AddRoomsResponse{}
			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s/add-rooms", scheduler.Name), addRoomsRequest, addRoomsResponse)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", scheduler.Name), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.FinishedOperations) < 2 {
					return false
				}

				require.Equal(t, "add_rooms", listOperationsResponse.FinishedOperations[0].DefinitionName)
				require.Equal(t, "error", listOperationsResponse.FinishedOperations[0].Status)
				return true
			}, 240*time.Second, time.Second)

			require.Eventually(t, func() bool {
				pods, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				if len(pods.Items) == 0 {
					return false
				}
				return true
			}, 30*time.Second, time.Second)

		})

		t.Run("when created rooms replies its state back then it finishes the operation successfully", func(t *testing.T) {
			t.Parallel()

			scheduler, err := createSchedulerAndWaitForIt(t,
				maestro,
				managementApiClient,
				kubeClient,
				"test",
				[]string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request POST " +
					"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
					"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 1; done"})

			addRoomsRequest := &maestroApiV1.AddRoomsRequest{SchedulerName: scheduler.Name, Amount: 1}
			addRoomsResponse := &maestroApiV1.AddRoomsResponse{}
			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s/add-rooms", scheduler.Name), addRoomsRequest, addRoomsResponse)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", scheduler.Name), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.FinishedOperations) < 2 {
					return false
				}

				require.Equal(t, "add_rooms", listOperationsResponse.FinishedOperations[0].DefinitionName)
				require.Equal(t, "finished", listOperationsResponse.FinishedOperations[0].Status)
				return true
			}, 240*time.Second, 100*time.Millisecond)

			pods, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, pods.Items)

			require.Eventually(t, func() bool {
				room, err := roomsStorage.GetRoom(context.Background(), scheduler.Name, pods.Items[0].ObjectMeta.Name)

				return err == nil && room.Status == game_room.GameStatusReady
			}, time.Minute, 100*time.Millisecond)
		})

		t.Run("when trying to add rooms to a nonexistent scheduler then the operation fails", func(t *testing.T) {
			t.Parallel()

			schedulerName := "NonExistent"

			addRoomsRequest := &maestroApiV1.AddRoomsRequest{SchedulerName: schedulerName, Amount: 1}
			addRoomsResponse := &maestroApiV1.AddRoomsResponse{}
			err := managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s/add-rooms", schedulerName), addRoomsRequest, addRoomsResponse)
			require.Error(t, err, "failed with status 404, response body: {\"code\":5, \"message\":\"no "+
				"scheduler found to add rooms on it: scheduler NonExistent not found\", \"details\":[]}")
		})
	})

}
