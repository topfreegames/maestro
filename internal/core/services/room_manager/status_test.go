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

//go:build unit
// +build unit

package room_manager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/ports"

	mockeventsservice "github.com/topfreegames/maestro/internal/core/services/interfaces/mock/events_service"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	clockmock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	ismock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	pamock "github.com/topfreegames/maestro/internal/adapters/port_allocator/mock"
	runtimemock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
	"golang.org/x/sync/errgroup"
)

func TestComposeRoomStatus(t *testing.T) {
	// validate that all the combinations have a final state
	possiblePingStatus := []game_room.GameRoomPingStatus{
		game_room.GameRoomPingStatusUnknown,
		game_room.GameRoomPingStatusReady,
		game_room.GameRoomPingStatusOccupied,
		game_room.GameRoomPingStatusTerminating,
		game_room.GameRoomPingStatusTerminated,
	}

	possibleInstanceStatusType := []game_room.InstanceStatusType{
		game_room.InstanceUnknown,
		game_room.InstancePending,
		game_room.InstanceReady,
		game_room.InstanceTerminating,
		game_room.InstanceError,
	}

	// roomComposedStatus
	for _, pingStatus := range possiblePingStatus {
		for _, instanceStatusType := range possibleInstanceStatusType {
			_, err := roomComposedStatus(pingStatus, instanceStatusType)
			require.NoError(t, err)
		}
	}
}

func TestValidateRoomStatusTransition_SuccessTransitions(t *testing.T) {
	for fromStatus, transitions := range validStatusTransitions {
		for transition := range transitions {
			t.Run(fmt.Sprintf("transition from %s to %s", fromStatus.String(), transition.String()), func(t *testing.T) {
				err := validateRoomStatusTransition(fromStatus, transition)
				require.NoError(t, err)
			})
		}
	}
}

func TestValidateRoomStatusTransition_InvalidTransition(t *testing.T) {
	err := validateRoomStatusTransition(game_room.GameStatusTerminating, game_room.GameStatusReady)
	require.Error(t, err)
}

func TestRoomManager_WaitGameRoomStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	watcher := mockports.NewMockRoomStorageStatusWatcher(mockCtrl)
	roomManager := New(
		clockmock.NewFakeClock(time.Now()),
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		ismock.NewMockGameRoomInstanceStorage(mockCtrl),
		runtimemock.NewMockRuntime(mockCtrl),
		mockeventsservice.NewMockEventsService(mockCtrl),
		RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000},
	)

	transition := game_room.GameStatusReady
	gameRoom := &game_room.GameRoom{ID: "transition-test", SchedulerID: "scheduler-test", Status: game_room.GameStatusPending}

	var group errgroup.Group
	waitCalled := make(chan struct{})
	eventsChan := make(chan game_room.StatusEvent)
	group.Go(func() error {
		waitCalled <- struct{}{}

		roomStorage.EXPECT().GetRoom(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(gameRoom, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gameRoom).Return(watcher, nil)
		watcher.EXPECT().ResultChan().Return(eventsChan)
		watcher.EXPECT().Stop()

		return roomManager.WaitRoomStatus(context.Background(), gameRoom, transition)
	})

	<-waitCalled
	eventsChan <- game_room.StatusEvent{RoomID: gameRoom.ID, SchedulerName: gameRoom.SchedulerID, Status: transition}

	require.Eventually(t, func() bool {
		err := group.Wait()
		require.NoError(t, err)
		return err == nil
	}, 2*time.Second, time.Second)
}

func TestRoomManager_WaitGameRoomStatus_Deadline(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	watcher := mockports.NewMockRoomStorageStatusWatcher(mockCtrl)
	roomManager := New(
		clockmock.NewFakeClock(time.Now()),
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		ismock.NewMockGameRoomInstanceStorage(mockCtrl),
		runtimemock.NewMockRuntime(mockCtrl),
		mockeventsservice.NewMockEventsService(mockCtrl),
		RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000},
	)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Millisecond))
	gameRoom := &game_room.GameRoom{ID: "transition-test", SchedulerID: "scheduler-test", Status: game_room.GameStatusReady}

	var group errgroup.Group
	waitCalled := make(chan struct{})
	eventsChan := make(chan game_room.StatusEvent)
	group.Go(func() error {
		waitCalled <- struct{}{}
		defer cancel()

		roomStorage.EXPECT().GetRoom(ctx, gameRoom.SchedulerID, gameRoom.ID).Return(gameRoom, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gameRoom).Return(watcher, nil)
		watcher.EXPECT().ResultChan().Return(eventsChan)
		watcher.EXPECT().Stop()

		return roomManager.WaitRoomStatus(ctx, gameRoom, game_room.GameStatusOccupied)
	})

	<-waitCalled
	require.Eventually(t, func() bool {
		err := group.Wait()
		require.Error(t, err)
		return err != nil
	}, 2*time.Second, time.Second)
}

func TestUpdateGameRoomStatus(t *testing.T) {
	setup := func(mockCtrl *gomock.Controller) (*mockports.MockRoomStorage, *ismock.MockGameRoomInstanceStorage, ports.RoomManager) {
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
		roomManager := New(
			clockmock.NewFakeClock(time.Now()),
			pamock.NewMockPortAllocator(mockCtrl),
			roomStorage,
			instanceStorage,
			runtimemock.NewMockRuntime(mockCtrl),
			mockeventsservice.NewMockEventsService(mockCtrl),
			RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000},
		)

		return roomStorage, instanceStorage, roomManager
	}

	t.Run("when game room exists and changes states, it should return no error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerName := "schedulerName"
		roomId := "room-id"
		roomStorage, instanceStorage, roomManager := setup(mockCtrl)

		room := &game_room.GameRoom{PingStatus: game_room.GameRoomPingStatusReady, Status: game_room.GameStatusPending}
		roomStorage.EXPECT().GetRoom(context.Background(), schedulerName, roomId).Return(room, nil)

		instance := &game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceReady}}
		instanceStorage.EXPECT().GetInstance(context.Background(), schedulerName, roomId).Return(instance, nil)

		roomStorage.EXPECT().UpdateRoomStatus(context.Background(), schedulerName, roomId, game_room.GameStatusReady)

		err := roomManager.UpdateGameRoomStatus(context.Background(), schedulerName, roomId)
		require.NoError(t, err)
	})

	t.Run("when game room exists and there is not state transition, it should not update the room status and return no error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerName := "schedulerName"
		roomId := "room-id"
		roomStorage, instanceStorage, roomManager := setup(mockCtrl)

		room := &game_room.GameRoom{PingStatus: game_room.GameRoomPingStatusReady, Status: game_room.GameStatusReady}
		roomStorage.EXPECT().GetRoom(context.Background(), schedulerName, roomId).Return(room, nil)

		instance := &game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceReady}}
		instanceStorage.EXPECT().GetInstance(context.Background(), schedulerName, roomId).Return(instance, nil)

		err := roomManager.UpdateGameRoomStatus(context.Background(), schedulerName, roomId)
		require.NoError(t, err)
	})

	t.Run("when game room doesn't exists, it should return error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerName := "schedulerName"
		roomId := "room-id"
		roomStorage, _, roomManager := setup(mockCtrl)

		roomStorage.EXPECT().GetRoom(context.Background(), schedulerName, roomId).Return(nil, porterrors.ErrNotFound)

		err := roomManager.UpdateGameRoomStatus(context.Background(), schedulerName, roomId)
		require.Error(t, err)
	})

	t.Run("when game room instance doesn't exists, it should return error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerName := "schedulerName"
		roomId := "room-id"
		roomStorage, instanceStorage, roomManager := setup(mockCtrl)

		room := &game_room.GameRoom{PingStatus: game_room.GameRoomPingStatusReady, Status: game_room.GameStatusPending}
		roomStorage.EXPECT().GetRoom(context.Background(), schedulerName, roomId).Return(room, nil)

		instanceStorage.EXPECT().GetInstance(context.Background(), schedulerName, roomId).Return(nil, porterrors.ErrNotFound)

		err := roomManager.UpdateGameRoomStatus(context.Background(), schedulerName, roomId)
		require.Error(t, err)
	})

	t.Run("when game room exists and state transition is invalid, it should return error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerName := "schedulerName"
		roomId := "room-id"
		roomStorage, instanceStorage, roomManager := setup(mockCtrl)

		room := &game_room.GameRoom{PingStatus: game_room.GameRoomPingStatusReady, Status: game_room.GameStatusTerminating}
		roomStorage.EXPECT().GetRoom(context.Background(), schedulerName, roomId).Return(room, nil)

		instance := &game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceReady}}
		instanceStorage.EXPECT().GetInstance(context.Background(), schedulerName, roomId).Return(instance, nil)

		err := roomManager.UpdateGameRoomStatus(context.Background(), schedulerName, roomId)
		require.Error(t, err)
	})
}
