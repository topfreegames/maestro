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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	mockeventsservice "github.com/topfreegames/maestro/internal/core/services/interfaces/mock/events_service"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports/errors"

	clock_mock "github.com/topfreegames/maestro/internal/adapters/clock/mock"

	"github.com/golang/mock/gomock"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	instance_storage_mock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	port_allocator_mock "github.com/topfreegames/maestro/internal/adapters/port_allocator/mock"
	runtime_mock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
	"github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func TestRoomsHandler_UpdateRoomWithPing(t *testing.T) {
	dirPath, _ := os.Getwd()

	instance := &game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceReady}}

	validRequests, _ := ioutil.ReadFile(dirPath + "/fixtures/request/valid-ping-data-list.json")
	var validRawRequests []*json.RawMessage
	err := json.Unmarshal(validRequests, &validRawRequests)
	require.NoError(t, err)

	invalidStateRequests, _ := ioutil.ReadFile(dirPath + "/fixtures/request/invalid-state-transition-ping-data.json")
	var invalidStateRawRequests []*json.RawMessage
	err = json.Unmarshal(invalidStateRequests, &invalidStateRawRequests)
	require.NoError(t, err)

	t.Run("with valid request and existent game room it should return status code 200 with success = true", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		clockMock := clock_mock.NewFakeClock(time.Now())
		portAllocatorMock := port_allocator_mock.NewMockPortAllocator(mockCtrl)
		roomStorageMock := mock.NewMockRoomStorage(mockCtrl)
		instanceStorageMock := instance_storage_mock.NewMockGameRoomInstanceStorage(mockCtrl)
		eventsForwarderService := mockeventsservice.NewMockEventsService(mockCtrl)
		runtimeMock := runtime_mock.NewMockRuntime(mockCtrl)
		config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}

		roomsManager := room_manager.New(clockMock, portAllocatorMock, roomStorageMock, instanceStorageMock, runtimeMock, eventsForwarderService, config)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		for _, validRawRequest := range validRawRequests {
			// TODO(gabrielcorado): since we're exposing the room manager
			// internals we have to do it to mock it properly
			var updatedGameRoom *game_room.GameRoom
			roomStorageMock.EXPECT().UpdateRoom(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, receivedGameRoom *game_room.GameRoom) error {
				receivedGameRoom.Status = game_room.GameStatusReady
				updatedGameRoom = receivedGameRoom
				return nil
			})

			instanceStorageMock.EXPECT().GetInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(instance, nil)
			roomStorageMock.EXPECT().GetRoom(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ string, _ string) (*game_room.GameRoom, error) {
				return updatedGameRoom, nil
			})
			roomStorageMock.EXPECT().UpdateRoomStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			eventsForwarderService.EXPECT().ProduceEvent(gomock.Any(), gomock.Any())

			request, err := validRawRequest.MarshalJSON()
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodPost, "/scheduler/scheduler-name-1/rooms/room-name-1/ping", bytes.NewReader(request))
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			require.Equal(t, 200, rr.Code)
			bodyString := rr.Body.String()
			require.Equal(t, "{\"success\":true}", bodyString)
		}
	})

	t.Run("with valid request and nonexistent game room then it should return with status code 404", func(t *testing.T) {
		// TODO(gabrielcorado): we're skipping this test since the update room
		// currently doesn't fail if the room doesn't exists.
		t.Skip()

		mockCtrl := gomock.NewController(t)

		clockMock := clock_mock.NewFakeClock(time.Now())
		portAllocatorMock := port_allocator_mock.NewMockPortAllocator(mockCtrl)
		roomStorageMock := mock.NewMockRoomStorage(mockCtrl)
		instanceStorageMock := instance_storage_mock.NewMockGameRoomInstanceStorage(mockCtrl)
		eventsForwarderService := mockeventsservice.NewMockEventsService(mockCtrl)
		runtimeMock := runtime_mock.NewMockRuntime(mockCtrl)
		config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}
		roomsManager := room_manager.New(clockMock, portAllocatorMock, roomStorageMock, instanceStorageMock, runtimeMock, eventsForwarderService, config)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		roomStorageMock.EXPECT().UpdateRoom(gomock.Any(), gomock.Any()).Return(errors.ErrNotFound)

		request, err := validRawRequests[0].MarshalJSON()
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/scheduler/scheduler-name-1/rooms/room-name-1/ping", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 404, rr.Code)
	})

	t.Run("with valid request when have error while updating game room then it should return with status code 500", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		clockMock := clock_mock.NewFakeClock(time.Now())
		portAllocatorMock := port_allocator_mock.NewMockPortAllocator(mockCtrl)
		roomStorageMock := mock.NewMockRoomStorage(mockCtrl)
		instanceStorageMock := instance_storage_mock.NewMockGameRoomInstanceStorage(mockCtrl)
		eventsForwarderService := mockeventsservice.NewMockEventsService(mockCtrl)
		runtimeMock := runtime_mock.NewMockRuntime(mockCtrl)
		config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}
		roomsManager := room_manager.New(clockMock, portAllocatorMock, roomStorageMock, instanceStorageMock, runtimeMock, eventsForwarderService, config)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		roomStorageMock.EXPECT().UpdateRoom(gomock.Any(), gomock.Any()).Return(errors.ErrUnexpected)

		request, err := validRawRequests[0].MarshalJSON()
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/scheduler/scheduler-name-1/rooms/room-name-1/ping", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 500, rr.Code)
	})

	t.Run("with valid request when the new game room state transition is invalid then it should return with status code 500", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		clockMock := clock_mock.NewFakeClock(time.Now())
		portAllocatorMock := port_allocator_mock.NewMockPortAllocator(mockCtrl)
		roomStorageMock := mock.NewMockRoomStorage(mockCtrl)
		instanceStorageMock := instance_storage_mock.NewMockGameRoomInstanceStorage(mockCtrl)
		eventsForwarderService := mockeventsservice.NewMockEventsService(mockCtrl)
		runtimeMock := runtime_mock.NewMockRuntime(mockCtrl)
		config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}
		roomsManager := room_manager.New(clockMock, portAllocatorMock, roomStorageMock, instanceStorageMock, runtimeMock, eventsForwarderService, config)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		for _, invalidStateRawRequest := range invalidStateRawRequests {
			// TODO(gabrielcorado): since we're exposing the room manager
			// internals we have to do it to mock it properly
			var updatedGameRoom *game_room.GameRoom
			roomStorageMock.EXPECT().UpdateRoom(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, receivedGameRoom *game_room.GameRoom) error {
				receivedGameRoom.Status = game_room.GameStatusTerminating
				updatedGameRoom = receivedGameRoom
				return nil
			})

			instanceStorageMock.EXPECT().GetInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(instance, nil)
			roomStorageMock.EXPECT().GetRoom(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ string, _ string) (*game_room.GameRoom, error) {
				return updatedGameRoom, nil
			})

			request, err := invalidStateRawRequest.MarshalJSON()
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodPost, "/scheduler/scheduler-name-1/rooms/room-name-1/ping", bytes.NewReader(request))
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			require.Equal(t, 500, rr.Code)
		}
	})

	t.Run("with invalid request then it should return with status code 400", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		clockMock := clock_mock.NewFakeClock(time.Now())
		portAllocatorMock := port_allocator_mock.NewMockPortAllocator(mockCtrl)
		roomStorageMock := mock.NewMockRoomStorage(mockCtrl)
		instanceStorageMock := instance_storage_mock.NewMockGameRoomInstanceStorage(mockCtrl)
		eventsForwarderService := mockeventsservice.NewMockEventsService(mockCtrl)
		runtimeMock := runtime_mock.NewMockRuntime(mockCtrl)
		config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}
		roomsManager := room_manager.New(clockMock, portAllocatorMock, roomStorageMock, instanceStorageMock, runtimeMock, eventsForwarderService, config)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		request, err := ioutil.ReadFile(dirPath + "/fixtures/request/bad-ping-data.json")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/scheduler/scheduler-name-1/rooms/room-name-1/ping", bytes.NewReader(request))
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

	t.Run("when no error occur when forwarding then it return status code 200 with success = true", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		clockMock := clock_mock.NewFakeClock(time.Now())
		portAllocatorMock := port_allocator_mock.NewMockPortAllocator(mockCtrl)
		roomStorageMock := mock.NewMockRoomStorage(mockCtrl)
		instanceStorageMock := instance_storage_mock.NewMockGameRoomInstanceStorage(mockCtrl)
		eventsForwarderService := mockeventsservice.NewMockEventsService(mockCtrl)
		runtimeMock := runtime_mock.NewMockRuntime(mockCtrl)
		config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}

		roomsManager := room_manager.New(clockMock, portAllocatorMock, roomStorageMock, instanceStorageMock, runtimeMock, eventsForwarderService, config)
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

		clockMock := clock_mock.NewFakeClock(time.Now())
		portAllocatorMock := port_allocator_mock.NewMockPortAllocator(mockCtrl)
		roomStorageMock := mock.NewMockRoomStorage(mockCtrl)
		instanceStorageMock := instance_storage_mock.NewMockGameRoomInstanceStorage(mockCtrl)
		eventsForwarderService := mockeventsservice.NewMockEventsService(mockCtrl)
		runtimeMock := runtime_mock.NewMockRuntime(mockCtrl)
		config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}

		roomsManager := room_manager.New(clockMock, portAllocatorMock, roomStorageMock, instanceStorageMock, runtimeMock, eventsForwarderService, config)
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

		clockMock := clock_mock.NewFakeClock(time.Now())
		portAllocatorMock := port_allocator_mock.NewMockPortAllocator(mockCtrl)
		roomStorageMock := mock.NewMockRoomStorage(mockCtrl)
		instanceStorageMock := instance_storage_mock.NewMockGameRoomInstanceStorage(mockCtrl)
		eventsForwarderService := mockeventsservice.NewMockEventsService(mockCtrl)
		runtimeMock := runtime_mock.NewMockRuntime(mockCtrl)
		config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}

		roomsManager := room_manager.New(clockMock, portAllocatorMock, roomStorageMock, instanceStorageMock, runtimeMock, eventsForwarderService, config)
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

		clockMock := clock_mock.NewFakeClock(time.Now())
		portAllocatorMock := port_allocator_mock.NewMockPortAllocator(mockCtrl)
		roomStorageMock := mock.NewMockRoomStorage(mockCtrl)
		instanceStorageMock := instance_storage_mock.NewMockGameRoomInstanceStorage(mockCtrl)
		eventsForwarderService := mockeventsservice.NewMockEventsService(mockCtrl)
		runtimeMock := runtime_mock.NewMockRuntime(mockCtrl)
		config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}

		roomsManager := room_manager.New(clockMock, portAllocatorMock, roomStorageMock, instanceStorageMock, runtimeMock, eventsForwarderService, config)
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

	t.Run("it does nothing and returns 200 ok with success equal true for all requests", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		clockMock := clock_mock.NewFakeClock(time.Now())
		portAllocatorMock := port_allocator_mock.NewMockPortAllocator(mockCtrl)
		roomStorageMock := mock.NewMockRoomStorage(mockCtrl)
		instanceStorageMock := instance_storage_mock.NewMockGameRoomInstanceStorage(mockCtrl)
		eventsForwarderService := mockeventsservice.NewMockEventsService(mockCtrl)
		runtimeMock := runtime_mock.NewMockRuntime(mockCtrl)
		config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}

		roomsManager := room_manager.New(clockMock, portAllocatorMock, roomStorageMock, instanceStorageMock, runtimeMock, eventsForwarderService, config)
		mux := runtime.NewServeMux()
		err := api.RegisterRoomsServiceHandlerServer(context.Background(), mux, ProvideRoomsHandler(roomsManager, eventsForwarderService))
		require.NoError(t, err)

		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPut, "/scheduler/schedulerName/rooms/roomName/status", bytes.NewReader([]byte{}))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)
		require.NoError(t, err)
		require.Equal(t, true, body["success"])
	})
}
