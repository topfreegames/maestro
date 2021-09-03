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

package remove_rooms

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	clock_mock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	instance_storage_mock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	port_allocator_mock "github.com/topfreegames/maestro/internal/adapters/port_allocator/mock"
	"github.com/topfreegames/maestro/internal/adapters/room_storage/mock"
	runtime_mock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
)

func TestExecute(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	clockMock := clock_mock.NewFakeClock(time.Now())
	portAllocatorMock := port_allocator_mock.NewMockPortAllocator(mockCtrl)
	roomStorageMock := mock.NewMockRoomStorage(mockCtrl)
	instanceStorageMock := instance_storage_mock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtimeMock := runtime_mock.NewMockRuntime(mockCtrl)

	roomsManager := room_manager.NewRoomManager(clockMock, portAllocatorMock, roomStorageMock, instanceStorageMock, runtimeMock)
	executor := NewExecutor(roomsManager)

	t.Run("when there are no rooms to be deleted it returns without error", func(t *testing.T) {
		definition := &RemoveRoomsDefinition{Amount: 2}
		operation := &operation.Operation{ID: "random-uuid", SchedulerName: "test-scheduler"}

		ctx := context.Background()
		roomStorageMock.EXPECT().GetAllRoomIDs(ctx, operation.SchedulerName).Return([]string{}, nil)

		err := executor.Execute(ctx, operation, definition)
		require.NoError(t, err)
	})

	t.Run("when rooms are successfully deleted it returns without error", func(t *testing.T) {
		schedulerName := "test-scheduler"
		definition := &RemoveRoomsDefinition{Amount: 1}
		operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

		availableRooms := []*game_room.GameRoom{
			{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
			{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
		}
		gameRoomInstance := &game_room.Instance{}

		ctx := context.Background()
		roomStorageMock.EXPECT().GetAllRoomIDs(ctx, operation.SchedulerName).Return([]string{availableRooms[0].ID, availableRooms[1].ID}, nil)
		roomStorageMock.EXPECT().GetRoom(ctx, schedulerName, availableRooms[0].ID).Return(availableRooms[0], nil)

		instanceStorageMock.EXPECT().GetInstance(ctx, schedulerName, availableRooms[0].ID).Return(gameRoomInstance, nil)
		runtimeMock.EXPECT().DeleteGameRoomInstance(ctx, gameRoomInstance).Return(nil)
		roomStorageMock.EXPECT().UpdateRoom(ctx, availableRooms[0]).Return(nil)

		err := executor.Execute(ctx, operation, definition)
		require.NoError(t, err)
	})

	t.Run("when any room failed to delete it returns without", func(t *testing.T) {
		schedulerName := "test-scheduler"
		definition := &RemoveRoomsDefinition{Amount: 2}
		operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

		availableRooms := []*game_room.GameRoom{
			{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
			{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
		}
		gameRoomInstance := &game_room.Instance{}

		ctx := context.Background()
		roomStorageMock.EXPECT().GetAllRoomIDs(ctx, operation.SchedulerName).Return([]string{availableRooms[0].ID, availableRooms[1].ID}, nil)
		roomStorageMock.EXPECT().GetRoom(ctx, schedulerName, availableRooms[0].ID).Return(availableRooms[0], nil)
		roomStorageMock.EXPECT().GetRoom(ctx, schedulerName, availableRooms[1].ID).Return(availableRooms[1], nil)

		// first one is successfull
		instanceStorageMock.EXPECT().GetInstance(ctx, schedulerName, availableRooms[0].ID).Return(gameRoomInstance, nil)
		runtimeMock.EXPECT().DeleteGameRoomInstance(ctx, gameRoomInstance).Return(nil)
		roomStorageMock.EXPECT().UpdateRoom(ctx, availableRooms[0]).Return(nil)

		// second one fails on runtime
		instanceStorageMock.EXPECT().GetInstance(ctx, schedulerName, availableRooms[1].ID).Return(gameRoomInstance, nil)
		runtimeMock.EXPECT().DeleteGameRoomInstance(ctx, gameRoomInstance).Return(porterrors.ErrUnexpected)

		err := executor.Execute(ctx, operation, definition)
		require.NoError(t, err)
	})
}
