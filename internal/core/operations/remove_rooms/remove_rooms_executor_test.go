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

package remove_rooms

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

func TestExecute(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("should succeed - no rooms to be deleted => returns without error", func(t *testing.T) {
		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		executor := NewExecutor(roomsManager)

		schedulerName := uuid.NewString()
		definition := &RemoveRoomsDefinition{Amount: 2}
		operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

		ctx := context.Background()

		emptyGameRoomSlice := []*game_room.GameRoom{}
		roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(emptyGameRoomSlice, nil)

		err := executor.Execute(ctx, operation, definition)
		require.NoError(t, err)
	})

	t.Run("should succeed - rooms are successfully deleted => returns without error", func(t *testing.T) {
		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		executor := NewExecutor(roomsManager)

		schedulerName := uuid.NewString()
		definition := &RemoveRoomsDefinition{Amount: 2}
		operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

		ctx := context.Background()

		availableRooms := []*game_room.GameRoom{
			{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
			{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
		}
		roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(availableRooms, nil)
		roomsManager.EXPECT().DeleteRoomAndWaitForRoomTerminated(gomock.Any(), gomock.Any()).Return(nil).Times(2)

		err := executor.Execute(ctx, operation, definition)
		require.NoError(t, err)
	})

	t.Run("when any room failed to delete it returns with error", func(t *testing.T) {
		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		executor := NewExecutor(roomsManager)

		schedulerName := uuid.NewString()
		definition := &RemoveRoomsDefinition{Amount: 2}
		operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

		availableRooms := []*game_room.GameRoom{
			{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
			{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady},
		}

		roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(availableRooms, nil)
		roomsManager.EXPECT().DeleteRoomAndWaitForRoomTerminated(gomock.Any(), gomock.Any()).Return(errors.New("error"))
		roomsManager.EXPECT().DeleteRoomAndWaitForRoomTerminated(gomock.Any(), gomock.Any()).Return(nil).Times(1)

		err := executor.Execute(ctx, operation, definition)
		require.EqualError(t, err, "failed to remove room: failed to delete instance on the runtime: some error")
	})

	t.Run("when list rooms has error returns with error", func(t *testing.T) {
		roomsManager := mockports.NewMockRoomManager(mockCtrl)
		executor := NewExecutor(roomsManager)

		definition := &RemoveRoomsDefinition{Amount: 2}
		operation := &operation.Operation{ID: "random-uuid", SchedulerName: uuid.NewString()}

		ctx := context.Background()
		roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))

		err := executor.Execute(ctx, operation, definition)
		require.Error(t, err)
	})
}
