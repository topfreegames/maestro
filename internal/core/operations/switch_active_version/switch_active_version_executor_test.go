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
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/operations/switch_active_version"

	"github.com/topfreegames/maestro/internal/core/services/interfaces"
	mockeventsservice "github.com/topfreegames/maestro/internal/core/services/interfaces/mock/events_service"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	clock_mock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	instance_storage_mock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	port_allocator_mock "github.com/topfreegames/maestro/internal/adapters/port_allocator/mock"
	room_storage_mock "github.com/topfreegames/maestro/internal/adapters/room_storage/mock"
	runtimemock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
	schedulerstoragemock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"
	"github.com/topfreegames/maestro/internal/validations"
)

// mockRoomAndSchedulerManager struct that holds all the mocks necessary for the
// operation executor.
type mockRoomAndSchedulerManager struct {
	roomManager      *room_manager.RoomManager
	schedulerManager *scheduler_manager.SchedulerManager
	portAllocator    *port_allocator_mock.MockPortAllocator
	roomStorage      *room_storage_mock.MockRoomStorage
	instanceStorage  *instance_storage_mock.MockGameRoomInstanceStorage
	runtime          *runtimemock.MockRuntime
	eventsService    interfaces.EventsService
	schedulerStorage *schedulerstoragemock.MockSchedulerStorage
}

// gameRoomIdMatcher matches the game room ID with the one provided.
type gameRoomIdMatcher struct {
	id string
}

// gameRoomVersionMatcher matches the game room version with the one provided.
type gameRoomVersionMatcher struct {
	version string
}

func TestSwitchActiveVersionOperation_Execute(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mocks := newMockRoomAndSchedulerManager(mockCtrl)

	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	currentVersion := "v1"

	newScheduler := newValidScheduler()
	newScheduler.PortRange.Start = 1000
	newScheduler.MaxSurge = "3"

	definition := &switch_active_version.SwitchActiveVersionDefinition{
		NewActiveScheduler: newScheduler,
	}

	t.Run("should succeed - Execute switch active version operation", func(t *testing.T) {
		mocks.schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

		// list the rooms in two "cycles"
		firstRoomsIds := []string{"room-0", "room-1", "room-2"}
		secondRoomsIds := []string{"room-3", "room-4"}

		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return(firstRoomsIds, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return([]string{}, nil)

		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return(secondRoomsIds, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return([]string{}, nil)

		// third time we list we want it to be empty
		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Times(4).Return([]string{}, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return([]string{}, nil)

		// for each room we want to mock: a new room creation and its
		// deletion.
		for _, roomId := range append(firstRoomsIds, secondRoomsIds...) {
			currentGameRoom := game_room.GameRoom{
				ID:          roomId,
				Version:     currentVersion,
				SchedulerID: definition.NewActiveScheduler.Name,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			}

			mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), definition.NewActiveScheduler.Name, roomId).Return(&currentGameRoom, nil)

			currentGameRoomInstance := game_room.Instance{
				ID:          roomId,
				SchedulerID: definition.NewActiveScheduler.Name,
			}

			newGameRoomInstance := game_room.Instance{
				ID:          fmt.Sprintf("new-%s", roomId),
				SchedulerID: definition.NewActiveScheduler.Name,
			}

			newGameRoom := game_room.GameRoom{
				ID:          newGameRoomInstance.ID,
				SchedulerID: definition.NewActiveScheduler.Name,
				Status:      game_room.GameStatusPending,
				Version:     newScheduler.Spec.Version,
			}

			mocks.portAllocator.EXPECT().Allocate(gomock.Any(), 1).Return([]int32{5000}, nil)
			mocks.runtime.EXPECT().CreateGameRoomInstance(context.Background(), definition.NewActiveScheduler.Name, versionEq(newScheduler.Spec.Version)).Return(&newGameRoomInstance, nil)

			gameRoomReady := newGameRoom
			gameRoomReady.Status = game_room.GameStatusReady
			gameRoomTerminating := currentGameRoom
			gameRoomTerminating.Status = game_room.GameStatusTerminating
			mocks.roomStorage.EXPECT().CreateRoom(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newScheduler.Spec.Version))).Return(nil)
			mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoomReady.SchedulerID, gameRoomReady.ID).Return(&gameRoomReady, nil)
			roomStorageStatusWatcher := room_storage_mock.NewMockRoomStorageStatusWatcher(mockCtrl)
			mocks.roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newScheduler.Spec.Version))).Return(roomStorageStatusWatcher, nil)

			mocks.instanceStorage.EXPECT().GetInstance(gomock.Any(), definition.NewActiveScheduler.Name, roomId).Return(&currentGameRoomInstance, nil)
			mocks.roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.All(idEq(currentGameRoom.ID), versionEq(currentVersion))).Return(roomStorageStatusWatcher, nil)
			mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoomTerminating.SchedulerID, gameRoomTerminating.ID).Return(&gameRoomTerminating, nil)
			mocks.runtime.EXPECT().DeleteGameRoomInstance(gomock.Any(), &currentGameRoomInstance).Return(nil)

			roomStorageStatusWatcher.EXPECT().Stop().Times(2)
		}

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		err = executor.Execute(context.Background(), &operation.Operation{}, definition)
		require.NoError(t, err)
	})

	t.Run("should succeed - Execute switch active version operation (no running rooms)", func(t *testing.T) {

		mocks.schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil)

		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Times(4).Return([]string{}, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return([]string{}, nil)

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		err := executor.Execute(context.Background(), &operation.Operation{}, definition)
		require.NoError(t, err)
	})

	t.Run("should fail - Can't update scheduler (switch active version on database)", func(t *testing.T) {
		mocks.schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(errors.New("error"))

		// list the rooms in two "cycles"
		firstRoomsIds := []string{"room-0", "room-1", "room-2"}
		secondRoomsIds := []string{"room-3", "room-4"}

		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return(firstRoomsIds, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return([]string{}, nil)

		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return(secondRoomsIds, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return([]string{}, nil)

		// third time we list we want it to be empty
		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Times(4).Return([]string{}, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return([]string{}, nil)

		// for each room we want to mock: a new room creation and its
		// deletion.
		for _, roomId := range append(firstRoomsIds, secondRoomsIds...) {
			currentGameRoom := game_room.GameRoom{
				ID:          roomId,
				Version:     currentVersion,
				SchedulerID: definition.NewActiveScheduler.Name,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			}

			mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), definition.NewActiveScheduler.Name, roomId).Return(&currentGameRoom, nil)

			currentGameRoomInstance := game_room.Instance{
				ID:          roomId,
				SchedulerID: definition.NewActiveScheduler.Name,
			}

			newGameRoomInstance := game_room.Instance{
				ID:          fmt.Sprintf("new-%s", roomId),
				SchedulerID: definition.NewActiveScheduler.Name,
			}

			newGameRoom := game_room.GameRoom{
				ID:          newGameRoomInstance.ID,
				SchedulerID: definition.NewActiveScheduler.Name,
				Status:      game_room.GameStatusPending,
				Version:     newScheduler.Spec.Version,
			}

			mocks.portAllocator.EXPECT().Allocate(gomock.Any(), 1).Return([]int32{5000}, nil)
			mocks.runtime.EXPECT().CreateGameRoomInstance(context.Background(), definition.NewActiveScheduler.Name, versionEq(newScheduler.Spec.Version)).Return(&newGameRoomInstance, nil)

			gameRoomReady := newGameRoom
			gameRoomReady.Status = game_room.GameStatusReady
			gameRoomTerminating := currentGameRoom
			gameRoomTerminating.Status = game_room.GameStatusTerminating
			mocks.roomStorage.EXPECT().CreateRoom(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newScheduler.Spec.Version))).Return(nil)
			mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoomReady.SchedulerID, gameRoomReady.ID).Return(&gameRoomReady, nil)
			roomStorageStatusWatcher := room_storage_mock.NewMockRoomStorageStatusWatcher(mockCtrl)
			mocks.roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newScheduler.Spec.Version))).Return(roomStorageStatusWatcher, nil)

			mocks.instanceStorage.EXPECT().GetInstance(gomock.Any(), definition.NewActiveScheduler.Name, roomId).Return(&currentGameRoomInstance, nil)
			mocks.roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.All(idEq(currentGameRoom.ID), versionEq(currentVersion))).Return(roomStorageStatusWatcher, nil)
			mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoomTerminating.SchedulerID, gameRoomTerminating.ID).Return(&gameRoomTerminating, nil)
			mocks.runtime.EXPECT().DeleteGameRoomInstance(gomock.Any(), &currentGameRoomInstance).Return(nil)

			roomStorageStatusWatcher.EXPECT().Stop().Times(2)
		}

		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		err = executor.Execute(context.Background(), &operation.Operation{}, definition)
		require.Error(t, err)
	})
}

func TestSwitchActiveVersionOperation_OnError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mocks := newMockRoomAndSchedulerManager(mockCtrl)

	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	newScheduler := newValidScheduler()
	definition := &switch_active_version.SwitchActiveVersionDefinition{
		NewActiveScheduler: newScheduler,
	}

	t.Run("should succeed - Execute on error if operation finishes (no created rooms)", func(t *testing.T) {
		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		err = executor.OnError(context.Background(), &operation.Operation{}, definition, nil)
		require.NoError(t, err)
	})

	t.Run("should succeed - Execute on error if operation finishes (created rooms)", func(t *testing.T) {
		currentVersion := "v1"
		newScheduler.PortRange.Start = 1000
		newScheduler.MaxSurge = "3"

		mocks.schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(errors.New("error"))

		// list the rooms in two "cycles"
		firstRoomsIds := []string{"room-0", "room-1", "room-2"}
		secondRoomsIds := []string{"room-3", "room-4"}

		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return(firstRoomsIds, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return([]string{}, nil)

		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return(secondRoomsIds, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Times(3).Return([]string{}, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return([]string{}, nil)

		// third time we list we want it to be empty
		mocks.roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Times(4).Return([]string{}, nil)
		mocks.roomStorage.EXPECT().GetRoomIDsByLastPing(gomock.Any(), definition.NewActiveScheduler.Name, gomock.Any()).Return([]string{}, nil)

		// for each room we want to mock: a new room creation and its
		// deletion.
		for _, roomId := range append(firstRoomsIds, secondRoomsIds...) {
			currentGameRoom := game_room.GameRoom{
				ID:          roomId,
				Version:     currentVersion,
				SchedulerID: definition.NewActiveScheduler.Name,
				Status:      game_room.GameStatusReady,
				LastPingAt:  time.Now(),
			}

			mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), definition.NewActiveScheduler.Name, roomId).Return(&currentGameRoom, nil)

			currentGameRoomInstance := game_room.Instance{
				ID:          roomId,
				SchedulerID: definition.NewActiveScheduler.Name,
			}

			newGameRoomInstance := game_room.Instance{
				ID:          fmt.Sprintf("new-%s", roomId),
				SchedulerID: definition.NewActiveScheduler.Name,
			}

			newGameRoom := game_room.GameRoom{
				ID:          newGameRoomInstance.ID,
				SchedulerID: definition.NewActiveScheduler.Name,
				Status:      game_room.GameStatusPending,
				Version:     newScheduler.Spec.Version,
			}

			mocks.portAllocator.EXPECT().Allocate(gomock.Any(), 1).Return([]int32{5000}, nil)
			mocks.runtime.EXPECT().CreateGameRoomInstance(context.Background(), definition.NewActiveScheduler.Name, versionEq(newScheduler.Spec.Version)).Return(&newGameRoomInstance, nil)

			gameRoomReady := newGameRoom
			gameRoomReady.Status = game_room.GameStatusReady
			gameRoomTerminating := currentGameRoom
			gameRoomTerminating.Status = game_room.GameStatusTerminating
			mocks.roomStorage.EXPECT().CreateRoom(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newScheduler.Spec.Version))).Return(nil)
			mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoomReady.SchedulerID, gameRoomReady.ID).Return(&gameRoomReady, nil)
			roomStorageStatusWatcher := room_storage_mock.NewMockRoomStorageStatusWatcher(mockCtrl)
			mocks.roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.All(idEq(newGameRoom.ID), versionEq(newScheduler.Spec.Version))).Return(roomStorageStatusWatcher, nil)

			mocks.instanceStorage.EXPECT().GetInstance(gomock.Any(), definition.NewActiveScheduler.Name, roomId).Return(&currentGameRoomInstance, nil)
			mocks.roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.All(idEq(currentGameRoom.ID), versionEq(currentVersion))).Return(roomStorageStatusWatcher, nil)
			mocks.roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoomTerminating.SchedulerID, gameRoomTerminating.ID).Return(&gameRoomTerminating, nil)
			mocks.runtime.EXPECT().DeleteGameRoomInstance(gomock.Any(), &currentGameRoomInstance).Return(nil)

			roomStorageStatusWatcher.EXPECT().Stop().Times(2)
		}

		ctx := context.Background()
		executor := switch_active_version.NewExecutor(mocks.roomManager, mocks.schedulerManager)
		err = executor.Execute(ctx, &operation.Operation{}, definition)
		require.Error(t, err)
		err = executor.OnError(ctx, &operation.Operation{}, definition, nil)
		require.NoError(t, err)
	})
}

func newMockRoomAndSchedulerManager(mockCtrl *gomock.Controller) *mockRoomAndSchedulerManager {
	clock := clock_mock.NewFakeClock(time.Now())
	portAllocator := port_allocator_mock.NewMockPortAllocator(mockCtrl)
	roomStorage := room_storage_mock.NewMockRoomStorage(mockCtrl)
	instanceStorage := instance_storage_mock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	eventsForwarderService := mockeventsservice.NewMockEventsService(mockCtrl)
	schedulerStorage := schedulerstoragemock.NewMockSchedulerStorage(mockCtrl)

	config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Second * 2}
	roomManager := room_manager.NewRoomManager(clock, portAllocator, roomStorage, instanceStorage, runtime, eventsForwarderService, config)
	schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

	return &mockRoomAndSchedulerManager{
		roomManager,
		schedulerManager,
		portAllocator,
		roomStorage,
		instanceStorage,
		runtime,
		eventsForwarderService,
		schedulerStorage,
	}
}

func versionEq(version string) gomock.Matcher {
	return &gameRoomVersionMatcher{version}
}

func idEq(id string) gomock.Matcher {
	return &gameRoomIdMatcher{id}
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

func newValidScheduler() entities.Scheduler {
	return entities.Scheduler{
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
