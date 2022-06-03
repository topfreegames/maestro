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

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	_struct "github.com/golang/protobuf/ptypes/struct"
	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/core/entities/events"
	"github.com/topfreegames/maestro/internal/core/ports"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/topfreegames/maestro/internal/core/ports/errors"

	"github.com/golang/mock/gomock"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func TestRoomsHandler_UpdateRoomWithPing(t *testing.T) {
	dirPath, _ := os.Getwd()

	validRequests, _ := ioutil.ReadFile(dirPath + "/fixtures/request/valid-ping-data-list.json")
	var validRawRequests []*json.RawMessage
	err := json.Unmarshal(validRequests, &validRawRequests)
	require.NoError(t, err)

	invalidStateRequests, _ := ioutil.ReadFile(dirPath + "/fixtures/request/invalid-state-transition-ping-data.json")
	var invalidStateRawRequests []*json.RawMessage
	err = json.Unmarshal(invalidStateRequests, &invalidStateRawRequests)
	require.NoError(t, err)

	t.Run("should succeed - valid request, existent game room => return status code 200", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		eventsForwarderService := mockports.NewMockEventsService(mockCtrl)

		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		for _, validRawRequest := range validRawRequests {
			roomsManager.EXPECT().UpdateRoom(gomock.Any(), gomock.Any()).Return(nil)

			request, err := validRawRequest.MarshalJSON()
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodPut, "/scheduler/scheduler-name-1/rooms/room-name-1/ping", bytes.NewReader(request))
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			require.Equal(t, 200, rr.Code)
			bodyString := rr.Body.String()
			require.Equal(t, "{\"success\":true}", bodyString)
		}
	})

	t.Run("should fail - valid request, non-existent game room => return status code 404", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)

		eventsForwarderService := mockports.NewMockEventsService(mockCtrl)

		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		roomsManager.EXPECT().UpdateRoom(gomock.Any(), gomock.Any()).Return(errors.NewErrNotFound("NOT FOUND"))

		request, err := validRawRequests[0].MarshalJSON()
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPut, "/scheduler/scheduler-name-1/rooms/room-name-1/ping", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 404, rr.Code)
	})

	t.Run("should fail - valid request, error while updating game room => return status code 500", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		eventsForwarderService := mockports.NewMockEventsService(mockCtrl)

		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		roomsManager.EXPECT().UpdateRoom(gomock.Any(), gomock.Any()).Return(errors.ErrUnexpected)

		request, err := validRawRequests[0].MarshalJSON()
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPut, "/scheduler/scheduler-name-1/rooms/room-name-1/ping", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 500, rr.Code)
	})

	t.Run("should fail - invalid request => return status code 400", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		eventsForwarderService := mockports.NewMockEventsService(mockCtrl)

		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		request, err := ioutil.ReadFile(dirPath + "/fixtures/request/bad-ping-data.json")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPut, "/scheduler/scheduler-name-1/rooms/room-name-1/ping", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 400, rr.Code)
	})
}

func TestRoomsHandler_ForwardRoomEvent(t *testing.T) {
	dirPath, _ := os.Getwd()

	requests, _ := ioutil.ReadFile(dirPath + "/fixtures/request/room-events.json")
	var rawRequests []*json.RawMessage
	err := json.Unmarshal(requests, &rawRequests)
	require.NoError(t, err)

	t.Run("should succeed - no error occur when forwarding => returns status code 200", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		eventsForwarderService := mockports.NewMockEventsService(mockCtrl)

		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		for _, rawRequest := range rawRequests {
			eventsForwarderService.EXPECT().ProduceEvent(gomock.Any(), gomock.Any())
			request, err := rawRequest.MarshalJSON()
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodPost, "/scheduler/schedulerName1/rooms/roomName1/roomevent", bytes.NewReader(request))
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			require.Equal(t, 200, rr.Code)
			bodyString := rr.Body.String()
			var body map[string]interface{}
			err = json.Unmarshal([]byte(bodyString), &body)
			require.NoError(t, err)
			require.Equal(t, true, body["success"])
			require.Equal(t, "", body["message"])

		}
	})

	t.Run("when some error occur when forwarding then it return status code 200 with success = false", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		eventsForwarderService := mockports.NewMockEventsService(mockCtrl)

		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		for _, rawRequest := range rawRequests {
			eventsForwarderService.EXPECT().ProduceEvent(gomock.Any(), gomock.Any()).Return(errors.NewErrUnexpected("Failed to forward room events"))
			request, err := rawRequest.MarshalJSON()
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodPost, "/scheduler/schedulerName1/rooms/roomName1/roomevent", bytes.NewReader(request))
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			require.Equal(t, 200, rr.Code)
			bodyString := rr.Body.String()
			var body map[string]interface{}
			err = json.Unmarshal([]byte(bodyString), &body)
			require.NoError(t, err)
			require.Equal(t, false, body["success"])
		}
	})
}

func TestRoomsHandler_ForwardPlayerEvent(t *testing.T) {
	dirPath, _ := os.Getwd()

	requests, _ := ioutil.ReadFile(dirPath + "/fixtures/request/player-events.json")
	var rawRequests []*json.RawMessage
	err := json.Unmarshal(requests, &rawRequests)
	require.NoError(t, err)

	t.Run("when no error occur when forwarding then it return status code 200 with success = true", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		eventsForwarderService := mockports.NewMockEventsService(mockCtrl)

		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		for _, rawRequest := range rawRequests {
			eventsForwarderService.EXPECT().ProduceEvent(gomock.Any(), gomock.Any()).Return(nil)
			request, err := rawRequest.MarshalJSON()
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodPost, "/scheduler/schedulerName1/rooms/roomName1/playerevent", bytes.NewReader(request))
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			require.Equal(t, 200, rr.Code)
			bodyString := rr.Body.String()
			var body map[string]interface{}
			err = json.Unmarshal([]byte(bodyString), &body)
			require.NoError(t, err)
			require.Equal(t, true, body["success"])
			require.Equal(t, "", body["message"])
		}
	})

	t.Run("when some error occur when forwarding then it return status code 200 with success = false", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		eventsForwarderService := mockports.NewMockEventsService(mockCtrl)

		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		for _, rawRequest := range rawRequests {
			eventsForwarderService.EXPECT().ProduceEvent(gomock.Any(), gomock.Any()).Return(errors.NewErrUnexpected("Failed to forward player events"))
			request, err := rawRequest.MarshalJSON()
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodPost, "/scheduler/schedulerName1/rooms/roomName1/playerevent", bytes.NewReader(request))
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			require.Equal(t, 200, rr.Code)
			bodyString := rr.Body.String()
			var body map[string]interface{}
			err = json.Unmarshal([]byte(bodyString), &body)
			require.NoError(t, err)
			require.Equal(t, false, body["success"])
		}
	})
}

func TestRoomsHandler_UpdateRoomStatus(t *testing.T) {
	type args struct {
		ctx     context.Context
		message *api.UpdateRoomStatusRequest
	}

	type mockPreparation func(controller *gomock.Controller) (ports.RoomManager, ports.EventsService)
	tests := []struct {
		name            string
		mockPreparation mockPreparation
		args            args
		responseWanted  *api.UpdateRoomStatusResponse
		errWanted       error
	}{
		{
			"return success when no error occurs",
			func(controller *gomock.Controller) (ports.RoomManager, ports.EventsService) {
				roomManagerMock := mockports.NewMockRoomManager(controller)
				eventsServiceMock := mockports.NewMockEventsService(controller)
				requiredEvent := &events.Event{
					Name:        "RoomEvent",
					SchedulerID: "scheduler-name",
					RoomID:      "room-name",
					Attributes: map[string]interface{}{
						"eventType": "status",
						"pingType":  "ready",
						"roomType":  "red",
					},
				}
				eventsServiceMock.EXPECT().ProduceEvent(gomock.Any(), requiredEvent)
				return roomManagerMock, eventsServiceMock
			},
			args{
				ctx: context.Background(),
				message: &api.UpdateRoomStatusRequest{
					SchedulerName: "scheduler-name",
					RoomName:      "room-name",
					Metadata: &_struct.Struct{
						Fields: map[string]*structpb.Value{
							"roomType": {
								Kind: &structpb.Value_StringValue{
									StringValue: "red",
								},
							},
						},
					},
					Status:    "ready",
					Timestamp: 0,
				},
			},
			&api.UpdateRoomStatusResponse{Success: true},
			nil,
		},
		{
			"return no error and success false when some error occurs producing the event",
			func(controller *gomock.Controller) (ports.RoomManager, ports.EventsService) {
				roomManagerMock := mockports.NewMockRoomManager(controller)
				eventsServiceMock := mockports.NewMockEventsService(controller)
				requiredEvent := &events.Event{
					Name:        "RoomEvent",
					SchedulerID: "scheduler-name",
					RoomID:      "room-name",
					Attributes: map[string]interface{}{
						"eventType": "status",
						"pingType":  "ready",
						"roomType":  "red",
					},
				}
				eventsServiceMock.EXPECT().ProduceEvent(gomock.Any(), requiredEvent).Return(errors.NewErrUnexpected("some error"))
				return roomManagerMock, eventsServiceMock
			},
			args{
				ctx: context.Background(),
				message: &api.UpdateRoomStatusRequest{
					SchedulerName: "scheduler-name",
					RoomName:      "room-name",
					Metadata: &_struct.Struct{
						Fields: map[string]*structpb.Value{
							"roomType": {
								Kind: &structpb.Value_StringValue{
									StringValue: "red",
								},
							},
						},
					},
					Status:    "ready",
					Timestamp: 0,
				},
			},
			&api.UpdateRoomStatusResponse{Success: false},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockRoomManager, mockEventsService := tt.mockPreparation(mockCtrl)
			h := &RoomsHandler{
				roomManager:   mockRoomManager,
				eventsService: mockEventsService,
				logger:        zap.L(),
			}
			got, err := h.UpdateRoomStatus(tt.args.ctx, tt.args.message)
			if tt.errWanted != nil {
				assert.EqualError(t, err, tt.errWanted.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.responseWanted, got)
			}
		})
	}
}

func TestRoomsHandler_GetRoomAddress(t *testing.T) {

	t.Run("return the room address and status 200 when no error occurs", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		eventsForwarderService := mockports.NewMockEventsService(mockCtrl)
		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		roomsManager.EXPECT().GetRoomInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(&game_room.Instance{Address: &game_room.Address{
			Host:  "room-host",
			Ports: []game_room.Port{game_room.Port{Name: "port-name", Port: 8080, Protocol: "tcp"}},
		}}, nil)

		req, err := http.NewRequest(http.MethodGet, "/scheduler/schedulerName1/rooms/roomName1/address", bytes.NewReader([]byte{}))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		responseBody, expectedResponseBody := extractBodyForComparisons(t, rr.Body.Bytes(), "rooms_handler/get-room-address-success.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("return code 500 when some occurs", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		eventsForwarderService := mockports.NewMockEventsService(mockCtrl)
		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		roomsManager.EXPECT().GetRoomInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.NewErrUnexpected("some error"))

		req, err := http.NewRequest(http.MethodGet, "/scheduler/schedulerName1/rooms/roomName1/address", bytes.NewReader([]byte{}))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code)

		responseBody, expectedResponseBody := extractBodyForComparisons(t, rr.Body.Bytes(), "rooms_handler/get-room-address-error.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

}

func extractBodyForComparisons(t *testing.T, body []byte, expectedBodyFixturePath string) (string, string) {
	dirPath, err := os.Getwd()
	require.NoError(t, err)
	fixture, err := ioutil.ReadFile(fmt.Sprintf("%s/fixtures/response/%s", dirPath, expectedBodyFixturePath))
	require.NoError(t, err)
	bodyBuffer := new(bytes.Buffer)
	expectedBodyBuffer := new(bytes.Buffer)
	err = json.Compact(bodyBuffer, body)
	require.NoError(t, err)
	err = json.Compact(expectedBodyBuffer, fixture)
	require.NoError(t, err)
	return bodyBuffer.String(), expectedBodyBuffer.String()
}
