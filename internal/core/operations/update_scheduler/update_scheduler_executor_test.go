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

package update_scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	clock_mock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	events_forwarder_mock "github.com/topfreegames/maestro/internal/adapters/events_forwarder/mock"
	instance_storage_mock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	port_allocator_mock "github.com/topfreegames/maestro/internal/adapters/port_allocator/mock"
	room_storage_mock "github.com/topfreegames/maestro/internal/adapters/room_storage/mock"
	runtime_mock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
	scheduler_storage_mock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/ports"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"
)

func TestUpdateSchedulerExecutor_Execute_ReplaceRooms(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mocks := newMockRoomAndSchedulerManager(mockCtrl)

	currentVersion := "10.0.0"
	newVersion := "11.0.0"

	currentScheduler := newValidScheduler()
	currentScheduler.Spec.Version = currentVersion
	currentScheduler.PortRange.Start = 5000
	currentScheduler.MaxSurge = "3"

	newScheduler := newValidScheduler()
	newScheduler.PortRange.Start = 1000
	newScheduler.MaxSurge = "3"

	definition := &UpdateSchedulerDefinition{
		NewScheduler: newScheduler,
	}

	mocks.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), definition.NewScheduler.Name).Return(&currentScheduler, nil)
	mocks.schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

	// list the rooms in two "cycles"
	firstRoomsIds := []string{"room-0", "room-1", "room-2"}
	secondRoomsIds := []string{"room-3", "room-4"}

	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return(firstRoomsIds, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, nil)

	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return(secondRoomsIds, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, nil)

	// third time we list we want it to be empty
	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Times(4).Return([]string{}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, nil)

	// for each room we want to mock: a new room creation and its
	// deletion.
	for _, roomId := range append(firstRoomsIds, secondRoomsIds...) {
		currentGameRoom := game_room.GameRoom{
			ID:          roomId,
			Version:     currentVersion,
			SchedulerID: definition.NewScheduler.Name,
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Now(),
		}

		mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), definition.NewScheduler.Name, roomId).Return(&currentGameRoom, nil)

		currentGameRoomInstance := game_room.Instance{
			ID:          roomId,
			SchedulerID: definition.NewScheduler.Name,
		}

		newGameRoomInstance := game_room.Instance{
			ID:          fmt.Sprintf("new-%s", roomId),
			SchedulerID: definition.NewScheduler.Name,
		}

		newGameRoom := game_room.GameRoom{
			ID:          newGameRoomInstance.ID,
			SchedulerID: definition.NewScheduler.Name,
			Status:      game_room.GameStatusPending,
			Version:     newVersion,
		}

		mocks.portAllocator.EXPECT().Allocate(gomock.Any(), 1).Return([]int32{5000}, nil)
		mocks.runtime.EXPECT().CreateGameRoomInstance(context.Background(), definition.NewScheduler.Name, versionEq(newVersion)).Return(&newGameRoomInstance, nil)

		gameRoomReady := newGameRoom
		gameRoomReady.Status = game_room.GameStatusReady
		mocks.roomStorage.EXPECT().CreateRoom(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newVersion))).Return(nil)
		mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoomReady.SchedulerID, gameRoomReady.ID).Return(&gameRoomReady, nil)
		roomStorageStatusWatcher := room_storage_mock.NewMockRoomStorageStatusWatcher(mockCtrl)
		mocks.roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newVersion))).Return(roomStorageStatusWatcher, nil)
		roomStorageStatusWatcher.EXPECT().Stop()

		mocks.instanceStorage.EXPECT().GetInstance(gomock.Any(), definition.NewScheduler.Name, roomId).Return(&currentGameRoomInstance, nil)
		mocks.runtime.EXPECT().DeleteGameRoomInstance(gomock.Any(), &currentGameRoomInstance).Return(nil)
	}

	executor := NewExecutor(mocks.roomManager, mocks.schedulerManager)
	err := executor.Execute(context.Background(), &operation.Operation{}, definition)
	require.NoError(t, err)
}

func TestUpdateSchedulerExecutor_Execute_NoRunningRooms(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mocks := newMockRoomAndSchedulerManager(mockCtrl)

	currentScheduler := newValidScheduler()
	currentScheduler.Spec.Version = "10.0.0"
	currentScheduler.PortRange.Start = 5000
	currentScheduler.MaxSurge = "3"

	newScheduler := newValidScheduler()
	newScheduler.PortRange.Start = 1000
	newScheduler.MaxSurge = "3"

	definition := &UpdateSchedulerDefinition{
		NewScheduler: newScheduler,
	}

	mocks.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), definition.NewScheduler.Name).Return(&currentScheduler, nil)
	mocks.schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Times(4).Return([]string{}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, nil)

	executor := NewExecutor(mocks.roomManager, mocks.schedulerManager)
	err := executor.Execute(context.Background(), &operation.Operation{}, definition)
	require.NoError(t, err)
}

func TestUpdateSchedulerExecutor_Execute_MinorUpdate(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mocks := newMockRoomAndSchedulerManager(mockCtrl)

	currentScheduler := newValidScheduler()
	currentScheduler.MaxSurge = "3"

	newScheduler := newValidScheduler()
	newScheduler.MaxSurge = "5"

	definition := &UpdateSchedulerDefinition{
		NewScheduler: newScheduler,
	}

	mocks.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), definition.NewScheduler.Name).Return(&currentScheduler, nil)
	mocks.schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

	executor := NewExecutor(mocks.roomManager, mocks.schedulerManager)
	err := executor.Execute(context.Background(), &operation.Operation{}, definition)
	require.NoError(t, err)
}

func TestUpdateSchedulerExecutor_Execute_ReplaceRooms_MaxSurge(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mocks := newMockRoomAndSchedulerManager(mockCtrl)

	currentVersion := "10.0.0"
	newVersion := "11.0.0"

	currentScheduler := newValidScheduler()
	currentScheduler.Spec.Version = currentVersion
	currentScheduler.PortRange.Start = 5000
	currentScheduler.MaxSurge = "2"

	newScheduler := newValidScheduler()
	newScheduler.PortRange.Start = 1000
	newScheduler.MaxSurge = "2"

	definition := &UpdateSchedulerDefinition{
		NewScheduler: newScheduler,
	}

	mocks.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), definition.NewScheduler.Name).Return(&currentScheduler, nil)
	mocks.schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

	// list the rooms in two "cycles"
	firstRoomsIds := []string{"room-0", "room-1"}
	secondRoomsIds := []string{"room-2", "room-3"}
	thirdRoomsIds := []string{"room-4"}

	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return(firstRoomsIds, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, nil)

	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return(secondRoomsIds, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, nil)

	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return(thirdRoomsIds, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, nil)

	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Times(4).Return([]string{}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, nil)

	// first room is going to block one replace goroutine
	firstRoomId := firstRoomsIds[0]
	currentGameRoom := game_room.GameRoom{
		ID:          firstRoomId,
		Version:     currentVersion,
		SchedulerID: definition.NewScheduler.Name,
		Status:      game_room.GameStatusReady,
		LastPingAt:  time.Now(),
	}

	mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), definition.NewScheduler.Name, firstRoomId).Return(&currentGameRoom, nil)

	newGameRoomInstance := game_room.Instance{
		ID:          fmt.Sprintf("new-%s", firstRoomId),
		SchedulerID: definition.NewScheduler.Name,
	}

	newGameRoom := game_room.GameRoom{
		ID:          newGameRoomInstance.ID,
		SchedulerID: definition.NewScheduler.Name,
		Status:      game_room.GameStatusPending,
		Version:     newVersion,
	}

	mocks.portAllocator.EXPECT().Allocate(gomock.Any(), 1).Return([]int32{5000}, nil)
	mocks.runtime.EXPECT().CreateGameRoomInstance(context.Background(), definition.NewScheduler.Name, versionEq(newVersion)).Return(&newGameRoomInstance, nil)

	mocks.roomStorage.EXPECT().CreateRoom(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newVersion))).Return(nil)
	mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), newGameRoom.SchedulerID, newGameRoom.ID).Return(&newGameRoom, nil)
	roomStorageStatusWatcher := room_storage_mock.NewMockRoomStorageStatusWatcher(mockCtrl)
	mocks.roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newVersion))).Return(roomStorageStatusWatcher, nil)

	// this will make the first replace to block forever
	roomStorageStatusWatcher.EXPECT().ResultChan().Return(make(chan game_room.StatusEvent))
	roomStorageStatusWatcher.EXPECT().Stop()

	for _, roomId := range append(firstRoomsIds[1:], append(secondRoomsIds, thirdRoomsIds...)...) {
		currentGameRoom := game_room.GameRoom{
			ID:          roomId,
			Version:     currentVersion,
			SchedulerID: definition.NewScheduler.Name,
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Now(),
		}

		mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), definition.NewScheduler.Name, roomId).Return(&currentGameRoom, nil)

		currentGameRoomInstance := game_room.Instance{
			ID:          roomId,
			SchedulerID: definition.NewScheduler.Name,
		}

		newGameRoomInstance := game_room.Instance{
			ID:          fmt.Sprintf("new-%s", roomId),
			SchedulerID: definition.NewScheduler.Name,
		}

		newGameRoom := game_room.GameRoom{
			ID:          newGameRoomInstance.ID,
			SchedulerID: definition.NewScheduler.Name,
			Status:      game_room.GameStatusPending,
			Version:     newVersion,
		}

		mocks.portAllocator.EXPECT().Allocate(gomock.Any(), 1).Return([]int32{5000}, nil)
		mocks.runtime.EXPECT().CreateGameRoomInstance(context.Background(), definition.NewScheduler.Name, versionEq(newVersion)).Return(&newGameRoomInstance, nil)

		gameRoomReady := newGameRoom
		gameRoomReady.Status = game_room.GameStatusReady
		mocks.roomStorage.EXPECT().CreateRoom(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newVersion))).Return(nil)
		mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoomReady.SchedulerID, gameRoomReady.ID).Return(&gameRoomReady, nil)
		roomStorageStatusWatcher := room_storage_mock.NewMockRoomStorageStatusWatcher(mockCtrl)
		mocks.roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newVersion))).Return(roomStorageStatusWatcher, nil)
		roomStorageStatusWatcher.EXPECT().Stop()

		mocks.instanceStorage.EXPECT().GetInstance(gomock.Any(), definition.NewScheduler.Name, roomId).Return(&currentGameRoomInstance, nil)
		mocks.runtime.EXPECT().DeleteGameRoomInstance(gomock.Any(), &currentGameRoomInstance).Return(nil)
	}

	executor := NewExecutor(mocks.roomManager, mocks.schedulerManager)
	err := executor.Execute(context.Background(), &operation.Operation{}, definition)
	require.NoError(t, err)
}

func TestUpdateSchedulerExecutor_Execute_ReplaceRooms_ReplaceFail(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mocks := newMockRoomAndSchedulerManager(mockCtrl)

	currentVersion := "10.0.0"
	newVersion := "11.0.0"

	currentScheduler := newValidScheduler()
	currentScheduler.Spec.Version = currentVersion
	currentScheduler.PortRange.Start = 5000
	currentScheduler.MaxSurge = "2"

	newScheduler := newValidScheduler()
	newScheduler.PortRange.Start = 1000
	newScheduler.MaxSurge = "2"

	definition := &UpdateSchedulerDefinition{
		NewScheduler: newScheduler,
	}

	mocks.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), definition.NewScheduler.Name).Return(&currentScheduler, nil)
	mocks.schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

	// list the rooms in two "cycles"
	firstRoomsIds := []string{"room-0", "room-1"}
	secondRoomsIds := []string{"room-2", "room-3"}

	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return(firstRoomsIds, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, nil)

	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return(secondRoomsIds, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, nil)

	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Times(4).Return([]string{}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, nil)

	// first room is going to block one replace goroutine
	firstRoomId := firstRoomsIds[0]
	currentGameRoom := game_room.GameRoom{
		ID:          firstRoomId,
		Version:     currentVersion,
		SchedulerID: definition.NewScheduler.Name,
		Status:      game_room.GameStatusReady,
		LastPingAt:  time.Now(),
	}

	mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), definition.NewScheduler.Name, firstRoomId).Return(&currentGameRoom, nil)

	newGameRoomInstance := game_room.Instance{
		ID:          fmt.Sprintf("new-%s", firstRoomId),
		SchedulerID: definition.NewScheduler.Name,
	}

	newGameRoom := game_room.GameRoom{
		ID:          newGameRoomInstance.ID,
		SchedulerID: definition.NewScheduler.Name,
		Status:      game_room.GameStatusPending,
		Version:     newVersion,
	}

	mocks.portAllocator.EXPECT().Allocate(gomock.Any(), 1).Return([]int32{5000}, nil)
	mocks.runtime.EXPECT().CreateGameRoomInstance(context.Background(), definition.NewScheduler.Name, versionEq(newVersion)).Return(&newGameRoomInstance, nil)
	// fail the first room creation
	mocks.roomStorage.EXPECT().CreateRoom(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newVersion))).Return(porterrors.ErrUnexpected)

	for _, roomId := range append(firstRoomsIds[1:], secondRoomsIds...) {
		currentGameRoom := game_room.GameRoom{
			ID:          roomId,
			Version:     currentVersion,
			SchedulerID: definition.NewScheduler.Name,
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Now(),
		}

		mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), definition.NewScheduler.Name, roomId).Return(&currentGameRoom, nil)

		currentGameRoomInstance := game_room.Instance{
			ID:          roomId,
			SchedulerID: definition.NewScheduler.Name,
		}

		newGameRoomInstance := game_room.Instance{
			ID:          fmt.Sprintf("new-%s", roomId),
			SchedulerID: definition.NewScheduler.Name,
		}

		newGameRoom := game_room.GameRoom{
			ID:          newGameRoomInstance.ID,
			SchedulerID: definition.NewScheduler.Name,
			Status:      game_room.GameStatusPending,
			Version:     newVersion,
		}

		mocks.portAllocator.EXPECT().Allocate(gomock.Any(), 1).Return([]int32{5000}, nil)
		mocks.runtime.EXPECT().CreateGameRoomInstance(context.Background(), definition.NewScheduler.Name, versionEq(newVersion)).Return(&newGameRoomInstance, nil)

		gameRoomReady := newGameRoom
		gameRoomReady.Status = game_room.GameStatusReady
		mocks.roomStorage.EXPECT().CreateRoom(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newVersion))).Return(nil)
		mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoomReady.SchedulerID, gameRoomReady.ID).Return(&gameRoomReady, nil)
		roomStorageStatusWatcher := room_storage_mock.NewMockRoomStorageStatusWatcher(mockCtrl)
		mocks.roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newVersion))).Return(roomStorageStatusWatcher, nil)
		roomStorageStatusWatcher.EXPECT().Stop()

		mocks.instanceStorage.EXPECT().GetInstance(gomock.Any(), definition.NewScheduler.Name, roomId).Return(&currentGameRoomInstance, nil)
		mocks.runtime.EXPECT().DeleteGameRoomInstance(gomock.Any(), &currentGameRoomInstance).Return(nil)
	}

	executor := NewExecutor(mocks.roomManager, mocks.schedulerManager)
	err := executor.Execute(context.Background(), &operation.Operation{}, definition)
	require.NoError(t, err)
}

func TestUpdateSchedulerExecutor_Execute_StopDuringReplace(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mocks := newMockRoomAndSchedulerManager(mockCtrl)

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	currentVersion := "10.0.0"
	newVersion := "11.0.0"

	currentScheduler := newValidScheduler()
	currentScheduler.Spec.Version = currentVersion
	currentScheduler.PortRange.Start = 5000
	currentScheduler.MaxSurge = "3"

	newScheduler := newValidScheduler()
	newScheduler.PortRange.Start = 1000
	newScheduler.MaxSurge = "3"

	definition := &UpdateSchedulerDefinition{
		NewScheduler: newScheduler,
	}

	mocks.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), definition.NewScheduler.Name).Return(&currentScheduler, nil)
	mocks.schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

	roomId := "room-0"
	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{roomId}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, nil)

	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Times(4).Return([]string{}, nil)
	mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, nil)

	currentGameRoom := game_room.GameRoom{
		ID:          roomId,
		Version:     currentVersion,
		SchedulerID: definition.NewScheduler.Name,
		Status:      game_room.GameStatusReady,
		LastPingAt:  time.Now(),
	}

	mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), definition.NewScheduler.Name, roomId).Return(&currentGameRoom, nil)

	currentGameRoomInstance := game_room.Instance{
		ID:          roomId,
		SchedulerID: definition.NewScheduler.Name,
	}

	newGameRoomInstance := game_room.Instance{
		ID:          fmt.Sprintf("new-%s", roomId),
		SchedulerID: definition.NewScheduler.Name,
	}

	newGameRoom := game_room.GameRoom{
		ID:          newGameRoomInstance.ID,
		SchedulerID: definition.NewScheduler.Name,
		Status:      game_room.GameStatusPending,
		Version:     newVersion,
	}

	mocks.portAllocator.EXPECT().Allocate(gomock.Any(), 1).Return([]int32{5000}, nil)
	mocks.runtime.EXPECT().CreateGameRoomInstance(context.Background(), definition.NewScheduler.Name, versionEq(newVersion)).Return(&newGameRoomInstance, nil)

	mocks.roomStorage.EXPECT().CreateRoom(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newVersion))).Return(nil)
	mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), newGameRoom.SchedulerID, newGameRoom.ID).Return(&newGameRoom, nil)

	// after started the watcher, we cancel the operation.
	roomStorageStatusWatcher := room_storage_mock.NewMockRoomStorageStatusWatcher(mockCtrl)
	mocks.roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newVersion))).DoAndReturn(
		func(_ context.Context, _ *game_room.GameRoom) (ports.RoomStorageStatusWatcher, error) {
			cancelFunc()
			return roomStorageStatusWatcher, nil
		},
	)

	// this will make the update to block until we produce into the statusChan
	statusChan := make(chan game_room.StatusEvent)
	roomStorageStatusWatcher.EXPECT().ResultChan().Return(statusChan)
	roomStorageStatusWatcher.EXPECT().Stop()

	mocks.instanceStorage.EXPECT().GetInstance(gomock.Any(), definition.NewScheduler.Name, roomId).Return(&currentGameRoomInstance, nil)
	mocks.runtime.EXPECT().DeleteGameRoomInstance(gomock.Any(), &currentGameRoomInstance).Return(nil)

	executor := NewExecutor(mocks.roomManager, mocks.schedulerManager)

	operationDone := make(chan error)
	go func() {
		operationDone <- executor.Execute(ctx, &operation.Operation{}, definition)
	}()

	require.Eventually(t, func() bool {
		select {
		case statusChan <- game_room.StatusEvent{Status: game_room.GameStatusReady}:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)

	var err error
	require.Eventually(t, func() bool {
		select {
		case err = <-operationDone:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)

	require.NoError(t, err)
}

func TestUpdateSchedulerExecutor_Execute_InvalidScheduler(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mocks := newMockRoomAndSchedulerManager(mockCtrl)

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	currentScheduler := newValidScheduler()
	currentScheduler.PortRange.Start = 5000

	newScheduler := entities.Scheduler{}
	definition := &UpdateSchedulerDefinition{
		NewScheduler: newScheduler,
	}

	executor := NewExecutor(mocks.roomManager, mocks.schedulerManager)
	err := executor.Execute(ctx, &operation.Operation{}, definition)
	require.Error(t, err)
}

func TestUpdateSchedulerExecutor_Execute_UpdateFails(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mocks := newMockRoomAndSchedulerManager(mockCtrl)

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	currentScheduler := newValidScheduler()
	currentScheduler.MaxSurge = "3"

	newScheduler := newValidScheduler()
	newScheduler.MaxSurge = "5"

	definition := &UpdateSchedulerDefinition{
		NewScheduler: newScheduler,
	}

	mocks.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), definition.NewScheduler.Name).Return(&currentScheduler, nil)
	mocks.schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(porterrors.ErrUnexpected)

	executor := NewExecutor(mocks.roomManager, mocks.schedulerManager)
	err := executor.Execute(ctx, &operation.Operation{}, definition)
	require.Error(t, err)
}

func TestUpdateSchedulerExecutor_Execute_MaxSurgeFails(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mocks := newMockRoomAndSchedulerManager(mockCtrl)

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	currentScheduler := newValidScheduler()
	currentScheduler.PortRange.Start = 5000
	currentScheduler.MaxSurge = "10%"

	newScheduler := newValidScheduler()
	newScheduler.PortRange.Start = 1000
	newScheduler.MaxSurge = "10%"

	definition := &UpdateSchedulerDefinition{
		NewScheduler: newScheduler,
	}

	mocks.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), definition.NewScheduler.Name).Return(&currentScheduler, nil)
	mocks.schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

	mocks.roomStorage.EXPECT().GetRoomCount(gomock.Any(), definition.NewScheduler.Name).Return(0, porterrors.ErrUnexpected)

	executor := NewExecutor(mocks.roomManager, mocks.schedulerManager)
	err := executor.Execute(ctx, &operation.Operation{}, definition)
	require.Error(t, err)
}

func TestUpdateSchedulerExecutor_Execute_FailedToListRoomsToBeReplaced(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mocks := newMockRoomAndSchedulerManager(mockCtrl)

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	currentScheduler := newValidScheduler()
	currentScheduler.PortRange.Start = 5000
	currentScheduler.MaxSurge = "3"

	newScheduler := newValidScheduler()
	newScheduler.PortRange.Start = 1000
	newScheduler.MaxSurge = "3"

	definition := &UpdateSchedulerDefinition{
		NewScheduler: newScheduler,
	}

	mocks.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), definition.NewScheduler.Name).Return(&currentScheduler, nil)
	mocks.schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

	mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewScheduler.Name, gomock.Any()).Return([]string{}, porterrors.ErrUnexpected)

	executor := NewExecutor(mocks.roomManager, mocks.schedulerManager)
	err := executor.Execute(ctx, &operation.Operation{}, definition)
	require.Error(t, err)
}

// mockRoomAndSchedulerManager struct that holds all the mocks necessary for the
// operation executor.
type mockRoomAndSchedulerManager struct {
	roomManager      *room_manager.RoomManager
	schedulerManager *scheduler_manager.SchedulerManager
	portAllocator    *port_allocator_mock.MockPortAllocator
	roomStorage      *room_storage_mock.MockRoomStorage
	instanceStorage  *instance_storage_mock.MockGameRoomInstanceStorage
	runtime          *runtime_mock.MockRuntime
	eventsForwarder  *events_forwarder_mock.MockEventsForwarder
	schedulerStorage *scheduler_storage_mock.MockSchedulerStorage
}

func newMockRoomAndSchedulerManager(mockCtrl *gomock.Controller) *mockRoomAndSchedulerManager {
	clock := clock_mock.NewFakeClock(time.Now())
	portAllocator := port_allocator_mock.NewMockPortAllocator(mockCtrl)
	roomStorage := room_storage_mock.NewMockRoomStorage(mockCtrl)
	instanceStorage := instance_storage_mock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtime_mock.NewMockRuntime(mockCtrl)
	eventsForwarder := events_forwarder_mock.NewMockEventsForwarder(mockCtrl)
	schedulerStorage := scheduler_storage_mock.NewMockSchedulerStorage(mockCtrl)

	config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Second * 2}
	roomManager := room_manager.NewRoomManager(clock, portAllocator, roomStorage, instanceStorage, runtime, eventsForwarder, config)
	schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

	return &mockRoomAndSchedulerManager{
		roomManager,
		schedulerManager,
		portAllocator,
		roomStorage,
		instanceStorage,
		runtime,
		eventsForwarder,
		schedulerStorage,
	}
}

func versionEq(version string) gomock.Matcher {
	return &gameRoomVersionMatcher{version}
}

func idEq(id string) gomock.Matcher {
	return &gameRoomIdMatcher{id}
}

// gameRoomIdMatcher matches the game room ID with the one provided.
type gameRoomIdMatcher struct {
	id string
}

func (m *gameRoomIdMatcher) Matches(x interface{}) bool {
	switch value := x.(type) {
	case game_room.GameRoom:
		return value.ID == m.id
	case *game_room.GameRoom:
		return value.ID == m.id
	default:
		return false
	}
}

func (m *gameRoomIdMatcher) String() string {
	return fmt.Sprintf("a game room with id \"%s\"", m.id)
}

// gameRoomVersionMatcher matches the game room version with the one provided.
type gameRoomVersionMatcher struct {
	version string
}

func (m *gameRoomVersionMatcher) Matches(x interface{}) bool {
	switch value := x.(type) {
	case game_room.Spec:
		return value.Version == m.version
	case *game_room.Spec:
		return value.Version == m.version
	case game_room.GameRoom:
		return value.Version == m.version
	case *game_room.GameRoom:
		return value.Version == m.version
	default:
		return false
	}
}

func (m *gameRoomVersionMatcher) String() string {
	return fmt.Sprintf("a game room with version \"%s\"", m.version)
}

// newValidScheduler generates a valid scheduler with the required fields.
// TODO(gabrielcorado): should we move this to the entities package as a
// "fixture"?
func newValidScheduler() entities.Scheduler {
	return entities.Scheduler{
		Name:            "scheduler",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "5",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Toleration:             "toleration",
			Affinity:               "affinity",
			Containers: []game_room.Container{
				{
					Name:            "default",
					Image:           "some-image",
					ImagePullPolicy: "Always",
					Command:         []string{"hello"},
					Ports: []game_room.ContainerPort{
						{Name: "tcp", Protocol: "tcp", Port: 80},
					},
					Requests: game_room.ContainerResources{
						CPU:    "10m",
						Memory: "100Mi",
					},
					Limits: game_room.ContainerResources{
						CPU:    "10m",
						Memory: "100Mi",
					},
				},
			},
		},
		PortRange: &entities.PortRange{
			Start: 40000,
			End:   60000,
		},
	}
}
