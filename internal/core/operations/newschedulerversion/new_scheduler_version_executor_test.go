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

package newschedulerversion_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/services/room_manager"

	"github.com/topfreegames/maestro/internal/core/operations/add_rooms"

	"github.com/topfreegames/maestro/internal/core/ports/errors"

	"github.com/topfreegames/maestro/internal/validations"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/operations/newschedulerversion"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"

	"github.com/golang/mock/gomock"
	clockmock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	instancestoragemock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	portallocatormock "github.com/topfreegames/maestro/internal/adapters/port_allocator/mock"
	runtimemock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
	schedulerstoragemock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/core/services/interfaces"
	mockeventsservice "github.com/topfreegames/maestro/internal/core/services/interfaces/mock/events_service"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"
)

func TestCreateNewSchedulerVersionExecutor_Execute(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	t.Run("given a major version update when the game room is valid (creation and deletion succeeds) then it returns no error and enqueue switch active version op", func(t *testing.T) {
		mocksForExecutor := newMockRoomAndSchedulerManager(mockCtrl)
		currentActiveScheduler := newValidScheduler("v1.0")
		newScheduler := *newValidScheduler("v1.2")
		operation := &operation.Operation{
			ID:             "123",
			Status:         operation.StatusInProgress,
			DefinitionName: newschedulerversion.OperationName,
			SchedulerName:  newScheduler.Name,
		}
		operationDef := &newschedulerversion.CreateNewSchedulerVersionDefinition{NewScheduler: &newScheduler}
		newGameRoomInstance := &game_room.Instance{
			ID:          "new-room-1",
			SchedulerID: newScheduler.Name,
		}
		createdGameRoom := &game_room.GameRoom{
			ID:          newGameRoomInstance.ID,
			SchedulerID: newScheduler.Name,
			Version:     newScheduler.Spec.Version,
			Status:      game_room.GameStatusReady,
			PingStatus:  game_room.GameRoomPingStatusReady,
		}
		deletedGameRoom := &game_room.GameRoom{
			ID:          newGameRoomInstance.ID,
			SchedulerID: newScheduler.Name,
			Version:     newScheduler.Spec.Version,
			Status:      game_room.GameStatusTerminating,
			PingStatus:  game_room.GameRoomPingStatusTerminating,
		}
		executor := newschedulerversion.NewExecutor(mocksForExecutor.roomManager, mocksForExecutor.schedulerManager)

		newSchedulerWithNewVersion := newScheduler
		newSchedulerWithNewVersion.Spec.Version = "v2.0.0"
		newSchedulerWithNewVersion.RollbackVersion = "v1.0.0"

		// mocks for SchedulerManager CreateNewSchedulerVersion method
		mocksForExecutor.schedulerStorage.EXPECT().RunWithTransaction(gomock.Any(), gomock.Any())

		// mocks for RoomManager CreateRoomAndWaitForReadiness
		mocksForExecutor.portAllocator.EXPECT().Allocate(gomock.Any(), gomock.Any()).Return([]int32{8080}, nil)
		mocksForExecutor.runtime.EXPECT().CreateGameRoomInstance(gomock.Any(), newScheduler.Name, newSchedulerWithNewVersion.Spec).Return(newGameRoomInstance, nil)
		mocksForExecutor.roomStorage.EXPECT().CreateRoom(gomock.Any(), gomock.Any())
		watcher := mockports.NewMockRoomStorageStatusWatcher(mockCtrl)
		mocksForExecutor.roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.Any()).Return(watcher, nil)
		watcher.EXPECT().Stop()
		mocksForExecutor.roomStorage.EXPECT().GetRoom(gomock.Any(), newScheduler.Name, newGameRoomInstance.ID).Return(createdGameRoom, nil)

		// mocks for RoomManager DeleteRoomAndWaitForRoomTerminated
		mocksForExecutor.instanceStorage.EXPECT().GetInstance(gomock.Any(), newScheduler.Name, createdGameRoom.ID).Return(newGameRoomInstance, nil)
		mocksForExecutor.runtime.EXPECT().DeleteGameRoomInstance(gomock.Any(), newGameRoomInstance)
		mocksForExecutor.roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.Any()).Return(watcher, nil)
		watcher.EXPECT().Stop()
		mocksForExecutor.roomStorage.EXPECT().GetRoom(gomock.Any(), newScheduler.Name, newGameRoomInstance.ID).Return(deletedGameRoom, nil)

		// mocks for SchedulerManager GetActiveScheduler method
		mocksForExecutor.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), newScheduler.Name).Return(currentActiveScheduler, nil)

		result := executor.Execute(context.Background(), operation, operationDef)

		require.NoError(t, result)
	})

	t.Run("given a major version update when the game room is invalid then it returns error and don't create new version nor switch to it", func(t *testing.T) {
		mocksForExecutor := newMockRoomAndSchedulerManager(mockCtrl)
		currentActiveScheduler := newValidScheduler("v1.0")
		newScheduler := *newValidScheduler("v1.2")
		operation := &operation.Operation{
			ID:             "123",
			Status:         operation.StatusInProgress,
			DefinitionName: newschedulerversion.OperationName,
			SchedulerName:  newScheduler.Name,
		}
		operationDef := &newschedulerversion.CreateNewSchedulerVersionDefinition{NewScheduler: &newScheduler}

		executor := newschedulerversion.NewExecutor(mocksForExecutor.roomManager, mocksForExecutor.schedulerManager)

		newSchedulerWithNewVersion := newScheduler
		newSchedulerWithNewVersion.Spec.Version = "v2.0.0"
		newSchedulerWithNewVersion.RollbackVersion = "v1.0.0"

		// mocks for RoomManager CreateRoomAndWaitForReadiness
		mocksForExecutor.portAllocator.EXPECT().Allocate(gomock.Any(), gomock.Any()).Return([]int32{8080}, errors.NewErrUnexpected("some error"))

		// mocks for SchedulerManager GetActiveScheduler method
		mocksForExecutor.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), newScheduler.Name).Return(currentActiveScheduler, nil)

		result := executor.Execute(context.Background(), operation, operationDef)

		require.EqualError(t, result, "error creating new game room for validating new version: some error")
	})

	t.Run("given a minor version update it returns no error and enqueue switch active version op", func(t *testing.T) {
		mocksForExecutor := newMockRoomAndSchedulerManager(mockCtrl)
		currentActiveScheduler := newValidScheduler("v1.0")
		newScheduler := *newValidScheduler("v1.0")
		operation := &operation.Operation{
			ID:             "123",
			Status:         operation.StatusInProgress,
			DefinitionName: newschedulerversion.OperationName,
			SchedulerName:  newScheduler.Name,
		}
		operationDef := &newschedulerversion.CreateNewSchedulerVersionDefinition{NewScheduler: &newScheduler}

		executor := newschedulerversion.NewExecutor(mocksForExecutor.roomManager, mocksForExecutor.schedulerManager)
		newSchedulerWithNewVersion := newScheduler
		newSchedulerWithNewVersion.Spec.Version = "v1.1.0"
		newSchedulerWithNewVersion.RollbackVersion = "v1.0.0"

		// mocks for SchedulerManager GetActiveScheduler method
		mocksForExecutor.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), newScheduler.Name).Return(currentActiveScheduler, nil)

		// mocks for SchedulerManager CreateNewSchedulerVersion method
		mocksForExecutor.schedulerStorage.EXPECT().RunWithTransaction(gomock.Any(), gomock.Any())

		result := executor.Execute(context.Background(), operation, operationDef)

		require.NoError(t, result)
	})

	t.Run("given a valid scheduler when some error occurs while creating a new version in db or enqueueing switch version it returns error and don't create new version nor switch to it", func(t *testing.T) {
		mocksForExecutor := newMockRoomAndSchedulerManager(mockCtrl)
		currentActiveScheduler := newValidScheduler("v1.0")
		newScheduler := *newValidScheduler("v1.0")
		operation := &operation.Operation{
			ID:             "123",
			Status:         operation.StatusInProgress,
			DefinitionName: newschedulerversion.OperationName,
			SchedulerName:  newScheduler.Name,
		}
		operationDef := &newschedulerversion.CreateNewSchedulerVersionDefinition{NewScheduler: &newScheduler}

		executor := newschedulerversion.NewExecutor(mocksForExecutor.roomManager, mocksForExecutor.schedulerManager)
		newSchedulerWithNewVersion := newScheduler
		newSchedulerWithNewVersion.Spec.Version = "v1.1.0"
		newSchedulerWithNewVersion.RollbackVersion = "v1.0.0"

		// mocks for SchedulerManager GetActiveScheduler method
		mocksForExecutor.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), newScheduler.Name).Return(currentActiveScheduler, nil)

		// mocks for SchedulerManager CreateNewSchedulerVersion method
		mocksForExecutor.schedulerStorage.EXPECT().RunWithTransaction(gomock.Any(), gomock.Any()).Return(errors.NewErrUnexpected("some error"))

		result := executor.Execute(context.Background(), operation, operationDef)

		require.EqualError(t, result, "error creating new scheduler version in db: some error")
	})

	t.Run("given a valid scheduler when some error occurs while retrieving the current active scheduler it returns error and don't create new version", func(t *testing.T) {
		mocksForExecutor := newMockRoomAndSchedulerManager(mockCtrl)
		newScheduler := *newValidScheduler("v1.0")
		operation := &operation.Operation{
			ID:             "123",
			Status:         operation.StatusInProgress,
			DefinitionName: newschedulerversion.OperationName,
			SchedulerName:  newScheduler.Name,
		}
		operationDef := &newschedulerversion.CreateNewSchedulerVersionDefinition{NewScheduler: &newScheduler}

		executor := newschedulerversion.NewExecutor(mocksForExecutor.roomManager, mocksForExecutor.schedulerManager)
		newSchedulerWithNewVersion := newScheduler
		newSchedulerWithNewVersion.Spec.Version = "v1.1.0"
		newSchedulerWithNewVersion.RollbackVersion = "v1.0.0"

		// mocks for SchedulerManager GetActiveScheduler method
		mocksForExecutor.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), newScheduler.Name).Return(nil, errors.NewErrUnexpected("some error"))

		result := executor.Execute(context.Background(), operation, operationDef)

		require.EqualError(t, result, "error getting active scheduler: some error")
	})

	t.Run("given a valid scheduler when the provided operation definition is not CreateNewSchedulerVersionDefinition it returns error and don't create new version", func(t *testing.T) {
		mocksForExecutor := newMockRoomAndSchedulerManager(mockCtrl)
		newScheduler := *newValidScheduler("v1.0")
		operation := &operation.Operation{
			ID:             "123",
			Status:         operation.StatusInProgress,
			DefinitionName: newschedulerversion.OperationName,
			SchedulerName:  newScheduler.Name,
		}
		operationDef := &add_rooms.AddRoomsDefinition{}

		executor := newschedulerversion.NewExecutor(mocksForExecutor.roomManager, mocksForExecutor.schedulerManager)
		newSchedulerWithNewVersion := newScheduler
		newSchedulerWithNewVersion.Spec.Version = "v1.1.0"
		newSchedulerWithNewVersion.RollbackVersion = "v1.0.0"

		result := executor.Execute(context.Background(), operation, operationDef)

		require.EqualError(t, result, "invalid operation definition for create_new_scheduler_version operation")
	})

	t.Run("given a invalid scheduler when the version parse fails it returns error and don't create new version", func(t *testing.T) {
		mocksForExecutor := newMockRoomAndSchedulerManager(mockCtrl)
		newScheduler := newInValidScheduler()
		currentActiveScheduler := newInValidScheduler()

		operation := &operation.Operation{
			ID:             "123",
			Status:         operation.StatusInProgress,
			DefinitionName: newschedulerversion.OperationName,
			SchedulerName:  newScheduler.Name,
		}
		operationDef := &newschedulerversion.CreateNewSchedulerVersionDefinition{NewScheduler: newScheduler}

		executor := newschedulerversion.NewExecutor(mocksForExecutor.roomManager, mocksForExecutor.schedulerManager)

		// mocks for SchedulerManager GetActiveScheduler method
		mocksForExecutor.schedulerStorage.EXPECT().GetScheduler(gomock.Any(), newScheduler.Name).Return(currentActiveScheduler, nil)

		result := executor.Execute(context.Background(), operation, operationDef)

		require.EqualError(t, result, "failed to parse scheduler current version: Invalid Semantic Version")
	})

}

// mockRoomAndSchedulerManager struct that holds all the mocks necessary for the
// operation executor.
type mockRoomAndSchedulerManager struct {
	roomManager      *room_manager.RoomManager
	schedulerManager *scheduler_manager.SchedulerManager
	operationFlow    *mockports.MockOperationFlow
	operationStorage *mockports.MockOperationStorage
	portAllocator    *portallocatormock.MockPortAllocator
	roomStorage      *mockports.MockRoomStorage
	instanceStorage  *instancestoragemock.MockGameRoomInstanceStorage
	runtime          *runtimemock.MockRuntime
	eventsService    interfaces.EventsService
	schedulerStorage *schedulerstoragemock.MockSchedulerStorage
}

func newMockRoomAndSchedulerManager(mockCtrl *gomock.Controller) *mockRoomAndSchedulerManager {
	clock := clockmock.NewFakeClock(time.Now())
	portAllocator := portallocatormock.NewMockPortAllocator(mockCtrl)
	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	instanceStorage := instancestoragemock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	eventsForwarderService := mockeventsservice.NewMockEventsService(mockCtrl)
	schedulerStorage := schedulerstoragemock.NewMockSchedulerStorage(mockCtrl)

	config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Second * 2}
	roomManager := room_manager.NewRoomManager(clock, portAllocator, roomStorage, instanceStorage, runtime, eventsForwarderService, config)
	operationFlow := mockports.NewMockOperationFlow(mockCtrl)
	operationStorage := mockports.NewMockOperationStorage(mockCtrl)
	operationLeaseStorage := mockports.NewMockOperationLeaseStorage(mockCtrl)
	opConfig := operation_manager.OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
	operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors(), operationLeaseStorage, opConfig)
	schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

	return &mockRoomAndSchedulerManager{
		roomManager,
		schedulerManager,
		operationFlow,
		operationStorage,
		portAllocator,
		roomStorage,
		instanceStorage,
		runtime,
		eventsForwarderService,
		schedulerStorage,
	}
}

func newValidScheduler(imageVersion string) *entities.Scheduler {
	return &entities.Scheduler{
		Name:            "scheduler",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "5",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1.0.0",
			TerminationGracePeriod: 60,
			Toleration:             "toleration",
			Affinity:               "affinity",
			Containers: []game_room.Container{
				{
					Name:            "default",
					Image:           fmt.Sprintf("some-image:%s", imageVersion),
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

func newInValidScheduler() *entities.Scheduler {
	scheduler := newValidScheduler("v1.0.0")
	scheduler.Spec.Version = "R1.0.0"
	return scheduler
}
