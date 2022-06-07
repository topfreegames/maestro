package management

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework"
	"github.com/topfreegames/maestro/e2e/framework/maestro"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
	"testing"
	"time"
)

const autoscalingTestGame = "autoscaling-game"

func TestAutoscaling(t *testing.T) {
	t.Parallel()

	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		scheduler, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, autoscalingTestGame, kubeClient)
		require.NoError(t, err)

		t.Run("Autoscaling not configured - use rooms replicas value to maintain state", func(t *testing.T) {
			schedulerInfoRequest := &maestroApiV1.GetSchedulersInfoRequest{}
			schedulerInfoResponse := &maestroApiV1.GetSchedulersInfoResponse{}

			require.Eventually(t, func() bool {

				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/info?game=%s", scheduler.Game), schedulerInfoRequest, schedulerInfoResponse)
				require.NoError(t, err)
				require.NotEmpty(t, schedulerInfoResponse.GetSchedulers())
				schedulerInfo := schedulerInfoResponse.GetSchedulers()[0]
				require.Nil(t, schedulerInfo.GetAutoscaling())
				return schedulerInfo.GetRoomsReplicas() == int32(2) &&
					schedulerInfo.GetRoomsReady() == int32(2) &&
					schedulerInfo.GetRoomsOccupied() == int32(0) &&
					schedulerInfo.GetRoomsTerminating() == int32(0) &&
					schedulerInfo.GetRoomsPending() == int32(0)
			}, time.Minute*1, time.Second*5, "Scheduler info should be updated to match rooms replicas")
		})

		t.Run("Autoscaling configured and disabled - use rooms replicas value to maintain state", func(t *testing.T) {
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

			// Wait for healthController to check for state change
			time.Sleep(time.Second * 10)

			schedulerInfoRequest := &maestroApiV1.GetSchedulersInfoRequest{}
			schedulerInfoResponse := &maestroApiV1.GetSchedulersInfoResponse{}

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/info?game=%s", scheduler.Game), schedulerInfoRequest, schedulerInfoResponse)
			require.NoError(t, err)
			require.NotEmpty(t, schedulerInfoResponse.GetSchedulers())
			schedulerInfo := schedulerInfoResponse.GetSchedulers()[0]
			require.Equal(t, false, schedulerInfo.GetAutoscaling().GetEnabled())
			require.Equal(t, int32(1), schedulerInfo.GetAutoscaling().GetMin())
			require.Equal(t, int32(10), schedulerInfo.GetAutoscaling().GetMax())
			require.Equal(t, int32(2), schedulerInfo.GetRoomsReplicas())
			require.Equal(t, int32(2), schedulerInfo.GetRoomsReady())
			require.Equal(t, int32(0), schedulerInfo.GetRoomsOccupied())
			require.Equal(t, int32(0), schedulerInfo.GetRoomsPending())
			require.Equal(t, int32(0), schedulerInfo.GetRoomsTerminating())
		})

		t.Run("Autoscaling configured and enabled - use min max and configured RoomOccupancy policy to full scale up ", func(t *testing.T) {
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
			requireEventualState(t, 1, 1, managementApiClient)

			// 2 ready and 2 occupied
			requireEventualState(t, 2, 2, managementApiClient)

			// 4 ready and 4 occupied
			requireEventualState(t, 4, 4, managementApiClient)

			// 2 ready and 8 occupied
			requireEventualState(t, 2, 8, managementApiClient)

			// 10 occupied
			requireEventualState(t, 0, 10, managementApiClient)

		})

		t.Run("Autoscaling configured and enabled - use min max and configured RoomOccupancy policy to full scale down ", func(t *testing.T) {

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
				schedulerInfo := schedulerInfoResponse.GetSchedulers()[0]

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
			requireEventualState(t, 1, 0, managementApiClient)
		})

	})
}

func requireEventualState(t *testing.T, numberOfReadyRooms, numberOfOccupiedRooms int, managementApiClient *framework.APIClient) {
	schedulerInfoRequest := &maestroApiV1.GetSchedulersInfoRequest{}
	schedulerInfoResponse := &maestroApiV1.GetSchedulersInfoResponse{}
	require.Eventually(t, func() bool {
		err := managementApiClient.Do("GET", fmt.Sprintf("/schedulers/info?game=%s", autoscalingTestGame), schedulerInfoRequest, schedulerInfoResponse)
		require.NoError(t, err)
		require.NotEmpty(t, schedulerInfoResponse.GetSchedulers())
		schedulerInfo := schedulerInfoResponse.GetSchedulers()[0]
		if schedulerInfo.GetAutoscaling() == nil || schedulerInfo.GetAutoscaling().Enabled == false {
			return false
		}

		return schedulerInfo.GetRoomsReady() == int32(numberOfReadyRooms) &&
			schedulerInfo.GetRoomsOccupied() == int32(numberOfOccupiedRooms)
	}, time.Minute*1, time.Second*1, "Timeout waiting for scheduler to reach expected state")

}
