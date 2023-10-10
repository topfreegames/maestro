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

package switchversion_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/operations/rooms/remove"
	"github.com/topfreegames/maestro/internal/core/operations/schedulers/switchversion"
	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/validations"
)

// mockRoomAndSchedulerManager struct that holds all the mocks necessary for the
// operation executor.
type mockRoomAndSchedulerAndOperationManager struct {
	roomManager      *mockports.MockRoomManager
	schedulerManager *mockports.MockSchedulerManager
	operationManager *mockports.MockOperationManager
	portAllocator    *mockports.MockPortAllocator
	roomStorage      *mockports.MockRoomStorage
	instanceStorage  *mockports.MockGameRoomInstanceStorage
	runtime          *mockports.MockRuntime
	eventsService    ports.EventsService
	schedulerStorage *mockports.MockSchedulerStorage
	schedulerCache   *mockports.MockSchedulerCache
}

func TestExecutor_Execute(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	newMajorScheduler := newValidSchedulerV2()
	newMajorScheduler.PortRange.Start = 1000
	newMajorScheduler.MaxSurge = "3"

	newMinorScheduler := newValidSchedulerV2()
	newMinorScheduler.Spec.Version = "v1.1.0"

	activeScheduler := newValidSchedulerV2()
	activeScheduler.Spec.Version = "v1.0.0"

	maxSurge := 3

	t.Run("should succeed - Execute switch active version operation replacing pods", func(t *testing.T) {
		definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}

		mocks := newMockRoomAndSchedulerManager(mockCtrl)
		mocks.roomManager.EXPECT().SchedulerMaxSurge(gomock.Any(), gomock.Any()).Return(3, nil)

		var gameRoomListCycle1 []*game_room.GameRoom
		var gameRoomListCycle2 []*game_room.GameRoom
		var gameRoomListCycle3 []*game_room.GameRoom
		for i := 0; i < maxSurge; i++ {
			gameRoomListCycle1 = append(gameRoomListCycle1, &game_room.GameRoom{
				ID:          fmt.Sprintf("room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			})
		}
		for i := maxSurge; i < maxSurge*2; i++ {
			gameRoomListCycle2 = append(gameRoomListCycle2, &game_room.GameRoom{
				ID:          fmt.Sprintf("room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			})
		}

		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(context.Background(), newMajorScheduler.Name, definition.NewActiveVersion).Return(newMajorScheduler, nil)
		mocks.schedulerManager.EXPECT().GetActiveScheduler(context.Background(), newMajorScheduler.Name).Return(activeScheduler, nil)

		mocks.roomStorage.EXPECT().GetRoomCount(gomock.Any(), newMajorScheduler.Name).Return(len(append(gameRoomListCycle1, gameRoomListCycle2...)), nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle1, nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle2, nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle3, nil)

		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any()).Times(4)

		for i := range append(gameRoomListCycle1, gameRoomListCycle2...) {
			gameRoom := &game_room.GameRoom{
				ID:          fmt.Sprintf("new-room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			}
			mocks.roomManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoom, nil, nil)
		}
		mocks.roomManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any(), remove.SwitchVersionReplace).Return(nil).MaxTimes(len(append(gameRoomListCycle1, gameRoomListCycle2...)))

		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), newMajorScheduler).Return(nil)

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)
		require.Nil(t, execErr)
	})

	t.Run("should succeed - Execute switch active version operation not replacing pods", func(t *testing.T) {
		noReplaceDefinition := &switchversion.Definition{NewActiveVersion: newMinorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMinorScheduler.Name, newMinorScheduler.Spec.Version).Return(newMinorScheduler, nil)
		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMinorScheduler.Name}, noReplaceDefinition)
		require.Nil(t, execErr)
	})

	t.Run("should succeed - Execute switch active version operation (no running rooms)", func(t *testing.T) {
		definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}

		mocks := newMockRoomAndSchedulerManager(mockCtrl)
		mocks.roomManager.EXPECT().SchedulerMaxSurge(gomock.Any(), gomock.Any()).Return(maxSurge, nil)

		var emptyGameRoom []*game_room.GameRoom
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(emptyGameRoom, nil)

		mocks.roomStorage.EXPECT().GetRoomCount(gomock.Any(), newMajorScheduler.Name).Return(0, nil)

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)
		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: activeScheduler.Name}, definition)
		require.Nil(t, execErr)
	})

	t.Run("should succeed - Can't create room", func(t *testing.T) {
		definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)

		mocks.roomManager.EXPECT().SchedulerMaxSurge(gomock.Any(), gomock.Any()).Return(3, nil)

		var gameRoomListCycle1 []*game_room.GameRoom
		for i := 0; i < maxSurge; i++ {
			gameRoomListCycle1 = append(gameRoomListCycle1, &game_room.GameRoom{
				ID:          fmt.Sprintf("room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			})
		}

		mocks.roomStorage.EXPECT().GetRoomCount(gomock.Any(), newMajorScheduler.Name).Return(len(gameRoomListCycle1), nil)
		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any()).Times(3)

		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle1, nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*game_room.GameRoom{}, nil).MaxTimes(1)

		mocks.roomManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("error")).MaxTimes(maxSurge)
		mocks.roomManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any(), remove.SwitchVersionReplace).Return(nil).MaxTimes(maxSurge)

		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)
		require.Nil(t, execErr)
	})

	t.Run("should fail - Can't update scheduler (switch active version on database)", func(t *testing.T) {
		noReplaceDefinition := &switchversion.Definition{NewActiveVersion: newMinorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)
		op := &operation.Operation{SchedulerName: newMinorScheduler.Name}

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMinorScheduler.Name, newMinorScheduler.Spec.Version).Return(newMinorScheduler, nil)
		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(errors.New("error"))
		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), op, "error updating scheduler with new active version: error")

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		execErr := executor.Execute(context.Background(), op, noReplaceDefinition)
		require.NotNil(t, execErr)
	})

	t.Run("should fail - Can't delete room", func(t *testing.T) {
		definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)
		mocks.roomManager.EXPECT().SchedulerMaxSurge(gomock.Any(), gomock.Any()).Return(maxSurge, nil)

		var gameRoomListCycle1 []*game_room.GameRoom
		for i := 0; i < maxSurge; i++ {
			gameRoomListCycle1 = append(gameRoomListCycle1, &game_room.GameRoom{
				ID:          fmt.Sprintf("room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			})
		}

		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(3)

		mocks.roomStorage.EXPECT().GetRoomCount(gomock.Any(), newMajorScheduler.Name).Return(len(gameRoomListCycle1), nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle1, nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*game_room.GameRoom{}, nil).MaxTimes(1)

		mocks.roomManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil).MaxTimes(maxSurge)
		mocks.roomManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any(), remove.SwitchVersionReplace).Return(errors.New("error")).MaxTimes(maxSurge)

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)
		require.NotNil(t, execErr)
	})

	t.Run("should fail - Can't find max surge", func(t *testing.T) {
		definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}
		op := &operation.Operation{SchedulerName: newMajorScheduler.Name}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.roomManager.EXPECT().SchedulerMaxSurge(gomock.Any(), gomock.Any()).Return(0, errors.New("error"))

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)

		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), op, "error fetching scheduler max surge: error")

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		execErr := executor.Execute(context.Background(), op, definition)
		require.NotNil(t, execErr)
	})

	t.Run("should fail - Can't list rooms to delete", func(t *testing.T) {
		definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}
		op := &operation.Operation{SchedulerName: newMajorScheduler.Name}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)

		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), op, "error replacing rooms: failed to list rooms for deletion")

		mocks.roomManager.EXPECT().SchedulerMaxSurge(gomock.Any(), gomock.Any()).Return(maxSurge, nil)

		mocks.roomStorage.EXPECT().GetRoomCount(gomock.Any(), newMajorScheduler.Name).Return(0, nil)

		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("error")).Times(10)

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		execErr := executor.Execute(context.Background(), op, definition)
		require.NotNil(t, execErr)
	})

	t.Run("should fail - Can't count total rooms amount", func(t *testing.T) {
		definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}
		op := &operation.Operation{SchedulerName: newMajorScheduler.Name}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)

		mocks.roomManager.EXPECT().SchedulerMaxSurge(gomock.Any(), gomock.Any()).Return(maxSurge, nil)

		mocks.roomStorage.EXPECT().GetRoomCount(gomock.Any(), newMajorScheduler.Name).Return(0, errors.New("error")).Times(10)

		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), op, gomock.Any())

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)
		require.NotNil(t, execErr)
	})

	t.Run("should fail - Can't get new scheduler", func(t *testing.T) {
		definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}
		op := &operation.Operation{SchedulerName: newMajorScheduler.Name}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(nil, errors.New("error"))

		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), op, "error fetching scheduler version to be switched to: error")

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		execErr := executor.Execute(context.Background(), op, definition)
		require.NotNil(t, execErr)
	})

	t.Run("should fail - Can't get active scheduler", func(t *testing.T) {
		definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}
		op := &operation.Operation{SchedulerName: newMajorScheduler.Name}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)
		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(nil, errors.New("error"))

		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), op, "error deciding if should replace pods: error")

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		execErr := executor.Execute(context.Background(), op, definition)
		require.NotNil(t, execErr)
	})
}

func TestExecutor_Rollback(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mocks := newMockRoomAndSchedulerManager(mockCtrl)

	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	newMajorScheduler := newValidSchedulerV2()
	newMajorScheduler.PortRange.Start = 1000
	newMajorScheduler.MaxSurge = "3"

	newMinorScheduler := newValidSchedulerV2()
	newMinorScheduler.Spec.Version = "v1.1.0"

	activeScheduler := newValidSchedulerV2()
	activeScheduler.Spec.Version = "v1.0.0"

	maxSurge := 3

	definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}

	t.Run("should succeed - Execute on error if operation finishes (no created rooms)", func(t *testing.T) {
		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		err = executor.Rollback(context.Background(), &operation.Operation{}, definition, nil)
		require.NoError(t, err)
	})

	t.Run("should succeed - Execute on error if operation finishes (created rooms)", func(t *testing.T) {

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)
		mocks.roomManager.EXPECT().SchedulerMaxSurge(gomock.Any(), gomock.Any()).Return(3, nil)

		var gameRoomListCycle1 []*game_room.GameRoom
		var gameRoomListCycle2 []*game_room.GameRoom
		var gameRoomListCycle3 []*game_room.GameRoom
		for i := 0; i < maxSurge; i++ {
			gameRoomListCycle1 = append(gameRoomListCycle1, &game_room.GameRoom{
				ID:          fmt.Sprintf("room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			})
		}
		for i := maxSurge; i < maxSurge*2; i++ {
			gameRoomListCycle2 = append(gameRoomListCycle2, &game_room.GameRoom{
				ID:          fmt.Sprintf("room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			})
		}

		mocks.roomStorage.EXPECT().GetRoomCount(gomock.Any(), newMajorScheduler.Name).Return(len(append(gameRoomListCycle1, gameRoomListCycle2...)), nil)

		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any()).Times(5)

		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle1, nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle2, nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle3, nil)

		for i := range append(gameRoomListCycle1, gameRoomListCycle2...) {
			gameRoom := &game_room.GameRoom{
				ID:          fmt.Sprintf("new-room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			}
			mocks.roomManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoom, nil, nil)
			mocks.roomManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any(), remove.SwitchVersionReplace).Return(nil)
		}

		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(errors.New("error"))

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		op := &operation.Operation{
			ID:             "op",
			DefinitionName: definition.Name(),
			SchedulerName:  newMajorScheduler.Name,
			CreatedAt:      time.Now(),
		}
		execErr := executor.Execute(context.Background(), op, definition)
		require.NotNil(t, execErr)

		for range append(gameRoomListCycle1, gameRoomListCycle2...) {
			mocks.roomManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any(), remove.SwitchVersionRollback).Return(nil)
		}

		err = executor.Rollback(context.Background(), op, definition, nil)
		require.NoError(t, err)
	})

	t.Run("should succeed - a create room fail, then a delete room fails and triggers rollback", func(t *testing.T) {

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)
		mocks.roomManager.EXPECT().SchedulerMaxSurge(gomock.Any(), gomock.Any()).Return(3, nil)

		var gameRoomListCycle1 []*game_room.GameRoom
		var gameRoomListCycle2 []*game_room.GameRoom
		var gameRoomListCycle3 []*game_room.GameRoom
		for i := 0; i < maxSurge; i++ {
			gameRoomListCycle1 = append(gameRoomListCycle1, &game_room.GameRoom{
				ID:          fmt.Sprintf("room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			})
		}
		for i := maxSurge; i < maxSurge*2; i++ {
			gameRoomListCycle2 = append(gameRoomListCycle2, &game_room.GameRoom{
				ID:          fmt.Sprintf("room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			})
		}

		mocks.roomStorage.EXPECT().GetRoomCount(gomock.Any(), newMajorScheduler.Name).Return(len(append(gameRoomListCycle1, gameRoomListCycle2...)), nil)
		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any()).MinTimes(0)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle1, nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle2, nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle3, nil)

		allRooms := append(gameRoomListCycle1, gameRoomListCycle2...)
		for i := range allRooms {
			if i == len(allRooms)-1 {
				mocks.roomManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("error"))
				mocks.roomManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any(), remove.SwitchVersionRollback).Return(errors.New("error"))
				continue
			}
			gameRoom := &game_room.GameRoom{
				ID:          fmt.Sprintf("new-room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			}
			mocks.roomManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoom, nil, nil)
			mocks.roomManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any(), remove.SwitchVersionRollback).Return(nil)
		}

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		op := &operation.Operation{
			ID:             "op",
			DefinitionName: definition.Name(),
			SchedulerName:  newMajorScheduler.Name,
			CreatedAt:      time.Now(),
		}
		execErr := executor.Execute(context.Background(), op, definition)
		require.NotNil(t, execErr)

		for i := range allRooms {
			if i == len(allRooms)-1 {
				continue
			}
			mocks.roomManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any(), remove.SwitchVersionRollback).Return(nil)
		}

		err = executor.Rollback(context.Background(), op, definition, nil)
		require.NoError(t, err)
	})

	t.Run("should fail - error deleting rooms", func(t *testing.T) {
		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)
		mocks.roomManager.EXPECT().SchedulerMaxSurge(gomock.Any(), gomock.Any()).Return(maxSurge, nil)

		var gameRoomListCycle1 []*game_room.GameRoom
		var gameRoomListCycle2 []*game_room.GameRoom
		var gameRoomListCycle3 []*game_room.GameRoom
		for i := 0; i < maxSurge; i++ {
			gameRoomListCycle1 = append(gameRoomListCycle1, &game_room.GameRoom{
				ID:          fmt.Sprintf("room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			})
		}
		for i := maxSurge; i < maxSurge*2; i++ {
			gameRoomListCycle2 = append(gameRoomListCycle2, &game_room.GameRoom{
				ID:          fmt.Sprintf("room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			})
		}
		mocks.roomStorage.EXPECT().GetRoomCount(gomock.Any(), newMajorScheduler.Name).Return(len(append(gameRoomListCycle1, gameRoomListCycle2...)), nil)
		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any()).MinTimes(0)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle1, nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle2, nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle3, nil)

		for i := range append(gameRoomListCycle1, gameRoomListCycle2...) {
			gameRoom := &game_room.GameRoom{
				ID:          fmt.Sprintf("new-room-%v", i),
				SchedulerID: newMajorScheduler.Name,
				Version:     activeScheduler.Spec.Version,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			}
			mocks.roomManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoom, nil, nil)
			mocks.roomManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any(), remove.SwitchVersionRollback).Return(nil)
		}

		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(errors.New("error"))

		executor := switchversion.NewExecutor(mocks.roomManager, mocks.schedulerManager, mocks.operationManager, mocks.roomStorage)
		op := &operation.Operation{
			ID:             "op",
			DefinitionName: definition.Name(),
			SchedulerName:  newMajorScheduler.Name,
			CreatedAt:      time.Now(),
		}
		execErr := executor.Execute(context.Background(), op, definition)
		require.NotNil(t, execErr)

		mocks.roomManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any(), remove.SwitchVersionRollback).Return(errors.New("error"))

		err = executor.Rollback(context.Background(), op, definition, nil)
		require.Error(t, err)
	})
}

func newMockRoomAndSchedulerManager(mockCtrl *gomock.Controller) *mockRoomAndSchedulerAndOperationManager {
	portAllocator := mockports.NewMockPortAllocator(mockCtrl)
	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	instanceStorage := mockports.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := mockports.NewMockRuntime(mockCtrl)
	eventsForwarderService := mockports.NewMockEventsService(mockCtrl)
	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
	schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)

	roomManager := mockports.NewMockRoomManager(mockCtrl)
	schedulerManager := mockports.NewMockSchedulerManager(mockCtrl)
	operationManager := mockports.NewMockOperationManager(mockCtrl)

	return &mockRoomAndSchedulerAndOperationManager{
		roomManager,
		schedulerManager,
		operationManager,
		portAllocator,
		roomStorage,
		instanceStorage,
		runtime,
		eventsForwarderService,
		schedulerStorage,
		schedulerCache,
	}
}

func newValidSchedulerV2() *entities.Scheduler {
	return &entities.Scheduler{
		Name:            "scheduler",
		Game:            "game",
		State:           entities.StateInSync,
		MaxSurge:        "5",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v2",
			TerminationGracePeriod: 60,
			Toleration:             "toleration",
			Affinity:               "affinity",
			Containers: []game_room.Container{
				{
					Name:            "default",
					Image:           "some-image",
					ImagePullPolicy: "IfNotPresent",
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
