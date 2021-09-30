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

	instanceStorageRedis "github.com/topfreegames/maestro/internal/adapters/instance_storage/redis"

	redisV8 "github.com/go-redis/redis/v8"
	roomStorageRedis "github.com/topfreegames/maestro/internal/adapters/room_storage/redis"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/stretchr/testify/require"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func TestRemoveRooms(t *testing.T) {
	framework.WithClients(t, func(apiClient *framework.APIClient, kubeclient kubernetes.Interface, redisClient *redisV8.Client, maestro *maestro.MaestroInstance) {
		instanceStorage := instanceStorageRedis.NewRedisInstanceStorage(redisClient, 10)
		roomsStorage := roomStorageRedis.NewRedisStateStorage(redisClient)

		t.Run("when game rooms are previously created with success should remove rooms with success", func(t *testing.T) {
			schedulerName, err := createSchedulerAndWaitForIt(t,
				maestro,
				apiClient,
				kubeclient,
				[]string{"/bin/sh", "-c", "apk add curl && curl --request POST " +
					"$ROOMS_API_ADDRESS:9097/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
					"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}'"})

			err, createdGameRoomName := addRoomsAndWaitForIt(t, schedulerName, err, apiClient, kubeclient, redisClient)
			require.NoError(t, err)

			err = instanceStorage.UpsertInstance(context.Background(), &game_room.Instance{
				ID:          createdGameRoomName,
				SchedulerID: schedulerName,
				Version:     "1.1",
				Status: game_room.InstanceStatus{
					Type:        2,
					Description: "ready",
				},
				Address: nil,
			})

			removeRoomsRequest := &maestroApiV1.RemoveRoomsRequest{SchedulerName: schedulerName, Amount: 1}
			removeRoomsResponse := &maestroApiV1.RemoveRoomsResponse{}
			err = apiClient.Do("POST", fmt.Sprintf("/schedulers/%s/remove-rooms", schedulerName), removeRoomsRequest, removeRoomsResponse)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.FinishedOperations) < 3 {
					return false
				}
				fmt.Println(listOperationsResponse.FinishedOperations)

				require.Equal(t, "remove_rooms", listOperationsResponse.FinishedOperations[2].DefinitionName)
				return true
			}, 240*time.Second, time.Second)

			require.Eventually(t, func() bool {
				pods, err := kubeclient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				if len(pods.Items) > 0 {
					return false
				}
				room, err := roomsStorage.GetRoom(context.Background(), schedulerName, createdGameRoomName)
				require.NoError(t, err)
				require.Equal(t, room.Status, game_room.GameStatusTerminating)
				return true
			}, 280*time.Second, time.Second*10)
		})

		t.Run("when some error occur when executing operation then it shouldn't remove room with success", func(t *testing.T) {
			schedulerName, err := createSchedulerAndWaitForIt(t,
				maestro,
				apiClient,
				kubeclient,
				[]string{"/bin/sh", "-c", "apk add curl && curl --request POST " +
					"$ROOMS_API_ADDRESS:9097/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
					"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}'"})

			err, createdGameRoomName := addRoomsAndWaitForIt(t, schedulerName, err, apiClient, kubeclient, redisClient)
			require.NoError(t, err)

			removeRoomsRequest := &maestroApiV1.RemoveRoomsRequest{SchedulerName: schedulerName, Amount: 1}
			removeRoomsResponse := &maestroApiV1.RemoveRoomsResponse{}
			err = apiClient.Do("POST", fmt.Sprintf("/schedulers/%s/remove-rooms", schedulerName), removeRoomsRequest, removeRoomsResponse)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.FinishedOperations) < 3 {
					return false
				}
				fmt.Println(listOperationsResponse.FinishedOperations)

				require.Equal(t, "remove_rooms", listOperationsResponse.FinishedOperations[2].DefinitionName)
				return true
			}, 240*time.Second, time.Second)

			require.Eventually(t, func() bool {
				pods, err := kubeclient.CoreV1().Pods(schedulerName).List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				if len(pods.Items) <= 0 {
					return false
				}
				room, err := roomsStorage.GetRoom(context.Background(), schedulerName, createdGameRoomName)
				require.NoError(t, err)
				require.Equal(t, room.Status, game_room.GameStatusReady)
				return true
			}, 240*time.Second, time.Second*10)
		})

		t.Run("when the provided scheduler doesn't exists then it should return error", func(t *testing.T) {
			schedulerName := "non-existent-name"
			removeRoomsRequest := &maestroApiV1.RemoveRoomsRequest{SchedulerName: schedulerName, Amount: 1}
			removeRoomsResponse := &maestroApiV1.RemoveRoomsResponse{}
			err := apiClient.Do("POST", fmt.Sprintf("/schedulers/%s/remove-rooms", schedulerName), removeRoomsRequest, removeRoomsResponse)
			require.Error(t, err)
		})

	})

}

func addRoomsAndWaitForIt(t *testing.T, schedulerName string, err error, apiClient *framework.APIClient, kubeclient kubernetes.Interface, redisClient *redisV8.Client) (error, string) {
	roomsStorage := roomStorageRedis.NewRedisStateStorage(redisClient)

	addRoomsRequest := &maestroApiV1.AddRoomsRequest{SchedulerName: schedulerName, Amount: 1}
	addRoomsResponse := &maestroApiV1.AddRoomsResponse{}
	err = apiClient.Do("POST", fmt.Sprintf("/schedulers/%s/add-rooms", schedulerName), addRoomsRequest, addRoomsResponse)

	require.Eventually(t, func() bool {
		listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
		listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
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

	createdGameRoomName := pods.Items[0].ObjectMeta.Name

	room, err := roomsStorage.GetRoom(context.Background(), schedulerName, createdGameRoomName)
	require.NoError(t, err)
	require.Equal(t, room.Status, game_room.GameStatusReady)
	return err, createdGameRoomName
}