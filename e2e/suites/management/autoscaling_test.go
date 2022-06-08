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
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework"
	"github.com/topfreegames/maestro/e2e/framework/maestro"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
)

const autoscalingTestGame = "autoscaling-game"

func TestAutoscaling(t *testing.T) {
	t.Parallel()

	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {

		t.Run("Autoscaling not configured - use rooms replicas value to maintain state", func(t *testing.T) {
			t.Parallel()
			scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, autoscalingTestGame, kubeClient)
			require.NoError(t, err)

			schedulerInfoRequest := &maestroApiV1.GetSchedulersInfoRequest{}
			schedulerInfoResponse := &maestroApiV1.GetSchedulersInfoResponse{}

			require.Eventually(t, func() bool {

				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/info?game=%s", scheduler.Game), schedulerInfoRequest, schedulerInfoResponse)
				require.NoError(t, err)
				require.NotEmpty(t, schedulerInfoResponse.GetSchedulers())
				schedulerInfo := getSchedulerInfo(scheduler.Name, schedulerInfoResponse.GetSchedulers())
				require.Nil(t, schedulerInfo.GetAutoscaling())
				return schedulerInfo.GetRoomsReplicas() == int32(2) &&
					schedulerInfo.GetRoomsReady() == int32(2) &&
					schedulerInfo.GetRoomsOccupied() == int32(0) &&
					schedulerInfo.GetRoomsTerminating() == int32(0) &&
					schedulerInfo.GetRoomsPending() == int32(0)
			}, time.Minute*1, time.Second*5, "Scheduler info should be updated to match rooms replicas")
		})

		t.Run("Autoscaling configured and disabled - use rooms replicas value to maintain state", func(t *testing.T) {
			t.Parallel()
			scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, autoscalingTestGame, kubeClient)
			require.NoError(t, err)

			enabled, min, max, policy := false, int32(1), int32(10), maestroApiV1.AutoscalingPolicy{
				Type:       string(autoscaling.RoomOccupancy),
				Parameters: &maestroApiV1.PolicyParameters{RoomOccupancy: &maestroApiV1.RoomOccupancy{ReadyTarget: 0.2}},
			}

			patchSchedulerRequest := &maestroApiV1.PatchSchedulerRequest{
				Autoscaling: &maestroApiV1.OptionalAutoscaling{
					Enabled: &enabled,
					Min:     &min,
					Max:     &max,
					Policy:  &policy,
				}}
			patchSchedulerResponse := &maestroApiV1.PatchSchedulerResponse{}
			err = managementApiClient.Do("PATCH", fmt.Sprintf("/schedulers/%s", scheduler.Name), patchSchedulerRequest, patchSchedulerResponse)
			require.NoError(t, err)

			schedulerInfoRequest := &maestroApiV1.GetSchedulersInfoRequest{}
			schedulerInfoResponse := &maestroApiV1.GetSchedulersInfoResponse{}

			require.Eventually(t, func() bool {
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/info?game=%s", scheduler.Game), schedulerInfoRequest, schedulerInfoResponse)
				require.NoError(t, err)
				require.NotEmpty(t, schedulerInfoResponse.GetSchedulers())
				schedulerInfo := getSchedulerInfo(scheduler.Name, schedulerInfoResponse.GetSchedulers())
				return schedulerInfo.GetAutoscaling() != nil &&
					schedulerInfo.GetAutoscaling().GetEnabled() == false &&
					schedulerInfo.GetAutoscaling().GetMax() == 10 &&
					schedulerInfo.GetAutoscaling().GetMin() == 1 &&
					schedulerInfo.GetRoomsReplicas() == 2
			}, time.Minute*1, time.Second*5)

			requireEventualState(t, 2, 0, scheduler.Name, managementApiClient)
		})

		t.Run("Autoscaling configured and enabled - use min max and configured RoomOccupancy policy to full scale up", func(t *testing.T) {
			t.Parallel()
			scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, autoscalingTestGame, kubeClient)
			require.NoError(t, err)

			enabled, min, max, policy := true, int32(1), int32(10), maestroApiV1.AutoscalingPolicy{
				Type:       string(autoscaling.RoomOccupancy),
				Parameters: &maestroApiV1.PolicyParameters{RoomOccupancy: &maestroApiV1.RoomOccupancy{ReadyTarget: 0.5}},
			}

			// Rooms will keep getting occupied to force a full scale up
			patchSchedulerRequest := &maestroApiV1.PatchSchedulerRequest{
				Spec: &maestroApiV1.OptionalSpec{
					Containers: []*maestroApiV1.OptionalContainer{
						{
							Command: []string{"/bin/sh", "-c", "apk add curl && " + "while true; do " +
								"curl --request PUT $ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping --data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && " +
								"sleep 5 && " +
								"curl --request PUT $ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping --data-raw '{\"status\": \"occupied\",\"timestamp\": \"12312312313\"}' && " +
								"sleep 999 ; done",
							},
						},
					}},
				Autoscaling: &maestroApiV1.OptionalAutoscaling{
					Enabled: &enabled,
					Min:     &min,
					Max:     &max,
					Policy:  &policy,
				}}
			patchSchedulerResponse := &maestroApiV1.PatchSchedulerResponse{}
			err = managementApiClient.Do("PATCH", fmt.Sprintf("/schedulers/%s", scheduler.Name), patchSchedulerRequest, patchSchedulerResponse)
			require.NoError(t, err)

			// Expect rooms to full scale up trying to maintain the 50% ready target since rooms will keep getting occupied
			// 1 ready and 1 occupied
			// 2 ready and 2 occupied
			// 4 ready and 4 occupied
			// 2 ready and 8 occupied
			// 10 occupied

			// 1 ready and 1 occupied
			requireEventualState(t, 1, 1, scheduler.Name, managementApiClient)

			// 2 ready and 2 occupied
			requireEventualState(t, 2, 2, scheduler.Name, managementApiClient)

			// 4 ready and 4 occupied
			requireEventualState(t, 4, 4, scheduler.Name, managementApiClient)

			// 2 ready and 8 occupied
			requireEventualState(t, 2, 8, scheduler.Name, managementApiClient)

			// 10 occupied
			requireEventualState(t, 0, 10, scheduler.Name, managementApiClient)

		})

		t.Run("Autoscaling configured and enabled - use min max and configured RoomOccupancy policy to full scale down ", func(t *testing.T) {
			t.Parallel()
			scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, autoscalingTestGame, kubeClient)
			require.NoError(t, err)

			enabled, min, max, policy := false, int32(1), int32(10), maestroApiV1.AutoscalingPolicy{
				Type:       string(autoscaling.RoomOccupancy),
				Parameters: &maestroApiV1.PolicyParameters{RoomOccupancy: &maestroApiV1.RoomOccupancy{ReadyTarget: 0.5}},
			}

			// First, force scheduler to have 10 ready rooms after enabling autoscaling
			roomsReplicas := int32(10)
			patchSchedulerRequest := &maestroApiV1.PatchSchedulerRequest{
				RoomsReplicas: &roomsReplicas,
				Spec: &maestroApiV1.OptionalSpec{
					Containers: []*maestroApiV1.OptionalContainer{
						{
							Command: []string{"/bin/sh", "-c", "apk add curl && " + "while true; do " +
								"curl --request PUT $ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping --data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && " +
								"sleep 5 ; done",
							},
						},
					}},
				Autoscaling: &maestroApiV1.OptionalAutoscaling{
					Enabled: &enabled,
					Min:     &min,
					Max:     &max,
					Policy:  &policy,
				},
			}
			patchSchedulerResponse := &maestroApiV1.PatchSchedulerResponse{}
			err = managementApiClient.Do("PATCH", fmt.Sprintf("/schedulers/%s", scheduler.Name), patchSchedulerRequest, patchSchedulerResponse)
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				schedulerInfoRequest := &maestroApiV1.GetSchedulersInfoRequest{}
				schedulerInfoResponse := &maestroApiV1.GetSchedulersInfoResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/info?game=%s", scheduler.Game), schedulerInfoRequest, schedulerInfoResponse)
				require.NoError(t, err)
				require.NotEmpty(t, schedulerInfoResponse.GetSchedulers())
				schedulerInfo := getSchedulerInfo(scheduler.Name, schedulerInfoResponse.GetSchedulers())

				return schedulerInfo.GetRoomsReady() == int32(10)
			}, time.Minute*2, time.Second*1, "Scheduler should initially have 10 ready rooms")

			// Now, enable autoscaling
			enabled = true

			// Rooms will keep getting occupied to force a full scale up
			patchSchedulerRequest = &maestroApiV1.PatchSchedulerRequest{
				Autoscaling: &maestroApiV1.OptionalAutoscaling{
					Enabled: &enabled,
				}}
			patchSchedulerResponse = &maestroApiV1.PatchSchedulerResponse{}
			err = managementApiClient.Do("PATCH", fmt.Sprintf("/schedulers/%s", scheduler.Name), patchSchedulerRequest, patchSchedulerResponse)
			require.NoError(t, err)

			// Expect rooms to full scale down trying to maintain the 50% ready target since no room will get occupied

			// 1 ready and 0 occupied
			requireEventualState(t, 1, 0, scheduler.Name, managementApiClient)
		})

	})
}

func requireEventualState(t *testing.T, numberOfReadyRooms, numberOfOccupiedRooms int, schedulerName string, managementApiClient *framework.APIClient) {
	schedulerInfoRequest := &maestroApiV1.GetSchedulersInfoRequest{}
	schedulerInfoResponse := &maestroApiV1.GetSchedulersInfoResponse{}
	require.Eventually(t, func() bool {
		err := managementApiClient.Do("GET", fmt.Sprintf("/schedulers/info?game=%s", autoscalingTestGame), schedulerInfoRequest, schedulerInfoResponse)
		require.NoError(t, err)
		require.NotEmpty(t, schedulerInfoResponse.GetSchedulers())
		schedulerInfo := getSchedulerInfo(schedulerName, schedulerInfoResponse.GetSchedulers())
		if schedulerInfo.GetAutoscaling() == nil {
			return false
		}

		return schedulerInfo.GetRoomsReady() == int32(numberOfReadyRooms) &&
			schedulerInfo.GetRoomsOccupied() == int32(numberOfOccupiedRooms)
	},
		time.Minute*3,
		time.Second*1,
		fmt.Sprintf("Timeout waiting for scheduler %s to reach expected state: ready %d, occupied %d", schedulerName, numberOfReadyRooms, numberOfOccupiedRooms),
	)

}

func getSchedulerInfo(schedulerName string, schedulers []*maestroApiV1.SchedulerInfo) *maestroApiV1.SchedulerInfo {
	var schedulerInfo *maestroApiV1.SchedulerInfo
	for _, scheduler := range schedulers {
		if scheduler.GetName() == schedulerName {
			schedulerInfo = scheduler
			break
		}
	}
	return schedulerInfo
}
