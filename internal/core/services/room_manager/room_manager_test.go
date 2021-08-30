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
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/ports/errors"

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
	roomManager := NewRoomManager(fakeClock, portAllocator, roomStorage, instanceStorage, runtime)
	scheduler := entities.Scheduler{
		Name: "game",
		Spec: game_room.Spec{
			Containers: []game_room.Container{
				{
					Name: "container1",
					Ports: []game_room.ContainerPort{
						{Protocol: "tcp"},
					},
				},
				{
					Name: "container2",
					Ports: []game_room.ContainerPort{
						{Protocol: "udp"},
					},
				},
			},
		},
		PortRange: nil,
	}

	portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
	runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.Name, game_room.Spec{
		Containers: []game_room.Container{
			{
				Name: "container1",
				Ports: []game_room.ContainerPort{
					{
						Protocol: "tcp",
						HostPort: 5000,
					},
				},
			},
			{
				Name: "container2",
				Ports: []game_room.ContainerPort{
					{
						Protocol: "udp",
						HostPort: 6000,
					},
				},
			},
		},
	}).Return(&game_room.Instance{
		ID:          "game-1",
		SchedulerID: "game",
		Version:     "1",
	}, nil)
	roomStorage.EXPECT().CreateRoom(context.Background(), &game_room.GameRoom{
		ID:          "game-1",
		SchedulerID: "game",
		Status:      game_room.GameStatusPending,
		LastPingAt:  now,
	})

	room, instance, err := roomManager.CreateRoom(context.Background(), scheduler)
	require.NoError(t, err)
	require.Equal(t, &game_room.GameRoom{
		ID:          "game-1",
		SchedulerID: "game",
		Status:      game_room.GameStatusPending,
		LastPingAt:  now,
	}, room)
	require.Equal(t, &game_room.Instance{
		ID:          "game-1",
		SchedulerID: "game",
		Version:     "1",
	}, instance)
}

func TestRoomManager_DeleteRoom(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	roomManager := NewRoomManager(
		clockmock.NewFakeClock(time.Now()),
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
	)

	nextStatus := game_room.GameStatusTerminating
	gameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusReady}
	newGameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: nextStatus}
	instance := &game_room.Instance{ID: "test-instance"}
	instanceStorage.EXPECT().GetInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(instance, nil)
	roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoom).Return(nil)
	runtime.EXPECT().DeleteGameRoomInstance(context.Background(), instance).Return(nil)

	err := roomManager.DeleteRoom(context.Background(), gameRoom)
	require.NoError(t, err)
}

func TestRoomManager_UpdateRoom(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	roomManager := NewRoomManager(
		clockmock.NewFakeClock(time.Now()),
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
	)
	currentGameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusReady}
	newGameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusOccupied}

	t.Run("when the current game room exists then it execute without returning error", func(t *testing.T) {
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(currentGameRoom, nil)
		roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoom).Return(nil)

		err := roomManager.UpdateRoom(context.Background(), newGameRoom)

		require.NoError(t, err)
	})

	t.Run("when the current game room does not exist then it returns proper error", func(t *testing.T) {
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(nil, errors.ErrUnexpected)

		err := roomManager.UpdateRoom(context.Background(), newGameRoom)

		require.Error(t, err)
	})

	t.Run("when there is some error while updating the room then it returns proper error", func(t *testing.T) {
		roomStorage.EXPECT().GetRoom(context.Background(), newGameRoom.SchedulerID, newGameRoom.ID).Return(currentGameRoom, nil)
		roomStorage.EXPECT().UpdateRoom(context.Background(), newGameRoom).Return(errors.ErrUnexpected)

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
