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
	"fmt"
	"context"
	"errors"
	"testing"
	"time"

	eventsForwarder "github.com/topfreegames/maestro/internal/adapters/events_forwarder/mock"

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
	eventsForwarder := eventsForwarder.NewMockEventsForwarder(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	fakeClock := clockmock.NewFakeClock(now)
	config := RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}
	roomManager := NewRoomManager(fakeClock, portAllocator, roomStorage, instanceStorage, runtime, eventsForwarder, config)
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
		}).Return(nil, porterrors.NewErrUnexpected("error create game room instance"))

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

		roomStorage.EXPECT().CreateRoom(context.Background(), &gameRoom).Return(porterrors.NewErrUnexpected("error storing room on redis"))

		room, instance, err := roomManager.CreateRoom(context.Background(), scheduler)
		require.Error(t, err)
		require.Nil(t, room)
		require.Nil(t, instance)
	})

	t.Run("when game room creation fails while allocating ports then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return(nil, porterrors.NewErrInvalidArgument("not enough ports to allocate"))

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
	eventsForwarder := eventsForwarder.NewMockEventsForwarder(mockCtrl)
	config := RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}

	roomManager := NewRoomManager(
		clockmock.NewFakeClock(time.Now()),
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
		eventsForwarder,
		config,
	)

	t.Run("when the game room status transition is valid then it deletes the game room and update its status", func(t *testing.T) {
		gameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusReady}
		instance := &game_room.Instance{ID: "test-instance"}
		instanceStorage.EXPECT().GetInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(instance, nil)
		runtime.EXPECT().DeleteGameRoomInstance(context.Background(), instance).Return(nil)

		err := roomManager.DeleteRoom(context.Background(), gameRoom)
		require.NoError(t, err)
	})

	t.Run("when room deletion has error returns error", func(t *testing.T) {
		gameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusTerminating}
		instance := &game_room.Instance{ID: "test-instance"}
		instanceStorage.EXPECT().GetInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(instance, nil)
		runtime.EXPECT().DeleteGameRoomInstance(context.Background(), instance).Return(porterrors.ErrUnexpected)

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
	eventsForwarder := eventsForwarder.NewMockEventsForwarder(mockCtrl)
	clock := clockmock.NewFakeClock(time.Now())
	config := RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}
	roomManager := NewRoomManager(
		clock,
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
		eventsForwarder,
		config,
	)
	currentInstance := &game_room.Instance{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.InstanceStatus{Type: game_room.InstanceReady}}
	newGameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusReady, PingStatus: game_room.GameRoomPingStatusOccupied, LastPingAt: clock.Now()}

	t.Run("when the current game room exists then it execute without returning error", func(t *testing.T) {
		roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoom).Return(nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(currentInstance, nil)
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(newGameRoom, nil)
		roomStorage.EXPECT().UpdateRoomStatus(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID, game_room.GameStatusOccupied).Return(nil)
		eventsForwarder.EXPECT().ForwardRoomEvent(context.Background(), newGameRoom, "pingReady", "", newGameRoom.Metadata).Return(nil)

		err := roomManager.UpdateRoom(context.Background(), newGameRoom)
		require.NoError(t, err)
	})

	t.Run("when update fails then it returns proper error", func(t *testing.T) {
		roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoom).Return(porterrors.ErrUnexpected)

		err := roomManager.UpdateRoom(context.Background(), newGameRoom)
		require.Error(t, err)
	})

	t.Run("when there is some error while updating the room then it returns proper error", func(t *testing.T) {
		roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoom).Return(porterrors.ErrUnexpected)

		err := roomManager.UpdateRoom(context.Background(), newGameRoom)
		require.Error(t, err)
	})

	t.Run("when the game room state transition is invalid then it returns proper error", func(t *testing.T) {
		newGameRoomInvalidState := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusTerminating, PingStatus: game_room.GameRoomPingStatusReady}
		roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoomInvalidState).Return(nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), newGameRoomInvalidState.SchedulerID, newGameRoomInvalidState.ID).Return(currentInstance, nil)
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoomInvalidState.SchedulerID, newGameRoomInvalidState.ID).Return(newGameRoomInvalidState, nil)

		err := roomManager.UpdateRoom(context.Background(), newGameRoomInvalidState)
		require.Error(t, err)
	})

	t.Run("when update status fails then it returns error", func(t *testing.T) {
		roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoom).Return(nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(currentInstance, nil)
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(newGameRoom, nil)
		roomStorage.EXPECT().UpdateRoomStatus(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID, game_room.GameStatusOccupied).Return(porterrors.ErrUnexpected)

		err := roomManager.UpdateRoom(context.Background(), newGameRoom)
		require.Error(t, err)
	})

	t.Run("when some error occurs on event forwarding then it does not return with error", func(t *testing.T) {
		roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoom).Return(nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(currentInstance, nil)
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(newGameRoom, nil)
		roomStorage.EXPECT().UpdateRoomStatus(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID, game_room.GameStatusOccupied).Return(nil)
		eventsForwarder.EXPECT().ForwardRoomEvent(context.Background(), newGameRoom, "pingReady", "", newGameRoom.Metadata).Return(errors.New("some error"))

		err := roomManager.UpdateRoom(context.Background(), newGameRoom)
		require.NoError(t, err)
	})
}

func TestRoomManager_ListRoomsWithDeletionPriority(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	eventsForwarder := eventsForwarder.NewMockEventsForwarder(mockCtrl)
	clock := clockmock.NewFakeClock(time.Now())
	roomManager := NewRoomManager(
		clock,
		nil,
		roomStorage,
		nil,
		runtime,
		eventsForwarder,
		RoomManagerConfig{RoomPingTimeout: time.Hour},
	)

	t.Run("when there are enough rooms it should return the specified number", func(t *testing.T) {
		ctx := context.Background()
		schedulerName := "test-scheduler"
		schedulerLastVersion := "v1.2.3"
		availableRooms := []*game_room.GameRoom{
			{ID: "first-room", SchedulerID: schedulerName, Version: schedulerLastVersion, Status: game_room.GameStatusError},
			{ID: "second-room", SchedulerID: schedulerName, Version: schedulerLastVersion, Status: game_room.GameStatusReady},
			{ID: "third-room", SchedulerID: schedulerName, Version: schedulerLastVersion, Status: game_room.GameStatusPending},
			{ID: "forth-room", SchedulerID: schedulerName, Version: schedulerLastVersion, Status: game_room.GameStatusReady},
			{ID: "fifth-room", SchedulerID: schedulerName, Version: schedulerLastVersion, Status: game_room.GameStatusOccupied},
		}

		roomStorage.EXPECT().
			GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusError).
			Return([]string{availableRooms[0].ID}, nil)

		roomStorage.EXPECT().
			GetRoomIDsByLastPing(ctx, schedulerName, gomock.Any()).
			Return([]string{availableRooms[1].ID}, nil)

		roomStorage.EXPECT().
			GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusPending).
			Return([]string{availableRooms[2].ID}, nil)

		roomStorage.EXPECT().
			GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusReady).
			Return([]string{availableRooms[3].ID}, nil)

		roomStorage.EXPECT().
			GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusOccupied).
			Return([]string{availableRooms[4].ID, availableRooms[1].ID}, nil)

		roomStorage.EXPECT().GetRoom(ctx, schedulerName, availableRooms[0].ID).Return(availableRooms[0], nil)
		roomStorage.EXPECT().GetRoom(ctx, schedulerName, availableRooms[1].ID).Return(availableRooms[1], nil)
		roomStorage.EXPECT().GetRoom(ctx, schedulerName, availableRooms[2].ID).Return(availableRooms[2], nil)
		roomStorage.EXPECT().GetRoom(ctx, schedulerName, availableRooms[3].ID).Return(availableRooms[3], nil)
		roomStorage.EXPECT().GetRoom(ctx, schedulerName, availableRooms[4].ID).Return(availableRooms[4], nil)

		rooms, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "v1.2.2", 5)
		require.NoError(t, err)
		require.Len(t, rooms, 5)
	})

	t.Run("when error happens while fetching on-error room ids it returns error", func(t *testing.T) {
		ctx := context.Background()
		schedulerName := "test-scheduler"
		getRoomIDsErr := errors.New("failed to get rooms IDs")

		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusError).Return(nil, getRoomIDsErr)

		_, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", 2)
		require.Error(t, err)
		require.ErrorIs(t, err, getRoomIDsErr)
	})

	t.Run("when error happens while fetching old ping room ids it returns error", func(t *testing.T) {
		ctx := context.Background()
		schedulerName := "test-scheduler"
		getRoomIDsErr := errors.New("failed to get rooms IDs")

		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusError).Return([]string{}, nil)
		roomStorage.EXPECT().GetRoomIDsByLastPing(ctx, schedulerName, gomock.Any()).Return(nil, getRoomIDsErr)

		_, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", 2)
		require.Error(t, err)
		require.ErrorIs(t, err, getRoomIDsErr)
	})

	t.Run("when error happens while fetching pending room ids it returns error", func(t *testing.T) {
		ctx := context.Background()
		schedulerName := "test-scheduler"
		getRoomIDsErr := errors.New("failed to get rooms IDs")

		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusError).Return([]string{}, nil)
		roomStorage.EXPECT().GetRoomIDsByLastPing(ctx, schedulerName, gomock.Any()).Return([]string{}, nil)
		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusPending).Return(nil, getRoomIDsErr)

		_, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", 2)
		require.Error(t, err)
		require.ErrorIs(t, err, getRoomIDsErr)
	})

	t.Run("when error happens while fetching ready room ids it returns error", func(t *testing.T) {
		ctx := context.Background()
		schedulerName := "test-scheduler"
		getRoomIDsErr := errors.New("failed to get rooms IDs")

		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusError).Return([]string{}, nil)
		roomStorage.EXPECT().GetRoomIDsByLastPing(ctx, schedulerName, gomock.Any()).Return([]string{}, nil)
		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusPending).Return([]string{}, nil)
		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusReady).Return(nil, getRoomIDsErr)

		_, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", 2)
		require.Error(t, err)
		require.ErrorIs(t, err, getRoomIDsErr)
	})

	t.Run("when error happens while fetching occupied room ids it returns error", func(t *testing.T) {
		ctx := context.Background()
		schedulerName := "test-scheduler"
		getRoomIDsErr := errors.New("failed to get rooms IDs")

		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusError).Return([]string{}, nil)
		roomStorage.EXPECT().GetRoomIDsByLastPing(ctx, schedulerName, gomock.Any()).Return([]string{}, nil)
		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusPending).Return([]string{}, nil)
		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusReady).Return([]string{}, nil)
		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusOccupied).Return(nil, getRoomIDsErr)

		_, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", 2)
		require.Error(t, err)
		require.ErrorIs(t, err, getRoomIDsErr)
	})

	t.Run("when error happens while fetch a room it returns error", func(t *testing.T) {
		ctx := context.Background()
		schedulerName := "test-scheduler"
		availableRooms := []*game_room.GameRoom{
			{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
			{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
		}

		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, gomock.Any()).Return([]string{availableRooms[0].ID}, nil).AnyTimes()
		roomStorage.EXPECT().GetRoomIDsByLastPing(ctx, schedulerName, gomock.Any()).Return([]string{availableRooms[1].ID}, nil)

		roomStorage.EXPECT().GetRoom(ctx, schedulerName, availableRooms[0].ID).Return(availableRooms[0], nil)

		getRoomErr := errors.New("failed to get")
		roomStorage.EXPECT().GetRoom(ctx, schedulerName, availableRooms[1].ID).Return(nil, getRoomErr)

		_, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", 2)
		require.Error(t, err)
		require.ErrorIs(t, err, getRoomErr)
	})

	t.Run("when no room matches version returns an empty list", func(t *testing.T) {
		ctx := context.Background()
		schedulerName := "test-scheduler"
		ignoredVersion := "v1.2.3"
		availableRooms := []*game_room.GameRoom{
			{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Version: ignoredVersion},
			{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Version: ignoredVersion},
		}

		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, gomock.Any()).Return([]string{availableRooms[0].ID}, nil).AnyTimes()
		roomStorage.EXPECT().GetRoomIDsByLastPing(ctx, schedulerName, gomock.Any()).Return([]string{availableRooms[1].ID}, nil)

		roomStorage.EXPECT().GetRoom(ctx, schedulerName, availableRooms[0].ID).Return(availableRooms[0], nil)
		roomStorage.EXPECT().GetRoom(ctx, schedulerName, availableRooms[1].ID).Return(availableRooms[1], nil)

		rooms, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, ignoredVersion, 2)
		require.NoError(t, err)
		require.Empty(t, rooms)
	})
}

func TestRoomManager_UpdateRoomInstance(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	eventsForwarder := eventsForwarder.NewMockEventsForwarder(mockCtrl)
	clock := clockmock.NewFakeClock(time.Now())
	config := RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}
	roomManager := NewRoomManager(
		clock,
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
		eventsForwarder,
		config,
	)
	currentGameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusReady, PingStatus: game_room.GameRoomPingStatusReady, LastPingAt: clock.Now()}
	newGameRoomInstance := &game_room.Instance{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.InstanceStatus{Type: game_room.InstanceError}}

	t.Run("updates rooms with success", func(t *testing.T) {
		instanceStorage.EXPECT().UpsertInstance(context.Background(), newGameRoomInstance).Return(nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), newGameRoomInstance.SchedulerID, newGameRoomInstance.ID).Return(newGameRoomInstance, nil)
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoomInstance.SchedulerID, newGameRoomInstance.ID).Return(currentGameRoom, nil)
		roomStorage.EXPECT().UpdateRoomStatus(context.Background(), newGameRoomInstance.SchedulerID, newGameRoomInstance.ID, game_room.GameStatusError).Return(nil)

		err := roomManager.UpdateRoomInstance(context.Background(), newGameRoomInstance)
		require.NoError(t, err)
	})

	t.Run("when storage fails to update returns errors", func(t *testing.T) {
		instanceStorage.EXPECT().UpsertInstance(context.Background(), newGameRoomInstance).Return(porterrors.ErrUnexpected)

		err := roomManager.UpdateRoomInstance(context.Background(), newGameRoomInstance)
		require.Error(t, err)
	})
}

func TestRoomManager_CleanRoomState(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	eventsForwarder := eventsForwarder.NewMockEventsForwarder(mockCtrl)
	clock := clockmock.NewFakeClock(time.Now())
	config := RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}
	roomManager := NewRoomManager(
		clock,
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
		eventsForwarder,
		config,
	)
	schedulerName := "scheduler-name"
	roomId := "some-unique-room-id"

	t.Run("when room and instance deletions do not return error", func(t *testing.T) {
		roomStorage.EXPECT().DeleteRoom(context.Background(), schedulerName, roomId).Return(nil)
		instanceStorage.EXPECT().DeleteInstance(context.Background(), schedulerName, roomId).Return(nil)

		err := roomManager.CleanRoomState(context.Background(), schedulerName, roomId)
		require.NoError(t, err)
	})

	t.Run("when room is not found but instance is, returns no error", func(t *testing.T) {
		roomStorage.EXPECT().DeleteRoom(context.Background(), schedulerName, roomId).Return(porterrors.ErrNotFound)
		instanceStorage.EXPECT().DeleteInstance(context.Background(), schedulerName, roomId).Return(nil)

		err := roomManager.CleanRoomState(context.Background(), schedulerName, roomId)
		require.NoError(t, err)
	})

	t.Run("when room is present but instance isn't, returns no error", func(t *testing.T) {
		roomStorage.EXPECT().DeleteRoom(context.Background(), schedulerName, roomId).Return(nil)
		instanceStorage.EXPECT().DeleteInstance(context.Background(), schedulerName, roomId).Return(porterrors.ErrNotFound)

		err := roomManager.CleanRoomState(context.Background(), schedulerName, roomId)
		require.NoError(t, err)
	})

	t.Run("when deletions returns unexpected error, returns error", func(t *testing.T) {
		roomStorage.EXPECT().DeleteRoom(context.Background(), schedulerName, roomId).Return(porterrors.ErrUnexpected)

		err := roomManager.CleanRoomState(context.Background(), schedulerName, roomId)
		require.Error(t, err)

		roomStorage.EXPECT().DeleteRoom(context.Background(), schedulerName, roomId).Return(nil)
		instanceStorage.EXPECT().DeleteInstance(context.Background(), schedulerName, roomId).Return(porterrors.ErrUnexpected)

		err = roomManager.CleanRoomState(context.Background(), schedulerName, roomId)
		require.Error(t, err)
	})
}

func TestSchedulerMaxSurge(t *testing.T) {
	setupRoomStorage := func(mockCtrl *gomock.Controller) (*rsmock.MockRoomStorage, *RoomManager) {
		roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
		roomManager := NewRoomManager(
			clockmock.NewFakeClock(time.Now()),
			pamock.NewMockPortAllocator(mockCtrl),
			roomStorage,
			ismock.NewMockGameRoomInstanceStorage(mockCtrl),
			runtimemock.NewMockRuntime(mockCtrl),
			eventsForwarder.NewMockEventsForwarder(mockCtrl),
			RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000},
		)

		return roomStorage, roomManager
	}

	t.Run("max surge uses absolute number, returns value without error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		_, roomManager := setupRoomStorage(mockCtrl)
		scheduler := &entities.Scheduler{Name: "test", MaxSurge: "100"}

		maxSurgeValue, err := roomManager.SchedulerMaxSurge(context.Background(), scheduler)
		require.NoError(t, err)
		require.Equal(t, 100, maxSurgeValue)
	})

	t.Run("max surge uses relative number and there are rooms, returns value without error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		roomStorage, roomManager := setupRoomStorage(mockCtrl)
		scheduler := &entities.Scheduler{Name: "test", MaxSurge: "50%"}

		roomStorage.EXPECT().GetRoomCount(gomock.Any(), scheduler.Name).Return(10, nil)

		maxSurgeValue, err := roomManager.SchedulerMaxSurge(context.Background(), scheduler)
		require.NoError(t, err)
		require.Equal(t, 5, maxSurgeValue)
	})

	t.Run("max surge uses relative number and there low number of rooms, returns min 1 without error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		roomStorage, roomManager := setupRoomStorage(mockCtrl)
		scheduler := &entities.Scheduler{Name: "test", MaxSurge: "10%"}

		roomStorage.EXPECT().GetRoomCount(gomock.Any(), scheduler.Name).Return(1, nil)

		maxSurgeValue, err := roomManager.SchedulerMaxSurge(context.Background(), scheduler)
		require.NoError(t, err)
		require.Equal(t, 1, maxSurgeValue)
	})

	t.Run("max surge uses relative number and failed to retrieve rooms count, returns error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		roomStorage, roomManager := setupRoomStorage(mockCtrl)
		scheduler := &entities.Scheduler{Name: "test", MaxSurge: "10%"}

		roomStorage.EXPECT().GetRoomCount(gomock.Any(), scheduler.Name).Return(0, porterrors.ErrUnexpected)

		_, err := roomManager.SchedulerMaxSurge(context.Background(), scheduler)
		require.Error(t, err)
	})

	t.Run("max surge is invalid, returns error", func(t *testing.T) {
		invalidMaxSurges := []string{"", "%", "a%", "a", "1a", "%123"}

		for _, invalidMaxSurge := range invalidMaxSurges {
			t.Run(fmt.Sprintf("max surge = %s", invalidMaxSurge), func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				_, roomManager := setupRoomStorage(mockCtrl)
				scheduler := &entities.Scheduler{Name: "test", MaxSurge: invalidMaxSurge}

				_, err := roomManager.SchedulerMaxSurge(context.Background(), scheduler)
				require.Error(t, err)
			})
		}
	})
}
