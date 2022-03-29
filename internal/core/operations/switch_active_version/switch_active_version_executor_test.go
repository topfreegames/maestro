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

package switch_active_version_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/topfreegames/maestro/internal/core/operations"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/operations/switch_active_version"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	instancestoragemock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	portallocatormock "github.com/topfreegames/maestro/internal/adapters/port_allocator/mock"
	runtimemock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/validations"
)

// mockRoomAndSchedulerManager struct that holds all the mocks necessary for the
// operation executor.
type mockRoomAndSchedulerManager struct {
	roomManager      *mockports.MockRoomManager
	schedulerManager *mockports.MockSchedulerManager
	portAllocator    *portallocatormock.MockPortAllocator
	roomStorage      *mockports.MockRoomStorage
	instanceStorage  *instancestoragemock.MockGameRoomInstanceStorage
	runtime          *runtimemock.MockRuntime
	eventsService    ports.EventsService
	schedulerStorage *mockports.MockSchedulerStorage
	schedulerCache   *mockports.MockSchedulerCache
}

func TestSwitchActiveVersionOperation_Execute(t *testing.T) {
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
		definition := &switch_active_version.SwitchActiveVersionDefinition{NewActiveVersion: newMajorScheduler.Spec.Version}

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
			mocks.roomManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoom, nil, nil)
		}
		mocks.roomManager.EXPECT().DeleteRoomAndWaitForRoomTerminated(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(len(append(gameRoomListCycle1, gameRoomListCycle2...)))

		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), newMajorScheduler).Return(nil)

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)
		require.Nil(t, execErr)
	})

	t.Run("should succeed - Execute switch active version operation not replacing pods", func(t *testing.T) {
		noReplaceDefinition := &switch_active_version.SwitchActiveVersionDefinition{NewActiveVersion: newMinorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMinorScheduler.Name, newMinorScheduler.Spec.Version).Return(newMinorScheduler, nil)
		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMinorScheduler.Name}, noReplaceDefinition)
		require.Nil(t, execErr)
	})

	t.Run("should succeed - Execute switch active version operation (no running rooms)", func(t *testing.T) {
		definition := &switch_active_version.SwitchActiveVersionDefinition{NewActiveVersion: newMajorScheduler.Spec.Version}

		mocks := newMockRoomAndSchedulerManager(mockCtrl)
		mocks.roomManager.EXPECT().SchedulerMaxSurge(gomock.Any(), gomock.Any()).Return(maxSurge, nil)

		var emptyGameRoom []*game_room.GameRoom
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(emptyGameRoom, nil)

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)
		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: activeScheduler.Name}, definition)
		require.Nil(t, execErr)
	})

	t.Run("should fail - Can't update scheduler (switch active version on database)", func(t *testing.T) {
		noReplaceDefinition := &switch_active_version.SwitchActiveVersionDefinition{NewActiveVersion: newMinorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMinorScheduler.Name, newMinorScheduler.Spec.Version).Return(newMinorScheduler, nil)
		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(errors.New("error"))

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMinorScheduler.Name}, noReplaceDefinition)
		require.NotNil(t, execErr)
		require.Equal(t, operations.ErrKindUnexpected, execErr.Kind())
	})

	t.Run("should fail - Can't create room", func(t *testing.T) {
		definition := &switch_active_version.SwitchActiveVersionDefinition{NewActiveVersion: newMajorScheduler.Spec.Version}
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
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle1, nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*game_room.GameRoom{}, nil).MaxTimes(1)

		mocks.roomManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("error")).MaxTimes(maxSurge)

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)
		require.NotNil(t, execErr)
		require.Equal(t, operations.ErrKindUnexpected, execErr.Kind())
	})

	t.Run("should fail - Can't delete room", func(t *testing.T) {
		definition := &switch_active_version.SwitchActiveVersionDefinition{NewActiveVersion: newMajorScheduler.Spec.Version}
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
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomListCycle1, nil)
		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*game_room.GameRoom{}, nil).MaxTimes(1)

		mocks.roomManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil).MaxTimes(maxSurge)
		mocks.roomManager.EXPECT().DeleteRoomAndWaitForRoomTerminated(gomock.Any(), gomock.Any()).Return(errors.New("error")).MaxTimes(maxSurge)

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)
		require.NotNil(t, execErr)
		require.Equal(t, operations.ErrKindUnexpected, execErr.Kind())
	})

	t.Run("should fail - Can't find max surge", func(t *testing.T) {
		definition := &switch_active_version.SwitchActiveVersionDefinition{NewActiveVersion: newMajorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)
		mocks.roomManager.EXPECT().SchedulerMaxSurge(gomock.Any(), gomock.Any()).Return(0, errors.New("error"))

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)
		require.NotNil(t, execErr)
		require.Equal(t, operations.ErrKindUnexpected, execErr.Kind())
	})

	t.Run("should fail - Can't list rooms to delete", func(t *testing.T) {
		definition := &switch_active_version.SwitchActiveVersionDefinition{NewActiveVersion: newMajorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(activeScheduler, nil)
		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)

		mocks.roomManager.EXPECT().SchedulerMaxSurge(gomock.Any(), gomock.Any()).Return(maxSurge, nil)

		mocks.roomManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)
		require.NotNil(t, execErr)
		require.Equal(t, operations.ErrKindUnexpected, execErr.Kind())
	})

	t.Run("should fail - Can't get new scheduler", func(t *testing.T) {
		definition := &switch_active_version.SwitchActiveVersionDefinition{NewActiveVersion: newMajorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(nil, errors.New("error"))

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)
		require.NotNil(t, execErr)
		require.Equal(t, operations.ErrKindUnexpected, execErr.Kind())
	})

	t.Run("should fail - Can't get active scheduler", func(t *testing.T) {
		definition := &switch_active_version.SwitchActiveVersionDefinition{NewActiveVersion: newMajorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(gomock.Any(), newMajorScheduler.Name, newMajorScheduler.Spec.Version).Return(newMajorScheduler, nil)
		mocks.schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), activeScheduler.Name).Return(nil, errors.New("error"))

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)
		require.NotNil(t, execErr)
		require.Equal(t, operations.ErrKindUnexpected, execErr.Kind())
	})
}

func TestSwitchActiveVersionOperation_Rollback(t *testing.T) {
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

	definition := &switch_active_version.SwitchActiveVersionDefinition{NewActiveVersion: newMajorScheduler.Spec.Version}

	t.Run("should succeed - Execute on error if operation finishes (no created rooms)", func(t *testing.T) {
		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
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
			mocks.roomManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoom, nil, nil)
			mocks.roomManager.EXPECT().DeleteRoomAndWaitForRoomTerminated(gomock.Any(), gomock.Any()).Return(nil)
		}

		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(errors.New("error"))

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		op := &operation.Operation{
			ID:             "op",
			DefinitionName: definition.Name(),
			SchedulerName:  newMajorScheduler.Name,
			CreatedAt:      time.Now(),
		}
		execErr := executor.Execute(context.Background(), op, definition)
		require.NotNil(t, execErr)
		require.Equal(t, operations.ErrKindUnexpected, execErr.Kind())

		for range append(gameRoomListCycle1, gameRoomListCycle2...) {
			mocks.roomManager.EXPECT().DeleteRoomAndWaitForRoomTerminated(gomock.Any(), gomock.Any()).Return(nil)
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
			mocks.roomManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoom, nil, nil)
			mocks.roomManager.EXPECT().DeleteRoomAndWaitForRoomTerminated(gomock.Any(), gomock.Any()).Return(nil)
		}

		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(errors.New("error"))

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		op := &operation.Operation{
			ID:             "op",
			DefinitionName: definition.Name(),
			SchedulerName:  newMajorScheduler.Name,
			CreatedAt:      time.Now(),
		}
		execErr := executor.Execute(context.Background(), op, definition)
		require.NotNil(t, execErr)
		require.Equal(t, operations.ErrKindUnexpected, execErr.Kind())

		mocks.roomManager.EXPECT().DeleteRoomAndWaitForRoomTerminated(gomock.Any(), gomock.Any()).Return(errors.New("error"))

		err = executor.Rollback(context.Background(), op, definition, nil)
		require.Error(t, err)
	})
}

func newMockRoomAndSchedulerManager(mockCtrl *gomock.Controller) *mockRoomAndSchedulerManager {
	portAllocator := portallocatormock.NewMockPortAllocator(mockCtrl)
	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	instanceStorage := instancestoragemock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	eventsForwarderService := mockports.NewMockEventsService(mockCtrl)
	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
	schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)

	roomManager := mockports.NewMockRoomManager(mockCtrl)
	schedulerManager := mockports.NewMockSchedulerManager(mockCtrl)

	return &mockRoomAndSchedulerManager{
		roomManager,
		schedulerManager,
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
		State:           entities.StateCreating,
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
