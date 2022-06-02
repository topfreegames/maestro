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
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/topfreegames/maestro/internal/core/entities/events"

	"golang.org/x/sync/errgroup"

	"github.com/topfreegames/maestro/internal/core/ports"

	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/golang/mock/gomock"

	clockmock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	ismock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	pamock "github.com/topfreegames/maestro/internal/adapters/port_allocator/mock"
	runtimemock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
	serviceerrors "github.com/topfreegames/maestro/internal/core/services/errors"

	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
)

func TestRoomManager_CreateRoomAndWaitForReadiness(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	now := time.Now()
	portAllocator := pamock.NewMockPortAllocator(mockCtrl)
	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	eventsService := mockports.NewMockEventsService(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	fakeClock := clockmock.NewFakeClock(now)
	config := RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000, RoomDeletionTimeout: time.Millisecond * 1000}
	roomManager := New(fakeClock, portAllocator, roomStorage, instanceStorage, runtime, eventsService, config)
	roomStorageStatusWatcher := mockports.NewMockRoomStorageStatusWatcher(mockCtrl)

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

	containerWithHostPort1 := game_room.Container{
		Name: "container1",
		Ports: []game_room.ContainerPort{
			{Protocol: "tcp", HostPort: 5000},
		},
	}

	containerWithHostPort2 := game_room.Container{
		Name: "container2",
		Ports: []game_room.ContainerPort{
			{Protocol: "udp", HostPort: 6000},
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
			Containers: []game_room.Container{containerWithHostPort1, containerWithHostPort2},
		}).Return(&gameRoomInstance, nil)

		gameRoomReady := gameRoom
		gameRoomReady.Status = game_room.GameStatusReady

		roomStorage.EXPECT().CreateRoom(context.Background(), &gameRoom)
		roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoom.SchedulerID, gameRoom.ID).Return(&gameRoomReady, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), &gameRoom).Return(roomStorageStatusWatcher, nil)
		instanceStorage.EXPECT().UpsertInstance(gomock.Any(), &gameRoomInstance).Return(nil)

		roomStorageStatusWatcher.EXPECT().Stop()

		room, instance, err := roomManager.CreateRoomAndWaitForReadiness(context.Background(), scheduler, false)
		require.NoError(t, err)
		require.Equal(t, &gameRoom, room)
		require.Equal(t, &gameRoomInstance, instance)
	})

	t.Run("when game room creation fails with initialization timeout then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
		runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.Name, game_room.Spec{
			Containers: []game_room.Container{containerWithHostPort1, containerWithHostPort2},
		}).Return(&gameRoomInstance, nil)

		roomStorage.EXPECT().CreateRoom(context.Background(), &gameRoom)
		roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoom.SchedulerID, gameRoom.ID).Return(&gameRoom, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), &gameRoom).Return(roomStorageStatusWatcher, nil)
		instanceStorage.EXPECT().UpsertInstance(gomock.Any(), &gameRoomInstance).Return(nil)

		roomStorageStatusWatcher.EXPECT().Stop()
		roomStorageStatusWatcher.EXPECT().ResultChan()

		instance := &game_room.Instance{ID: "test-instance"}
		gameRoomTerminating := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusTerminating}

		instanceStorage.EXPECT().GetInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(instance, nil)
		runtime.EXPECT().DeleteGameRoomInstance(gomock.Any(), instance).Return(nil)
		roomStorage.EXPECT().GetRoom(gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomTerminating, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.Any()).Return(roomStorageStatusWatcher, nil)
		roomStorageStatusWatcher.EXPECT().Stop()
		eventsService.EXPECT().ProduceEvent(gomock.Any(), gomock.Any())

		room, instance, err := roomManager.CreateRoomAndWaitForReadiness(context.Background(), scheduler, false)
		require.Error(t, err)
		require.True(t, errors.Is(err, serviceerrors.ErrGameRoomStatusWaitingTimeout))
		require.Nil(t, room)
		require.Nil(t, instance)
	})

	t.Run("when context is cancelled while waiting game room to be ready, it deletes the created game room", func(t *testing.T) {
		mainContext, mainContextCancelFunc := context.WithCancel(context.Background())
		portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
		runtime.EXPECT().CreateGameRoomInstance(gomock.Any(), scheduler.Name, game_room.Spec{
			Containers: []game_room.Container{containerWithHostPort1, containerWithHostPort2},
		}).Return(&gameRoomInstance, nil)

		roomStorage.EXPECT().CreateRoom(mainContext, &gameRoom)
		roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoom.SchedulerID, gameRoom.ID).Return(&gameRoom, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), &gameRoom).DoAndReturn(func(ctx context.Context, room *game_room.GameRoom) (ports.RoomStorageStatusWatcher, error) {
			mainContextCancelFunc()
			return roomStorageStatusWatcher, nil
		})
		instanceStorage.EXPECT().UpsertInstance(gomock.Any(), &gameRoomInstance).Return(nil)
		roomStorageStatusWatcher.EXPECT().Stop()
		roomStorageStatusWatcher.EXPECT().ResultChan()

		instance := &game_room.Instance{ID: "test-instance"}
		gameRoomTerminating := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusTerminating}

		instanceStorage.EXPECT().GetInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(instance, nil)
		runtime.EXPECT().DeleteGameRoomInstance(gomock.Any(), instance).DoAndReturn(func(deleteContext context.Context, gameRoomInstance *game_room.Instance) error {
			require.NoError(t, deleteContext.Err())
			return nil
		})
		roomStorage.EXPECT().GetRoom(gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoomTerminating, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gomock.Any()).Return(roomStorageStatusWatcher, nil)
		roomStorageStatusWatcher.EXPECT().Stop()
		eventsService.EXPECT().ProduceEvent(gomock.Any(), gomock.Any())

		room, instance, err := roomManager.CreateRoomAndWaitForReadiness(mainContext, scheduler, false)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), context.Canceled.Error()))
		require.Nil(t, room)
		require.Nil(t, instance)
	})

	t.Run("when game room creation fails while creating instance then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
		runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.Name, game_room.Spec{
			Containers: []game_room.Container{containerWithHostPort1, containerWithHostPort2},
		}).Return(nil, porterrors.NewErrUnexpected("error create game room instance"))

		room, instance, err := roomManager.CreateRoomAndWaitForReadiness(context.Background(), scheduler, false)
		require.Error(t, err)
		require.Nil(t, room)
		require.Nil(t, instance)
	})

	t.Run("when game room creation fails while creating game room on storage then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
		runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.Name, game_room.Spec{
			Containers: []game_room.Container{containerWithHostPort1, containerWithHostPort2},
		}).Return(&gameRoomInstance, nil)
		instanceStorage.EXPECT().UpsertInstance(gomock.Any(), &gameRoomInstance).Return(nil)

		roomStorage.EXPECT().CreateRoom(context.Background(), &gameRoom).Return(porterrors.NewErrUnexpected("error storing room on redis"))

		room, instance, err := roomManager.CreateRoomAndWaitForReadiness(context.Background(), scheduler, false)
		require.Error(t, err)
		require.Nil(t, room)
		require.Nil(t, instance)
	})

	t.Run("when game room creation fails while allocating ports then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return(nil, porterrors.NewErrInvalidArgument("not enough ports to allocate"))

		room, instance, err := roomManager.CreateRoomAndWaitForReadiness(context.Background(), scheduler, false)
		require.Error(t, err)
		require.Nil(t, room)
		require.Nil(t, instance)
	})

	t.Run("when upsert instance fails then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
		runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.Name, game_room.Spec{
			Containers: []game_room.Container{containerWithHostPort1, containerWithHostPort2},
		}).Return(&gameRoomInstance, nil)
		instanceStorage.EXPECT().UpsertInstance(gomock.Any(), &gameRoomInstance).Return(errors.New("error"))

		room, instance, err := roomManager.CreateRoomAndWaitForReadiness(context.Background(), scheduler, false)
		require.Error(t, err)
		require.Nil(t, room)
		require.Nil(t, instance)
	})

}

func TestRoomManager_CreateRoom(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	now := time.Now()
	portAllocator := pamock.NewMockPortAllocator(mockCtrl)
	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	eventsService := mockports.NewMockEventsService(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	fakeClock := clockmock.NewFakeClock(now)
	config := RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000, RoomDeletionTimeout: time.Millisecond * 1000}
	roomManager := New(fakeClock, portAllocator, roomStorage, instanceStorage, runtime, eventsService, config)

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

	containerWithHostPort1 := game_room.Container{
		Name: "container1",
		Ports: []game_room.ContainerPort{
			{Protocol: "tcp", HostPort: 5000},
		},
	}

	containerWithHostPort2 := game_room.Container{
		Name: "container2",
		Ports: []game_room.ContainerPort{
			{Protocol: "udp", HostPort: 6000},
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
			Containers: []game_room.Container{containerWithHostPort1, containerWithHostPort2},
		}).Return(&gameRoomInstance, nil)

		gameRoomReady := gameRoom
		gameRoomReady.Status = game_room.GameStatusReady

		roomStorage.EXPECT().CreateRoom(context.Background(), &gameRoom)
		instanceStorage.EXPECT().UpsertInstance(gomock.Any(), &gameRoomInstance).Return(nil)

		room, instance, err := roomManager.CreateRoom(context.Background(), scheduler, false)
		assert.NoError(t, err)
		assert.Equal(t, &gameRoom, room)
		assert.Equal(t, &gameRoomInstance, instance)
	})

	t.Run("when game room creation fails while creating instance on runtime then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
		runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.Name, game_room.Spec{
			Containers: []game_room.Container{containerWithHostPort1, containerWithHostPort2},
		}).Return(nil, porterrors.NewErrUnexpected("error creating game room on runtime"))

		room, instance, err := roomManager.CreateRoomAndWaitForReadiness(context.Background(), scheduler, false)
		assert.EqualError(t, err, "error creating game room on runtime")
		assert.Nil(t, room)
		assert.Nil(t, instance)
	})

	t.Run("when game room creation fails while creating game room on storage then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
		runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.Name, game_room.Spec{
			Containers: []game_room.Container{containerWithHostPort1, containerWithHostPort2},
		}).Return(&gameRoomInstance, nil)
		instanceStorage.EXPECT().UpsertInstance(gomock.Any(), &gameRoomInstance).Return(nil)

		roomStorage.EXPECT().CreateRoom(context.Background(), &gameRoom).Return(porterrors.NewErrUnexpected("error storing room on redis"))

		room, instance, err := roomManager.CreateRoomAndWaitForReadiness(context.Background(), scheduler, false)
		assert.EqualError(t, err, "error storing room on redis")
		assert.Nil(t, room)
		assert.Nil(t, instance)
	})

	t.Run("when game room creation fails while allocating ports then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return(nil, porterrors.NewErrInvalidArgument("not enough ports to allocate"))

		room, instance, err := roomManager.CreateRoomAndWaitForReadiness(context.Background(), scheduler, false)
		assert.EqualError(t, err, "not enough ports to allocate")
		assert.Nil(t, room)
		assert.Nil(t, instance)
	})

	t.Run("when upsert instance fails then it returns nil with proper error", func(t *testing.T) {
		portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
		runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.Name, game_room.Spec{
			Containers: []game_room.Container{containerWithHostPort1, containerWithHostPort2},
		}).Return(&gameRoomInstance, nil)
		instanceStorage.EXPECT().UpsertInstance(gomock.Any(), &gameRoomInstance).Return(errors.New("error creating instance"))

		room, instance, err := roomManager.CreateRoomAndWaitForReadiness(context.Background(), scheduler, false)
		assert.EqualError(t, err, "error creating instance")
		assert.Nil(t, room)
		assert.Nil(t, instance)
	})

}

func TestRoomManager_DeleteRoomAndWaitForRoomTerminating(t *testing.T) {
	t.Run("when game room status update is successful, it deletes the game room from runtime and waits for the status to be updated correctly", func(t *testing.T) {
		roomManager, _, roomStorage, instanceStorage, runtime, eventsService, roomStorageStatusWatcher := testSetup(t)
		gameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusReady}
		gameRoomTerminating := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusTerminating}
		expectedEvent := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: gameRoom.SchedulerID,
			RoomID:      gameRoom.ID,
			Attributes: map[string]interface{}{
				"eventType": "status",
				"roomEvent": "terminated",
				"pingType":  "terminated",
			},
		}

		instance := &game_room.Instance{ID: "test-instance"}
		instanceStorage.EXPECT().GetInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(instance, nil)
		runtime.EXPECT().DeleteGameRoomInstance(context.Background(), instance).Return(nil)
		roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoom.SchedulerID, gameRoom.ID).Return(gameRoomTerminating, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gameRoom).Return(roomStorageStatusWatcher, nil)
		roomStorageStatusWatcher.EXPECT().Stop()
		eventsService.EXPECT().ProduceEvent(gomock.Any(), expectedEvent).Return(nil)

		err := roomManager.DeleteRoomAndWaitForRoomTerminating(context.Background(), gameRoom)
		require.NoError(t, err)
	})

	t.Run("when game room deletion fails with deletion timeout upon waiting game room status update, it returns an error", func(t *testing.T) {
		roomManager, _, roomStorage, instanceStorage, runtime, _, roomStorageStatusWatcher := testSetup(t)

		gameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusReady}

		instance := &game_room.Instance{ID: "test-instance"}
		instanceStorage.EXPECT().GetInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(instance, nil)
		runtime.EXPECT().DeleteGameRoomInstance(context.Background(), instance).Return(nil)
		roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoom.SchedulerID, gameRoom.ID).Return(gameRoom, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gameRoom).Return(roomStorageStatusWatcher, nil)
		roomStorageStatusWatcher.EXPECT().ResultChan()
		roomStorageStatusWatcher.EXPECT().Stop()

		err := roomManager.DeleteRoomAndWaitForRoomTerminating(context.Background(), gameRoom)
		require.True(t, errors.Is(err, serviceerrors.ErrGameRoomStatusWaitingTimeout))
		require.EqualError(t, err, "failed to wait until room has desired status: terminating, reason: context deadline exceeded")
	})

	t.Run("when the deletion is successful and some errors occurs producing events it returns no error", func(t *testing.T) {
		roomManager, _, roomStorage, instanceStorage, runtime, eventsService, roomStorageStatusWatcher := testSetup(t)
		gameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusReady}
		gameRoomTerminating := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusTerminating}
		expectedEvent := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: gameRoom.SchedulerID,
			RoomID:      gameRoom.ID,
			Attributes: map[string]interface{}{
				"eventType": "status",
				"roomEvent": "terminated",
				"pingType":  "terminated",
			},
		}

		instance := &game_room.Instance{ID: "test-instance"}
		instanceStorage.EXPECT().GetInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(instance, nil)
		runtime.EXPECT().DeleteGameRoomInstance(context.Background(), instance).Return(nil)
		roomStorage.EXPECT().GetRoom(gomock.Any(), gameRoom.SchedulerID, gameRoom.ID).Return(gameRoomTerminating, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gameRoom).Return(roomStorageStatusWatcher, nil)
		roomStorageStatusWatcher.EXPECT().Stop()
		eventsService.EXPECT().ProduceEvent(gomock.Any(), expectedEvent).Return(errors.New("some error"))

		err := roomManager.DeleteRoomAndWaitForRoomTerminating(context.Background(), gameRoom)
		require.NoError(t, err)
	})

	t.Run("when room instance is not found on storage, try to delete game room, do not return error", func(t *testing.T) {
		roomManager, _, roomStorage, instanceStorage, _, _, _ := testSetup(t)

		gameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusTerminating}
		instanceStorage.EXPECT().GetInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(nil, porterrors.NewErrNotFound("error"))
		roomStorage.EXPECT().DeleteRoom(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(nil)

		err := roomManager.DeleteRoomAndWaitForRoomTerminating(context.Background(), gameRoom)
		require.NoError(t, err)
	})

	t.Run("when room instance is not found on runtime do not return error", func(t *testing.T) {
		roomManager, _, roomStorage, instanceStorage, runtime, _, _ := testSetup(t)

		gameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusTerminating}
		instance := &game_room.Instance{ID: "test-instance"}
		instanceStorage.EXPECT().GetInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(instance, nil)
		runtime.EXPECT().DeleteGameRoomInstance(context.Background(), instance).Return(porterrors.NewErrNotFound("error"))
		roomStorage.EXPECT().DeleteRoom(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(nil)
		instanceStorage.EXPECT().DeleteInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(nil)

		err := roomManager.DeleteRoomAndWaitForRoomTerminating(context.Background(), gameRoom)
		require.NoError(t, err)
	})

	t.Run("when some error occurs by fetching the instance it returns error", func(t *testing.T) {
		roomManager, _, _, instanceStorage, _, _, _ := testSetup(t)
		gameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusReady}

		instanceStorage.EXPECT().GetInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(nil, errors.New("some error"))

		err := roomManager.DeleteRoomAndWaitForRoomTerminating(context.Background(), gameRoom)
		require.Error(t, err)
	})

	t.Run("when room deletion has error returns error", func(t *testing.T) {
		roomManager, _, _, instanceStorage, runtime, _, _ := testSetup(t)

		gameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusTerminating}
		instance := &game_room.Instance{ID: "test-instance"}
		instanceStorage.EXPECT().GetInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(instance, nil)
		runtime.EXPECT().DeleteGameRoomInstance(context.Background(), instance).Return(porterrors.ErrUnexpected)

		err := roomManager.DeleteRoomAndWaitForRoomTerminating(context.Background(), gameRoom)
		require.Error(t, err)
	})
}

func TestRoomManager_UpdateRoom(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	eventsService := mockports.NewMockEventsService(mockCtrl)
	clock := clockmock.NewFakeClock(time.Now())
	config := RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000, RoomDeletionTimeout: time.Millisecond * 1000}

	roomManager := New(
		clock,
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
		eventsService,
		config,
	)
	currentInstance := &game_room.Instance{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.InstanceStatus{Type: game_room.InstanceReady}}
	newGameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusReady, PingStatus: game_room.GameRoomPingStatusOccupied, LastPingAt: clock.Now(), Metadata: map[string]interface{}{}}

	t.Run("when the current game room exists then it execute without returning error", func(t *testing.T) {
		roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoom).Return(nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(currentInstance, nil)
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(newGameRoom, nil)
		roomStorage.EXPECT().UpdateRoomStatus(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID, game_room.GameStatusOccupied).Return(nil)
		eventsService.EXPECT().ProduceEvent(context.Background(), gomock.Any())

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

	t.Run("when some error occurs on events forwarding then it does not return with error", func(t *testing.T) {
		roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoom).Return(nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(currentInstance, nil)
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(newGameRoom, nil)
		roomStorage.EXPECT().UpdateRoomStatus(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID, game_room.GameStatusOccupied).Return(nil)
		eventsService.EXPECT().ProduceEvent(context.Background(), gomock.Any())

		err := roomManager.UpdateRoom(context.Background(), newGameRoom)
		require.NoError(t, err)
	})
}

func TestRoomManager_ListRoomsWithDeletionPriority(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	eventsService := mockports.NewMockEventsService(mockCtrl)
	clock := clockmock.NewFakeClock(time.Now())
	roomManager := New(
		clock,
		nil,
		roomStorage,
		nil,
		runtime,
		eventsService,
		RoomManagerConfig{RoomPingTimeout: time.Hour},
	)
	roomsBeingReplaced := &sync.Map{}

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

		rooms, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "v1.2.2", 5, roomsBeingReplaced)
		require.NoError(t, err)
		require.Len(t, rooms, 5)
	})

	t.Run("when error happens while fetching on-error room ids it returns error", func(t *testing.T) {
		ctx := context.Background()
		schedulerName := "test-scheduler"
		getRoomIDsErr := errors.New("failed to get rooms IDs")

		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusError).Return(nil, getRoomIDsErr)

		_, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", 2, roomsBeingReplaced)
		require.Error(t, err)
		require.ErrorIs(t, err, getRoomIDsErr)
	})

	t.Run("when error happens while fetching old ping room ids it returns error", func(t *testing.T) {
		ctx := context.Background()
		schedulerName := "test-scheduler"
		getRoomIDsErr := errors.New("failed to get rooms IDs")

		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusError).Return([]string{}, nil)
		roomStorage.EXPECT().GetRoomIDsByLastPing(ctx, schedulerName, gomock.Any()).Return(nil, getRoomIDsErr)

		_, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", 2, roomsBeingReplaced)
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

		_, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", 2, roomsBeingReplaced)
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

		_, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", 2, roomsBeingReplaced)
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

		_, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", 2, roomsBeingReplaced)
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

		_, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", 2, roomsBeingReplaced)
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

		rooms, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, ignoredVersion, 2, roomsBeingReplaced)
		require.NoError(t, err)
		require.Empty(t, rooms)
	})

	t.Run("when retrieving rooms with terminating status it returns an empty list", func(t *testing.T) {
		ctx := context.Background()
		schedulerName := "test-scheduler"
		ignoredVersion := "v1.2.3"
		availableRooms := []*game_room.GameRoom{
			{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusTerminating, Version: "v1.1.1"},
			{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusTerminating, Version: "v1.1.1"},
		}

		roomStorage.EXPECT().GetRoomIDsByStatus(ctx, schedulerName, gomock.Any()).Return([]string{}, nil).AnyTimes()
		roomStorage.EXPECT().GetRoomIDsByLastPing(ctx, schedulerName, gomock.Any()).Return([]string{availableRooms[0].ID, availableRooms[1].ID}, nil)

		roomStorage.EXPECT().GetRoom(ctx, schedulerName, availableRooms[0].ID).Return(availableRooms[0], nil)
		roomStorage.EXPECT().GetRoom(ctx, schedulerName, availableRooms[1].ID).Return(availableRooms[1], nil)

		rooms, err := roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, ignoredVersion, 2, roomsBeingReplaced)
		require.NoError(t, err)
		require.Empty(t, rooms)
	})

}

func TestRoomManager_UpdateRoomInstance(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	eventsService := mockports.NewMockEventsService(mockCtrl)
	clock := clockmock.NewFakeClock(time.Now())
	config := RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000, RoomDeletionTimeout: time.Millisecond * 1000}
	roomManager := New(
		clock,
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
		eventsService,
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

	t.Run("should fail - room instance is nil => returns error", func(t *testing.T) {
		err := roomManager.UpdateRoomInstance(context.Background(), nil)
		require.Error(t, err)
	})
}

func TestRoomManager_CleanRoomState(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	eventsService := mockports.NewMockEventsService(mockCtrl)
	clock := clockmock.NewFakeClock(time.Now())
	config := RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000, RoomDeletionTimeout: time.Millisecond * 1000}
	roomManager := New(
		clock,
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
		eventsService,
		config,
	)
	schedulerName := "scheduler-name"
	roomId := "some-unique-room-id"

	t.Run("when room and instance deletions do not return error", func(t *testing.T) {
		roomStorage.EXPECT().DeleteRoom(context.Background(), schedulerName, roomId).Return(nil)
		instanceStorage.EXPECT().DeleteInstance(context.Background(), schedulerName, roomId).Return(nil)
		eventsService.EXPECT().ProduceEvent(context.Background(), gomock.Any()).Return(nil)

		err := roomManager.CleanRoomState(context.Background(), schedulerName, roomId)
		require.NoError(t, err)
	})

	t.Run("when room is not found but instance is, returns no error", func(t *testing.T) {
		roomStorage.EXPECT().DeleteRoom(context.Background(), schedulerName, roomId).Return(porterrors.ErrNotFound)
		instanceStorage.EXPECT().DeleteInstance(context.Background(), schedulerName, roomId).Return(nil)
		eventsService.EXPECT().ProduceEvent(context.Background(), gomock.Any()).Return(nil)

		err := roomManager.CleanRoomState(context.Background(), schedulerName, roomId)
		require.NoError(t, err)
	})

	t.Run("when room is present but instance isn't, returns no error", func(t *testing.T) {
		roomStorage.EXPECT().DeleteRoom(context.Background(), schedulerName, roomId).Return(nil)
		instanceStorage.EXPECT().DeleteInstance(context.Background(), schedulerName, roomId).Return(porterrors.ErrNotFound)
		eventsService.EXPECT().ProduceEvent(context.Background(), gomock.Any()).Return(nil)

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
	setupRoomStorage := func(mockCtrl *gomock.Controller) (*mockports.MockRoomStorage, ports.RoomManager) {
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		roomManager := New(
			clockmock.NewFakeClock(time.Now()),
			pamock.NewMockPortAllocator(mockCtrl),
			roomStorage,
			ismock.NewMockGameRoomInstanceStorage(mockCtrl),
			runtimemock.NewMockRuntime(mockCtrl),
			mockports.NewMockEventsService(mockCtrl),
			RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000},
		)

		return roomStorage, roomManager
	}

	t.Run("max surge is empty, returns minimum value without error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		_, roomManager := setupRoomStorage(mockCtrl)
		scheduler := &entities.Scheduler{Name: "test", MaxSurge: ""}

		maxSurgeValue, err := roomManager.SchedulerMaxSurge(context.Background(), scheduler)
		require.NoError(t, err)
		require.Equal(t, 1, maxSurgeValue)
	})

	t.Run("max surge uses absolute number, returns value without error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		_, roomManager := setupRoomStorage(mockCtrl)
		scheduler := &entities.Scheduler{Name: "test", MaxSurge: "100"}

		maxSurgeValue, err := roomManager.SchedulerMaxSurge(context.Background(), scheduler)
		require.NoError(t, err)
		require.Equal(t, 100, maxSurgeValue)
	})

	t.Run("max surge uses absolute number less than the minimum, returns minimum without error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		_, roomManager := setupRoomStorage(mockCtrl)
		scheduler := &entities.Scheduler{Name: "test", MaxSurge: "0"}

		maxSurgeValue, err := roomManager.SchedulerMaxSurge(context.Background(), scheduler)
		require.NoError(t, err)
		require.Equal(t, 1, maxSurgeValue)
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
		invalidMaxSurges := []string{"%", "a%", "a", "1a", "%123"}

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
		mockports.NewMockEventsService(mockCtrl),
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
		mockports.NewMockEventsService(mockCtrl),
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
	setup := func(mockCtrl *gomock.Controller) (*mockports.MockRoomStorage, *ismock.MockGameRoomInstanceStorage, ports.RoomManager, *mockports.MockEventsService) {
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
		eventsService := mockports.NewMockEventsService(mockCtrl)
		roomManager := New(
			clockmock.NewFakeClock(time.Now()),
			pamock.NewMockPortAllocator(mockCtrl),
			roomStorage,
			instanceStorage,
			runtimemock.NewMockRuntime(mockCtrl),
			eventsService,
			RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000},
		)

		return roomStorage, instanceStorage, roomManager, eventsService
	}

	t.Run("when game room exists and changes states, it should return no error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerName := "schedulerName"
		roomId := "room-id"
		roomStorage, instanceStorage, roomManager, _ := setup(mockCtrl)

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
		roomStorage, instanceStorage, roomManager, _ := setup(mockCtrl)

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
		roomStorage, _, roomManager, _ := setup(mockCtrl)

		roomStorage.EXPECT().GetRoom(context.Background(), schedulerName, roomId).Return(nil, porterrors.ErrNotFound)

		err := roomManager.UpdateGameRoomStatus(context.Background(), schedulerName, roomId)
		require.Error(t, err)
	})

	t.Run("when game room instance doesn't exists, it should return error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerName := "schedulerName"
		roomId := "room-id"
		roomStorage, instanceStorage, roomManager, _ := setup(mockCtrl)

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
		roomStorage, instanceStorage, roomManager, _ := setup(mockCtrl)

		room := &game_room.GameRoom{PingStatus: game_room.GameRoomPingStatusReady, Status: game_room.GameStatusTerminating}
		roomStorage.EXPECT().GetRoom(context.Background(), schedulerName, roomId).Return(room, nil)

		instance := &game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceReady}}
		instanceStorage.EXPECT().GetInstance(context.Background(), schedulerName, roomId).Return(instance, nil)

		err := roomManager.UpdateGameRoomStatus(context.Background(), schedulerName, roomId)
		require.Error(t, err)
	})

	t.Run("When instance status is terminating, forward ping", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerName := "schedulerName"
		roomId := "room-id"
		roomStorage, instanceStorage, roomManager, eventsService := setup(mockCtrl)

		room := &game_room.GameRoom{PingStatus: game_room.GameRoomPingStatusTerminating, Status: game_room.GameStatusReady}
		roomStorage.EXPECT().GetRoom(context.Background(), schedulerName, roomId).Return(room, nil)

		instance := &game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceTerminating}}
		instanceStorage.EXPECT().GetInstance(context.Background(), schedulerName, roomId).Return(instance, nil)

		roomStorage.EXPECT().UpdateRoomStatus(context.Background(), schedulerName, roomId, game_room.GameStatusTerminating).Return(nil)
		eventsService.EXPECT().ProduceEvent(context.Background(), gomock.Any()).Return(nil)

		err := roomManager.UpdateGameRoomStatus(context.Background(), schedulerName, roomId)
		require.NoError(t, err)
	})
}

func TestRoomManager_GetRoomInstance(t *testing.T) {
	setup := func(mockCtrl *gomock.Controller) (*mockports.MockRoomStorage, *ismock.MockGameRoomInstanceStorage, ports.RoomManager, *mockports.MockEventsService) {
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
		eventsService := mockports.NewMockEventsService(mockCtrl)
		roomManager := New(
			clockmock.NewFakeClock(time.Now()),
			pamock.NewMockPortAllocator(mockCtrl),
			roomStorage,
			instanceStorage,
			runtimemock.NewMockRuntime(mockCtrl),
			eventsService,
			RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000},
		)

		return roomStorage, instanceStorage, roomManager, eventsService
	}

	t.Run("when no error occurs return game room instance and no error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerName := "schedulerName"
		roomId := "room-id"
		_, instanceStorage, roomManager, _ := setup(mockCtrl)

		instance := &game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceReady}}
		instanceStorage.EXPECT().GetInstance(context.Background(), schedulerName, roomId).Return(instance, nil)

		address, err := roomManager.GetRoomInstance(context.Background(), schedulerName, roomId)
		require.NoError(t, err)
		require.Equal(t, instance, address)
	})

	t.Run("when some error occurs in instance storage it returns error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerName := "schedulerName"
		roomId := "room-id"
		_, instanceStorage, roomManager, _ := setup(mockCtrl)

		instanceStorage.EXPECT().GetInstance(context.Background(), schedulerName, roomId).Return(nil, errors.New("some error"))

		_, err := roomManager.GetRoomInstance(context.Background(), schedulerName, roomId)
		require.EqualError(t, err, "some error")
	})

}

func testSetup(t *testing.T) (
	ports.RoomManager,
	RoomManagerConfig,
	*mockports.MockRoomStorage,
	*ismock.MockGameRoomInstanceStorage,
	*runtimemock.MockRuntime,
	*mockports.MockEventsService,
	*mockports.MockRoomStorageStatusWatcher,
) {
	mockCtrl := gomock.NewController(t)

	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	eventsService := mockports.NewMockEventsService(mockCtrl)
	config := RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000, RoomDeletionTimeout: time.Millisecond * 1000}
	roomStorageStatusWatcher := mockports.NewMockRoomStorageStatusWatcher(mockCtrl)

	roomManager := New(
		clockmock.NewFakeClock(time.Now()),
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
		eventsService,
		config,
	)

	return roomManager, config, roomStorage, instanceStorage, runtime, eventsService, roomStorageStatusWatcher
}
