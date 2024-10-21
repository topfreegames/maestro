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

package suites

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework"
	"github.com/topfreegames/maestro/e2e/framework/maestro"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	"google.golang.org/protobuf/types/known/structpb"
	k8sCoreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func TestEventsForwarding(t *testing.T) {
	t.Parallel()

	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		schedulerWithForwarderAndRooms, roomsNames := createSchedulerWithForwarderAndRooms(t, maestro, kubeClient, managementApiClient, maestro.ServerMocks.GrpcForwarderAddress)
		schedulerWithRooms, err := createSchedulerWithRoomsAndWaitForIt(t, maestro, managementApiClient, "test-events-forwarding-game", kubeClient)
		require.NoError(t, err)
		var roomNameNoForwarder string
		require.Eventually(t, func() bool {
			pods, err := kubeClient.CoreV1().Pods(schedulerWithRooms.Name).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, pods.Items)

			if pods.Items[0].Status.Phase != v1.PodRunning {
				return false
			}

			roomNameNoForwarder = pods.Items[0].Name
			return true
		}, time.Minute, 10*time.Millisecond)

		schedulerWithInvalidGrpc, invalidGrpcRooms := createSchedulerWithForwarderAndRooms(t, maestro, kubeClient, managementApiClient, "invalid-grpc-address:9982")

		t.Run("[Player event success] Forward player event return success true when no error occurs while forwarding events call", func(t *testing.T) {
			t.Parallel()
			// This configuration make the grpc service return with success
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-player-event-success")
			require.NoError(t, err)

			playerEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
				RoomName:  roomsNames[0],
				Event:     "playerLeft",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{
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
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/playerevent", schedulerWithForwarderAndRooms.Name, roomsNames[0]), playerEventRequest, playerEventResponse)
			require.NoError(t, err)
			require.Equal(t, true, playerEventResponse.Success)
		})

		t.Run("[Player event success] Forward player event return success true when no forwarder is configured for the scheduler", func(t *testing.T) {
			t.Parallel()

			playerEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
				RoomName:  roomNameNoForwarder,
				Event:     "playerLeft",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{
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
			err := roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/playerevent", schedulerWithRooms.Name, roomNameNoForwarder), playerEventRequest, playerEventResponse)
			require.NoError(t, err)
			require.Equal(t, true, playerEventResponse.Success)
		})

		t.Run("[Player event failure] Forward player event return success false when some error occurs in GRPC call", func(t *testing.T) {
			t.Parallel()

			// This configuration make the grpc service return with error
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-player-event-failure")

			playerEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
				RoomName:  roomsNames[0],
				Event:     "playerLeft",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{

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
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/playerevent", schedulerWithForwarderAndRooms.Name, roomsNames[0]), playerEventRequest, playerEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, playerEventResponse.Success)
			require.Equal(t, "failed to forward event room at \"matchmaker-grpc\"", playerEventResponse.Message)
		})

		t.Run("[Player event failure] Forward player event return success false when forwarding event for an inexistent room", func(t *testing.T) {
			t.Parallel()

			inexistentRoom := "inexistent-room"

			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-player-event-failure")

			playerEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
				RoomName:  roomNameNoForwarder,
				Event:     "playerLeft",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{
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
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/playerevent", schedulerWithForwarderAndRooms.Name, inexistentRoom), playerEventRequest, playerEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, playerEventResponse.Success)
		})

		t.Run("[Player event failure] Forward player event return success false when the forwarder connection can't be established", func(t *testing.T) {
			t.Parallel()

			playerEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
				RoomName:  roomsNames[0],
				Event:     "playerLeft",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{
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
			err := roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/playerevent", schedulerWithInvalidGrpc.Name, invalidGrpcRooms[0]), playerEventRequest, playerEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, playerEventResponse.Success)
			require.Contains(t, playerEventResponse.Message, "transport: Error while dialing dial tcp: lookup invalid-grpc-address")
		})

		t.Run("[Room event success] Forward room event return success true when no error occurs while forwarding events call", func(t *testing.T) {
			t.Parallel()

			// This configuration make the grpc service return with success
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-room-event-success")
			require.NoError(t, err)

			roomEventRequest := &maestroApiV1.ForwardRoomEventRequest{
				RoomName:  roomsNames[0],
				Event:     "roomReady",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{
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
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/roomevent", schedulerWithForwarderAndRooms.Name, roomsNames[0]), roomEventRequest, roomEventResponse)
			require.NoError(t, err)
			require.Equal(t, true, roomEventResponse.Success)
		})

		t.Run("[Room event success] Forward room event return success true when no forwarder is configured for the scheduler", func(t *testing.T) {
			t.Parallel()

			roomEventRequest := &maestroApiV1.ForwardRoomEventRequest{
				RoomName:  roomNameNoForwarder,
				Event:     "ready",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{
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
			err := roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/roomevent", schedulerWithRooms.Name, roomNameNoForwarder), roomEventRequest, roomEventResponse)
			require.NoError(t, err)
			require.Equal(t, true, roomEventResponse.Success)
		})

		t.Run("[Room event failure] Forward room event return success false when some error occurs in GRPC call", func(t *testing.T) {
			t.Parallel()

			// This configuration make the grpc service return with failure
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-room-event-failure")
			require.NoError(t, err)

			roomEventRequest := &maestroApiV1.ForwardRoomEventRequest{
				RoomName:  roomsNames[0],
				Event:     "roomReady",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{
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
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/roomevent", schedulerWithForwarderAndRooms.Name, roomsNames[0]), roomEventRequest, roomEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, roomEventResponse.Success)
			require.Equal(t, "failed to forward event room at \"matchmaker-grpc\"", roomEventResponse.Message)
		})

		t.Run("[Room event failure] Forward room event return success false when forwarding event for an inexistent room", func(t *testing.T) {
			t.Parallel()
			inexistentRoom := "inexistent-room"

			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-player-event-failure")

			roomEventRequest := &maestroApiV1.ForwardRoomEventRequest{
				RoomName:  roomNameNoForwarder,
				Event:     "ready",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{
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
			err = roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/roomevent", schedulerWithForwarderAndRooms.Name, inexistentRoom), roomEventRequest, roomEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, roomEventResponse.Success)
		})

		t.Run("[Room event failure] Forward room event return success false when the forwarder connection can't be established", func(t *testing.T) {
			t.Parallel()

			roomEventRequest := &maestroApiV1.ForwardPlayerEventRequest{
				RoomName:  roomsNames[0],
				Event:     "occupied",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{
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
			err := roomsApiClient.Do("POST", fmt.Sprintf("/scheduler/%s/rooms/%s/roomevent", schedulerWithInvalidGrpc.Name, invalidGrpcRooms[0]), roomEventRequest, roomEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, roomEventResponse.Success)
		})

		t.Run("[Ping event success] Ping event return success when no error occurs while forwarding events call", func(t *testing.T) {
			t.Parallel()

			// This configuration make the grpc service return with success
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-room-resync-success")
			require.NoError(t, err)

			pingRequest := &maestroApiV1.UpdateRoomStatusRequest{
				RoomName:  roomsNames[0],
				Status:    "ready",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{
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
			err = roomsApiClient.Do("PUT", fmt.Sprintf("/scheduler/%s/rooms/%s/ping", schedulerWithForwarderAndRooms.Name, roomsNames[0]), pingRequest, pingResponse)
			require.NoError(t, err)
			require.Equal(t, true, pingResponse.Success)
		})

		t.Run("[Ping event success] Ping event return success when no forwarder is configured for the scheduler", func(t *testing.T) {
			t.Parallel()
			pingRequest := &maestroApiV1.UpdateRoomStatusRequest{
				RoomName:  roomNameNoForwarder,
				Status:    "ready",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{
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
			err := roomsApiClient.Do("PUT", fmt.Sprintf("/scheduler/%s/rooms/%s/ping", schedulerWithRooms.Name, roomNameNoForwarder), pingRequest, pingResponse)
			require.NoError(t, err)
			require.Equal(t, true, pingResponse.Success)
		})

		t.Run("[Ping event failure] Ping event return success when some error occurs in GRPC call", func(t *testing.T) {
			t.Parallel()

			// This configuration make the grpc service return with failure
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-room-resync-failure")
			require.NoError(t, err)

			pingRequest := &maestroApiV1.UpdateRoomStatusRequest{
				RoomName:  roomsNames[0],
				Status:    "ready",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{
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
			err = roomsApiClient.Do("PUT", fmt.Sprintf("/scheduler/%s/rooms/%s/ping", schedulerWithForwarderAndRooms.Name, roomsNames[0]), pingRequest, pingResponse)
			require.NoError(t, err)
			require.Equal(t, true, pingResponse.Success)
		})

		t.Run("[Ping event failure] Ping event return success when the forwarder connection can't be established", func(t *testing.T) {
			t.Parallel()

			pingRequest := &maestroApiV1.UpdateRoomStatusRequest{
				RoomName:  roomsNames[0],
				Status:    "ready",
				Timestamp: time.Now().Unix(),
				Metadata: &structpb.Struct{
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
			err := roomsApiClient.Do("PUT", fmt.Sprintf("/scheduler/%s/rooms/%s/ping", schedulerWithInvalidGrpc.Name, invalidGrpcRooms[0]), pingRequest, pingResponse)
			require.NoError(t, err)
			require.Equal(t, true, pingResponse.Success)
		})

		t.Run("[Room status event success] Forward room status event return success true when no error occurs while forwarding events call", func(t *testing.T) {
			t.Parallel()

			// This configuration make the grpc service return with success
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-room-status-event-success")
			require.NoError(t, err)

			roomStatusEventRequest := &maestroApiV1.UpdateRoomStatusRequest{
				Status: "ready",
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"mockIdentifier": {
							Kind: &structpb.Value_StringValue{
								StringValue: "9da046b8-3ef9-4f28-966d-d5c0cd1fe627",
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
				Timestamp: time.Now().Unix(),
			}
			roomStatusEventResponse := &maestroApiV1.UpdateRoomStatusResponse{}
			err = roomsApiClient.Do("PUT", fmt.Sprintf("/scheduler/%s/rooms/%s/status", schedulerWithForwarderAndRooms.Name, roomsNames[0]), roomStatusEventRequest, roomStatusEventResponse)
			require.NoError(t, err)
			require.Equal(t, true, roomStatusEventResponse.Success)
		})

		t.Run("[Room status event success] Forward room status event return success true when no forwarder is configured for the scheduler", func(t *testing.T) {
			t.Parallel()

			roomStatusEventRequest := &maestroApiV1.UpdateRoomStatusRequest{
				Status:    "ready",
				Timestamp: time.Now().Unix(),
			}
			roomStatusEventResponse := &maestroApiV1.UpdateRoomStatusResponse{}
			err := roomsApiClient.Do("PUT", fmt.Sprintf("/scheduler/%s/rooms/%s/status", schedulerWithRooms.Name, roomNameNoForwarder), roomStatusEventRequest, roomStatusEventResponse)
			require.NoError(t, err)
			require.Equal(t, true, roomStatusEventResponse.Success)
		})

		t.Run("[Room status event failure] Forward room status event return success false when some error occurs in GRPC call", func(t *testing.T) {
			t.Parallel()

			// This configuration make the grpc service return with failure
			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-room-event-failure")
			require.NoError(t, err)

			roomStatusEventRequest := &maestroApiV1.UpdateRoomStatusRequest{
				Status: "ready",
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"mockIdentifier": {
							Kind: &structpb.Value_StringValue{
								StringValue: "24db7925-fdc9-45ef-8b87-f1fff6a9eaaa",
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
				Timestamp: time.Now().Unix(),
			}
			roomStatusEventResponse := &maestroApiV1.UpdateRoomStatusResponse{}
			err = roomsApiClient.Do("PUT", fmt.Sprintf("/scheduler/%s/rooms/%s/status", schedulerWithForwarderAndRooms.Name, roomsNames[0]), roomStatusEventRequest, roomStatusEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, roomStatusEventResponse.Success)
		})

		t.Run("[Room status event failure] Forward room status event return success false when forwarding event for an inexistent room", func(t *testing.T) {
			t.Parallel()
			inexistentRoom := "inexistent-room"

			err := addStubRequestToMockedGrpcServer("events-forwarder-grpc-send-player-event-failure")

			roomStatusEventRequest := &maestroApiV1.UpdateRoomStatusRequest{
				Status: "ready",
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"mockIdentifier": {
							Kind: &structpb.Value_StringValue{
								StringValue: "9da046b8-3ef9-4f28-966d-d5c0cd1fe627",
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
				Timestamp: time.Now().Unix(),
			}
			roomStatusEventResponse := &maestroApiV1.UpdateRoomStatusResponse{}
			err = roomsApiClient.Do("PUT", fmt.Sprintf("/scheduler/%s/rooms/%s/status", schedulerWithForwarderAndRooms.Name, inexistentRoom), roomStatusEventRequest, roomStatusEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, roomStatusEventResponse.Success)
		})

		t.Run("[Room status event failure] Forward room status event return success false when the forwarder connection can't be established", func(t *testing.T) {
			t.Parallel()

			roomStatusEventRequest := &maestroApiV1.UpdateRoomStatusRequest{
				Status: "ready",
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"mockIdentifier": {
							Kind: &structpb.Value_StringValue{
								StringValue: "9da046b8-3ef9-4f28-966d-d5c0cd1fe627",
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
				Timestamp: time.Now().Unix(),
			}
			roomStatusEventResponse := &maestroApiV1.UpdateRoomStatusResponse{}
			err := roomsApiClient.Do("PUT", fmt.Sprintf("/scheduler/%s/rooms/%s/status", schedulerWithInvalidGrpc.Name, invalidGrpcRooms[0]), roomStatusEventRequest, roomStatusEventResponse)
			require.NoError(t, err)
			require.Equal(t, false, roomStatusEventResponse.Success)
		})

	})

}

func createSchedulerWithForwarderAndRooms(t *testing.T, maestro *maestro.MaestroInstance, kubeClient kubernetes.Interface, managementApiClient *framework.APIClient, forwarderAddress string) (*maestroApiV1.Scheduler, []string) {
	forwarders := []*maestroApiV1.Forwarder{
		{
			Name:    "matchmaker-grpc",
			Enable:  true,
			Type:    "gRPC",
			Address: forwarderAddress,
			Options: &maestroApiV1.ForwarderOptions{
				Timeout: 5000,
				Metadata: &structpb.Struct{
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
		[]string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request PUT " +
			"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
			"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 1; done"},
		forwarders,
	)
	require.NoError(t, err)

	var pods *k8sCoreV1.PodList
	require.Eventually(t, func() bool {
		pods, err = kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)

		return len(pods.Items) == 2
	}, 30*time.Second, time.Second)

	return scheduler, []string{pods.Items[0].Name, pods.Items[1].Name}
}
