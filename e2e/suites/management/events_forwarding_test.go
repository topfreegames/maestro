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

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func TestEventsForwarding(t *testing.T) {
	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {

		t.Run("[Player event success] Forward player event return success true when no error occurs while forwarding events call", func(t *testing.T) {
			t.Parallel()

			schedulerName, roomName := createSchedulerWithForwarderAndRooms(t, maestro, kubeClient, managementApiClient, maestro.ServerMocks.GrpcForwarderAddress)

			// This configuration make the grpc service return with success
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-player-event-success")
			require.NoError(t, err)

			playerEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
				RoomName: roomName,

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
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/playerevent", schedulerName, roomName), playerEventRequest, playerEventResponse)
			require.NoError(t, err)
			require.Equal(t, true, playerEventResponse.Success)
		})

		t.Run("[Player event success] Forward player event return success true when no forwarder is configured for the scheduler", func(t *testing.T) {
			t.Parallel()

			schedulerName, roomName := createSchedulerWithRooms(t, maestro, kubeClient, managementApiClient)

			playerEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
				RoomName:  roomName,
				Event:     "playerLeft",
				Timestamp: time.Now().Unix(),
				Metadata: &_struct.Struct{
					Fields: map[string]*structpb.Value{
						"playerId": {
							Kind: &structpb.Value_StringValue{
								StringValue: "5280087d-6dff-4bbf-abc8-45cb8786ad00",
							},
						},
					},
				},
			}
			playerEventResponse := &maestroApiV1.ForwardPlayerEventResponse{}
			err := roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/playerevent", schedulerName, roomName), playerEventRequest, playerEventResponse)
			require.NoError(t, err)
			require.Equal(t, true, playerEventResponse.Success)
		})

		t.Run("[Player event failure] Forward player event return success false when some error occurs in GRPC call", func(t *testing.T) {
			t.Parallel()

			schedulerName, roomName := createSchedulerWithForwarderAndRooms(t, maestro, kubeClient, managementApiClient, maestro.ServerMocks.GrpcForwarderAddress)

			// This configuration make the grpc service return with error
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-player-event-failure")

			playerEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
				RoomName: roomName,

				Event:     "playerLeft",
				Timestamp: time.Now().Unix(),
				Metadata: &_struct.Struct{

					Fields: map[string]*structpb.Value{
						"playerId": {
							Kind: &structpb.Value_StringValue{
								StringValue: "446bb3d0-0334-4468-a4e7-8068a97caa53",
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
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/playerevent", schedulerName, roomName), playerEventRequest, playerEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, playerEventResponse.Success)
			require.Equal(t, "failed to forward event room at \"matchmaker-grpc\"", playerEventResponse.Message)
		})

		t.Run("[Player event failure] Forward player event return success false when forwarding event for an inexistent room", func(t *testing.T) {
			t.Parallel()

			schedulerName, _ := createSchedulerWithForwarderAndRooms(t, maestro, kubeClient, managementApiClient, maestro.ServerMocks.GrpcForwarderAddress)
			roomName := "inexistent-room"

			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-player-event-failure")

			playerEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
				RoomName:  roomName,
				Event:     "playerLeft",
				Timestamp: time.Now().Unix(),
				Metadata: &_struct.Struct{
					Fields: map[string]*structpb.Value{
						"playerId": {
							Kind: &structpb.Value_StringValue{
								StringValue: "c50c9d8a-5a40-40ee-97ea-d477d7b0abd9",
							},
						},
					},
				},
			}
			playerEventResponse := &maestroApiV1.ForwardPlayerEventResponse{}
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/playerevent", schedulerName, roomName), playerEventRequest, playerEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, playerEventResponse.Success)
		})

		t.Run("[Player event failure] Forward player event return success false when the forwarder connection can't be established", func(t *testing.T) {
			t.Parallel()

			schedulerName, roomName := createSchedulerWithForwarderAndRooms(t, maestro, kubeClient, managementApiClient, "invalid-grpc-address:9982")

			playerEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
				RoomName:  roomName,
				Event:     "playerLeft",
				Timestamp: time.Now().Unix(),
				Metadata: &_struct.Struct{
					Fields: map[string]*structpb.Value{
						"playerId": {
							Kind: &structpb.Value_StringValue{
								StringValue: "c50c9d8a-5a40-40ee-97ea-d477d7b0abd9",
							},
						},
					},
				},
			}
			playerEventResponse := &maestroApiV1.ForwardPlayerEventResponse{}
			err := roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/playerevent", schedulerName, roomName), playerEventRequest, playerEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, playerEventResponse.Success)
			require.Contains(t, playerEventResponse.Message, "transport: Error while dialing dial tcp: lookup invalid-grpc-address")
		})

		t.Run("[Room event success] Forward room event return success true when no error occurs while forwarding events call", func(t *testing.T) {
			t.Parallel()

			schedulerName, roomName := createSchedulerWithForwarderAndRooms(t, maestro, kubeClient, managementApiClient, maestro.ServerMocks.GrpcForwarderAddress)

			// This configuration make the grpc service return with success
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-room-event-success")
			require.NoError(t, err)

			roomEventRequest := &maestroApiV1.ForwardRoomEventRequest{
				RoomName:  roomName,
				Event:     "roomReady",
				Timestamp: time.Now().Unix(),
				Metadata: &_struct.Struct{
					Fields: map[string]*structpb.Value{
						"mockIdentifier": {
							Kind: &structpb.Value_StringValue{
								StringValue: "5437c6c8-3b06-41e6-b87a-de88834503de",
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
			roomEventResponse := &maestroApiV1.ForwardPlayerEventResponse{}
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/roomevent", schedulerName, roomName), roomEventRequest, roomEventResponse)
			require.NoError(t, err)
			require.Equal(t, true, roomEventResponse.Success)
		})

		t.Run("[Room event success] Forward room event return success true when no forwarder is configured for the scheduler", func(t *testing.T) {
			t.Parallel()

			schedulerName, roomName := createSchedulerWithRooms(t, maestro, kubeClient, managementApiClient)

			roomEventRequest := &maestroApiV1.ForwardRoomEventRequest{
				RoomName:  roomName,
				Event:     "ready",
				Timestamp: time.Now().Unix(),
				Metadata: &_struct.Struct{
					Fields: map[string]*structpb.Value{
						"mockIdentifier": {
							Kind: &structpb.Value_StringValue{
								StringValue: "c146705f-14ae-480f-8d5a-83711d44426f",
							},
						},
					},
				},
			}
			roomEventResponse := &maestroApiV1.ForwardPlayerEventResponse{}
			err := roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/roomevent", schedulerName, roomName), roomEventRequest, roomEventResponse)
			require.NoError(t, err)
			require.Equal(t, true, roomEventResponse.Success)
		})

		t.Run("[Room event failures] Forward room event return success false when some error occurs in GRPC call", func(t *testing.T) {
			t.Parallel()

			schedulerName, roomName := createSchedulerWithForwarderAndRooms(t, maestro, kubeClient, managementApiClient, maestro.ServerMocks.GrpcForwarderAddress)

			// This configuration make the grpc service return with failure
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-room-event-failure")
			require.NoError(t, err)

			roomEventRequest := &maestroApiV1.ForwardRoomEventRequest{
				RoomName:  roomName,
				Event:     "roomReady",
				Timestamp: time.Now().Unix(),
				Metadata: &_struct.Struct{
					Fields: map[string]*structpb.Value{
						"mockIdentifier": {
							Kind: &structpb.Value_StringValue{
								StringValue: "f942d3d9-1b47-4631-904f-a154894b54ce",
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
			roomEventResponse := &maestroApiV1.ForwardPlayerEventResponse{}
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/roomevent", schedulerName, roomName), roomEventRequest, roomEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, roomEventResponse.Success)
			require.Equal(t, "failed to forward event room at \"matchmaker-grpc\"", roomEventResponse.Message)
		})

		t.Run("[Room event failure] Forward player event return success false when forwarding event for an inexistent room", func(t *testing.T) {
			t.Parallel()

			schedulerName, _ := createSchedulerWithForwarderAndRooms(t, maestro, kubeClient, managementApiClient, maestro.ServerMocks.GrpcForwarderAddress)
			roomName := "inexistent-room"

			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-player-event-failure")

			roomEventRequest := &maestroApiV1.ForwardRoomEventRequest{
				RoomName:  roomName,
				Event:     "ready",
				Timestamp: time.Now().Unix(),
				Metadata: &_struct.Struct{
					Fields: map[string]*structpb.Value{
						"mockIdentifier": {
							Kind: &structpb.Value_StringValue{
								StringValue: "c50c9d8a-5a40-40ee-97ea-d477d7b0abd9",
							},
						},
					},
				},
			}
			roomEventResponse := &maestroApiV1.ForwardRoomEventResponse{}
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/roomevent", schedulerName, roomName), roomEventRequest, roomEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, roomEventResponse.Success)
		})

		t.Run("[Room event failure] Forward player event return success false when the forwarder connection can't be established", func(t *testing.T) {
			t.Parallel()

			schedulerName, roomName := createSchedulerWithForwarderAndRooms(t, maestro, kubeClient, managementApiClient, "invalid-grpc-address:9982")

			roomEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
				RoomName:  roomName,
				Event:     "occupied",
				Timestamp: time.Now().Unix(),
				Metadata: &_struct.Struct{
					Fields: map[string]*structpb.Value{
						"mockIdentifier": {
							Kind: &structpb.Value_StringValue{
								StringValue: "c50c9d8a-5a40-40ee-97ea-d477d7b0abd9",
							},
						},
					},
				},
			}
			roomEventResponse := &maestroApiV1.ForwardPlayerEventResponse{}
			err := roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/roomevent", schedulerName, roomName), roomEventRequest, roomEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, roomEventResponse.Success)
			require.Contains(t, roomEventResponse.Message, "transport: Error while dialing dial tcp: lookup invalid-grpc-address")
		})

		t.Run("[Ping event success] Ping event return success when no error occurs while forwarding events call", func(t *testing.T) {
			t.Parallel()

			schedulerName, roomName := createSchedulerWithForwarderAndRooms(t, maestro, kubeClient, managementApiClient, maestro.ServerMocks.GrpcForwarderAddress)

			// This configuration make the grpc service return with success
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-room-resync-success")
			require.NoError(t, err)

			pingRequest := &maestroApiV1.UpdateRoomStatusRequest{
				RoomName:  roomName,
				Status:    "terminating",
				Timestamp: time.Now().Unix(),
				Metadata: &_struct.Struct{
					Fields: map[string]*structpb.Value{
						"mockIdentifier": {
							Kind: &structpb.Value_StringValue{
								StringValue: "65e810a0-bb81-4633-93c9-826414a0062d",
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

			pingResponse := &maestroApiV1.UpdateRoomWithPingResponse{}
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/ping", schedulerName, roomName), pingRequest, pingResponse)
			require.NoError(t, err)
			require.Equal(t, true, pingResponse.Success)
		})

		t.Run("[Ping event success] Ping event return success when no forwarder is configured for the scheduler", func(t *testing.T) {
			t.Parallel()

			schedulerName, roomName := createSchedulerWithRooms(t, maestro, kubeClient, managementApiClient)

			pingRequest := &maestroApiV1.UpdateRoomStatusRequest{
				RoomName:  roomName,
				Status:    "ready",
				Timestamp: time.Now().Unix(),
				Metadata: &_struct.Struct{
					Fields: map[string]*structpb.Value{
						"mockIdentifier": {
							Kind: &structpb.Value_StringValue{
								StringValue: "8293bcb6-2c4b-4a85-8c43-0a0ea4c0d0b5",
							},
						},
					},
				},
			}
			pingResponse := &maestroApiV1.UpdateRoomWithPingResponse{}
			err := roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/ping", schedulerName, roomName), pingRequest, pingResponse)
			require.NoError(t, err)
			require.Equal(t, true, pingResponse.Success)
		})

		t.Run("[Ping event failures] Ping event return success when some error occurs in GRPC call", func(t *testing.T) {
			t.Parallel()

			schedulerName, roomName := createSchedulerWithForwarderAndRooms(t, maestro, kubeClient, managementApiClient, maestro.ServerMocks.GrpcForwarderAddress)

			// This configuration make the grpc service return with failure
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-room-resync-failure")
			require.NoError(t, err)

			pingRequest := &maestroApiV1.UpdateRoomStatusRequest{
				RoomName:  roomName,
				Status:    "terminating",
				Timestamp: time.Now().Unix(),
				Metadata: &_struct.Struct{
					Fields: map[string]*structpb.Value{
						"mockIdentifier": {
							Kind: &structpb.Value_StringValue{
								StringValue: "f7415c97-5c28-418b-b19b-87380e2d0113",
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
			pingResponse := &maestroApiV1.UpdateRoomWithPingResponse{}
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/ping", schedulerName, roomName), pingRequest, pingResponse)
			require.NoError(t, err)
			require.Equal(t, true, pingResponse.Success)
		})

		t.Run("[Ping event failure] Ping event return success when the forwarder connection can't be established", func(t *testing.T) {
			t.Parallel()

			schedulerName, roomName := createSchedulerWithForwarderAndRooms(t, maestro, kubeClient, managementApiClient, "invalid-grpc-address:9982")

			pingRequest := &maestroApiV1.UpdateRoomStatusRequest{
				RoomName:  roomName,
				Status:    "ready",
				Timestamp: time.Now().Unix(),
				Metadata: &_struct.Struct{
					Fields: map[string]*structpb.Value{
						"mockIdentifier": {
							Kind: &structpb.Value_StringValue{
								StringValue: "71dbc1fd-dd66-4b08-a0a5-7dd23288dd6a",
							},
						},
					},
				},
			}
			pingResponse := &maestroApiV1.UpdateRoomWithPingResponse{}
			err := roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/ping", schedulerName, roomName), pingRequest, pingResponse)
			require.NoError(t, err)
			require.Equal(t, true, pingResponse.Success)
		})
	})

}

func createSchedulerWithRooms(t *testing.T, maestro *maestro.MaestroInstance, kubeClient kubernetes.Interface, managementApiClient *framework.APIClient) (string, string) {
	scheduler, err := createSchedulerAndWaitForIt(t,
		maestro,
		managementApiClient,
		kubeClient,
		"test",
		[]string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request POST " +
			"$ROOMS_API_ADDRESS:9097/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
			"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 1; done"},
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
		return true
	}, 240*time.Second, time.Second)

	pods, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, pods.Items)

	return scheduler.Name, pods.Items[0].Name
}

func createSchedulerWithForwarderAndRooms(t *testing.T, maestro *maestro.MaestroInstance, kubeClient kubernetes.Interface, managementApiClient *framework.APIClient, forwarderAddress string) (string, string) {
	forwarders := []*maestroApiV1.Forwarder{
		{
			Name:    "matchmaker-grpc",
			Enable:  true,
			Type:    "grpc",
			Address: forwarderAddress,
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

	scheduler, err := createSchedulerWithForwardersAndWaitForIt(
		t,
		maestro,
		managementApiClient,
		kubeClient,
		[]string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request POST " +
			"$ROOMS_API_ADDRESS:9097/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
			"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 1; done"},
		forwarders,
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
		return true
	}, 240*time.Second, time.Second)

	pods, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, pods.Items)

	return scheduler.Name, pods.Items[0].Name
}
