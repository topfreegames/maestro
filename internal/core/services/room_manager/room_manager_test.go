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

//+build unit

package room_manager

import (
	"context"
	"errors"
	"testing"
	"time"

	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/golang/mock/gomock"

	clockmock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	ismock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	pamock "github.com/topfreegames/maestro/internal/adapters/port_allocator/mock"
	rsmock "github.com/topfreegames/maestro/internal/adapters/room_storage/mock"
	runtimemock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
)

func TestRoomManager_CreateRoom(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()
	portAllocator := pamock.NewMockPortAllocator(mockCtrl)
	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	fakeClock := clockmock.NewFakeClock(now)
	config := RoomManagerConfig{RoomInitializationTimeoutMillis: time.Millisecond * 1000}
	roomManager := NewRoomManager(fakeClock, portAllocator, roomStorage, instanceStorage, runtime, config)
	roomStorageStatusWatcher := rsmock.NewMockRoomStorageStatusWatcher(mockCtrl)

	container1 := game_room.Container{
		Name: "container1",
		Ports: []game_room.ContainerPort{
			{Protocol: "tcp"},
		},
	}

	container2 := game_room.Container{
		Name: "container2",
		Ports: []game_room.ContainerPort{
			{Protocol: "udp"},
		},
	}

	scheduler := entities.Scheduler{
		Name: "game",
		Spec: game_room.Spec{
			Containers: []game_room.Container{container1, container2},
		},
		PortRange: nil,
	}

	gameRoom := game_room.GameRoom{
		ID:          "game-1",
		SchedulerID: "game",
		Status:      game_room.GameStatusPending,
		LastPingAt:  now,
	}

	gameRoomInstance := game_room.Instance{
		ID:          "game-1",
		SchedulerID: "game",
		Version:     "1",
	}

	t.Run("when room creation is successful then it returns the game room and instance", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
		runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.Name, game_room.Spec{
			Containers: []game_room.Container{container1, container2},
		}).Return(&gameRoomInstance, nil)

		gameRoomReady := gameRoom
		gameRoomReady.Status = game_room.GameStatusReady

		roomStorage.EXPECT().CreateRoom(context.Background(), &gameRoom)
		roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoom.SchedulerID, gameRoom.ID).Return(&gameRoomReady, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), &gameRoom).Return(roomStorageStatusWatcher, nil)

		roomStorageStatusWatcher.EXPECT().Stop()

		room, instance, err := roomManager.CreateRoom(context.Background(), scheduler)
		require.NoError(t, err)
		require.Equal(t, &gameRoom, room)
		require.Equal(t, &gameRoomInstance, instance)
	})

	t.Run("when game room creation fails with initialization timeout then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
		runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.Name, game_room.Spec{
			Containers: []game_room.Container{container1, container2},
		}).Return(&gameRoomInstance, nil)

		roomStorage.EXPECT().CreateRoom(context.Background(), &gameRoom)
		roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoom.SchedulerID, gameRoom.ID).Return(&gameRoom, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), &gameRoom).Return(roomStorageStatusWatcher, nil)

		roomStorageStatusWatcher.EXPECT().Stop()
		roomStorageStatusWatcher.EXPECT().ResultChan()

		room, instance, err := roomManager.CreateRoom(context.Background(), scheduler)
		require.Error(t, err)
		require.Nil(t, room)
		require.Nil(t, instance)
	})

	t.Run("when game room creation fails while creating instance then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
		runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.Name, game_room.Spec{
			Containers: []game_room.Container{container1, container2},
		}).Return(nil, errors.NewErrUnexpected("error create game room instance"))

		room, instance, err := roomManager.CreateRoom(context.Background(), scheduler)
		require.Error(t, err)
		require.Nil(t, room)
		require.Nil(t, instance)
	})

	t.Run("when game room creation fails while creating game room on storage then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
		runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.Name, game_room.Spec{
			Containers: []game_room.Container{container1, container2},
		}).Return(&gameRoomInstance, nil)

		roomStorage.EXPECT().CreateRoom(context.Background(), &gameRoom).Return(errors.NewErrUnexpected("error storing room on redis"))

		room, instance, err := roomManager.CreateRoom(context.Background(), scheduler)
		require.Error(t, err)
		require.Nil(t, room)
		require.Nil(t, instance)
	})

	t.Run("when game room creation fails while allocating ports then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return(nil, errors.NewErrInvalidArgument("not enough ports to allocate"))

		room, instance, err := roomManager.CreateRoom(context.Background(), scheduler)
		require.Error(t, err)
		require.Nil(t, room)
		require.Nil(t, instance)
	})

}

func TestRoomManager_DeleteRoom(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	config := RoomManagerConfig{RoomInitializationTimeoutMillis: time.Millisecond * 1000}

	roomManager := NewRoomManager(
		clockmock.NewFakeClock(time.Now()),
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
		config,
	)

	t.Run("when the game room status transition is valid then it deletes the game room and update its status", func(t *testing.T) {
		nextStatus := game_room.GameStatusTerminating
		gameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusReady}
		newGameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: nextStatus}
		instance := &game_room.Instance{ID: "test-instance"}
		instanceStorage.EXPECT().GetInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(instance, nil)
		roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoom).Return(nil)
		runtime.EXPECT().DeleteGameRoomInstance(context.Background(), instance).Return(nil)

		err := roomManager.DeleteRoom(context.Background(), gameRoom)
		require.NoError(t, err)
	})

	t.Run("when the game room status transition is invalid then it deletes the game room and returns with proper error", func(t *testing.T) {
		gameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusTerminating}
		instance := &game_room.Instance{ID: "test-instance"}
		instanceStorage.EXPECT().GetInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(instance, nil)
		runtime.EXPECT().DeleteGameRoomInstance(context.Background(), instance).Return(nil)

		err := roomManager.DeleteRoom(context.Background(), gameRoom)
		require.Error(t, err)
	})
}

func TestRoomManager_UpdateRoom(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	clock := clockmock.NewFakeClock(time.Now())
	config := RoomManagerConfig{RoomInitializationTimeoutMillis: time.Millisecond * 1000}
	roomManager := NewRoomManager(
		clock,
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
		config,
	)
	currentGameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusReady}
	newGameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusOccupied, LastPingAt: clock.Now()}

	t.Run("when the current game room exists then it execute without returning error", func(t *testing.T) {
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(currentGameRoom, nil)
		roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoom).Return(nil)

		err := roomManager.UpdateRoom(context.Background(), newGameRoom)

		require.NoError(t, err)
	})

	t.Run("when the current game room does not exist then it returns proper error", func(t *testing.T) {
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(nil, porterrors.ErrUnexpected)

		err := roomManager.UpdateRoom(context.Background(), newGameRoom)

		require.Error(t, err)
	})

	t.Run("when there is some error while updating the room then it returns proper error", func(t *testing.T) {
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(currentGameRoom, nil)
		roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoom).Return(porterrors.ErrUnexpected)

		err := roomManager.UpdateRoom(context.Background(), newGameRoom)

		require.Error(t, err)
	})

	t.Run("when the game room state transition is invalid then it returns proper error", func(t *testing.T) {
		newGameRoomInvalidState := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusPending}
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoomInvalidState.SchedulerID, newGameRoomInvalidState.ID).Return(currentGameRoom, nil)

		err := roomManager.UpdateRoom(context.Background(), newGameRoomInvalidState)

		require.Error(t, err)
	})

}

func TestRoomManager_ListRoomsWithDeletionPriority(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	clock := clockmock.NewFakeClock(time.Now())
	roomManager := NewRoomManager(
		clock,
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
	)

	t.Run("when there enough rooms it should return the specified number", func(t *testing.T) {
		schedulerName := "test-scheduler"
		availableRooms := []*game_room.GameRoom{
			{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
			{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
			{ID: "third-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
			{ID: "forth-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
		}

		roomStorage.EXPECT().GetAllRoomIDs(context.Background(), schedulerName).Return(
			[]string{
				availableRooms[0].ID,
				availableRooms[1].ID,
				availableRooms[2].ID,
				availableRooms[3].ID,
			},
			nil,
		)
		roomStorage.EXPECT().GetRoom(context.Background(), schedulerName, availableRooms[0].ID).Return(availableRooms[0], nil)
		roomStorage.EXPECT().GetRoom(context.Background(), schedulerName, availableRooms[1].ID).Return(availableRooms[1], nil)

		rooms, err := roomManager.ListRoomsWithDeletionPriority(context.Background(), schedulerName, 2)
		require.NoError(t, err)
		require.Len(t, rooms, 2)
	})

	t.Run("when there error while fetch a room it returns error", func(t *testing.T) {
		schedulerName := "test-scheduler"
		availableRooms := []*game_room.GameRoom{
			{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
			{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
		}

		roomStorage.EXPECT().GetAllRoomIDs(context.Background(), schedulerName).Return(
			[]string{
				availableRooms[0].ID,
				availableRooms[1].ID,
			},
			nil,
		)
		roomStorage.EXPECT().GetRoom(context.Background(), schedulerName, availableRooms[0].ID).Return(availableRooms[0], nil)

		getRoomErr := errors.New("failed to get")
		roomStorage.EXPECT().GetRoom(context.Background(), schedulerName, availableRooms[1].ID).Return(nil, getRoomErr)

		_, err := roomManager.ListRoomsWithDeletionPriority(context.Background(), schedulerName, 2)
		require.Error(t, err)
		require.ErrorIs(t, err, getRoomErr)
	})
}
