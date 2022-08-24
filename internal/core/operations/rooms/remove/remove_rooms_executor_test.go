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

package remove

import (
	"context"
	"errors"
	"fmt"
	"testing"

	serviceerrors "github.com/topfreegames/maestro/internal/core/services/errors"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

func TestExecute(t *testing.T) {

	t.Run("RemoveRoom by Amount", func(t *testing.T) {
		t.Run("should succeed - no rooms to be removed => returns without error", func(t *testing.T) {
			executor, _, roomsManager, _ := testSetup(t)

			schedulerName := uuid.NewString()
			definition := &RemoveRoomsDefinition{Amount: 2}
			operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			ctx := context.Background()

			emptyGameRoomSlice := []*game_room.GameRoom{}
			roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(emptyGameRoomSlice, nil)

			err := executor.Execute(ctx, operation, definition)
			require.Nil(t, err)
		})

		t.Run("should succeed - rooms are successfully removed => returns without error", func(t *testing.T) {
			executor, _, roomsManager, _ := testSetup(t)

			schedulerName := uuid.NewString()
			definition := &RemoveRoomsDefinition{Amount: 2}
			operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}
			ctx := context.Background()
			availableRooms := []*game_room.GameRoom{
				{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Metadata: map[string]interface{}{}},
				{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Metadata: map[string]interface{}{}},
			}
			roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(availableRooms, nil)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any()).Return(nil).Times(2)
			err := executor.Execute(ctx, operation, definition)

			require.Nil(t, err)
		})

		t.Run("when any room failed to delete with unexpected error it returns with error", func(t *testing.T) {
			executor, _, roomsManager, operationManager := testSetup(t)

			schedulerName := uuid.NewString()
			definition := &RemoveRoomsDefinition{Amount: 2}
			operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			availableRooms := []*game_room.GameRoom{
				{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Metadata: map[string]interface{}{}},
				{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Metadata: map[string]interface{}{}},
			}

			roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(availableRooms, nil)

			roomsManager.EXPECT().DeleteRoom(gomock.Any(), availableRooms[0]).Return(nil)

			roomsManager.EXPECT().DeleteRoom(gomock.Any(), availableRooms[1]).Return(errors.New("failed to remove instance on the runtime: some error"))
			operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), operation, gomock.Any())

			err := executor.Execute(context.Background(), operation, definition)
			require.ErrorContains(t, err, "failed to remove room by amount: failed to remove instance on the runtime: some error")
		})

		t.Run("when any room failed to delete with timeout error it returns with error", func(t *testing.T) {
			executor, _, roomsManager, operationManager := testSetup(t)

			schedulerName := uuid.NewString()
			definition := &RemoveRoomsDefinition{Amount: 2}
			operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			availableRooms := []*game_room.GameRoom{
				{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Metadata: map[string]interface{}{}},
				{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Metadata: map[string]interface{}{}},
			}

			roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(availableRooms, nil)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), availableRooms[0]).Return(nil)

			roomsManager.EXPECT().DeleteRoom(gomock.Any(), availableRooms[1]).Return(serviceerrors.NewErrGameRoomStatusWaitingTimeout("some error"))
			operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), operation, gomock.Any())

			err := executor.Execute(context.Background(), operation, definition)
			require.EqualError(t, err, "failed to remove room by amount: some error")
		})

		t.Run("when list rooms has error returns with error", func(t *testing.T) {
			executor, _, roomsManager, _ := testSetup(t)

			definition := &RemoveRoomsDefinition{Amount: 2}
			operation := &operation.Operation{ID: "random-uuid", SchedulerName: uuid.NewString()}

			ctx := context.Background()
			roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))

			err := executor.Execute(ctx, operation, definition)
			require.NotNil(t, err)
			require.ErrorContains(t, err, "failed to remove room")
		})
	})

	t.Run("RemoveRoom by RoomsIDs", func(t *testing.T) {
		t.Run("should succeed - no rooms to be removed => returns without error", func(t *testing.T) {
			executor, _, _, _ := testSetup(t)

			schedulerName := uuid.NewString()
			definition := &RemoveRoomsDefinition{RoomsIDs: []string{}}
			operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			ctx := context.Background()

			err := executor.Execute(ctx, operation, definition)
			require.Nil(t, err)
		})

		t.Run("should succeed - rooms are successfully removed => returns without error", func(t *testing.T) {
			executor, roomsStorage, roomsManager, _ := testSetup(t)

			firstRoomID := "first-room-id"
			secondRoomID := "second-room-id"

			schedulerName := uuid.NewString()
			definition := &RemoveRoomsDefinition{RoomsIDs: []string{firstRoomID, secondRoomID}}
			operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			ctx := context.Background()

			room := &game_room.GameRoom{
				ID:          firstRoomID,
				SchedulerID: schedulerName,
				Status:      game_room.GameStatusReady,
				Metadata:    map[string]interface{}{},
			}
			secondRoom := &game_room.GameRoom{
				ID:          secondRoomID,
				SchedulerID: schedulerName,
				Status:      game_room.GameStatusReady,
				Metadata:    map[string]interface{}{},
			}
			roomsStorage.EXPECT().GetRoom(gomock.Any(), schedulerName, firstRoomID).Return(room, nil)
			roomsStorage.EXPECT().GetRoom(gomock.Any(), schedulerName, secondRoomID).Return(secondRoom, nil)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), room).Return(nil)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), secondRoom).Return(nil)
			err := executor.Execute(ctx, operation, definition)
			require.Nil(t, err)
		})

		t.Run("when failed to get any room it returns with error", func(t *testing.T) {
			executor, roomsStorage, _, _ := testSetup(t)

			firstRoomID := "first-room-id"
			secondRoomID := "second-room-id"

			schedulerName := uuid.NewString()
			definition := &RemoveRoomsDefinition{RoomsIDs: []string{firstRoomID, secondRoomID}}
			operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			ctx := context.Background()

			room := &game_room.GameRoom{
				ID:          firstRoomID,
				SchedulerID: schedulerName,
				Status:      game_room.GameStatusReady,
			}
			roomsStorage.EXPECT().GetRoom(gomock.Any(), schedulerName, firstRoomID).Return(room, nil)
			roomsStorage.EXPECT().GetRoom(gomock.Any(), schedulerName, secondRoomID).Return(nil, fmt.Errorf("Error on GetRoom"))

			err := executor.Execute(ctx, operation, definition)
			require.ErrorContains(t, err, "failed to remove room by ids")
		})

		t.Run("when any room failed to delete with unexpected error it returns with error", func(t *testing.T) {
			executor, roomsStorage, roomsManager, operationManager := testSetup(t)

			firstRoomID := "first-room-id"
			secondRoomID := "second-room-id"

			schedulerName := uuid.NewString()
			definition := &RemoveRoomsDefinition{RoomsIDs: []string{firstRoomID, secondRoomID}}
			operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			ctx := context.Background()

			room := &game_room.GameRoom{
				ID:          firstRoomID,
				SchedulerID: schedulerName,
				Status:      game_room.GameStatusReady,
			}
			secondRoom := &game_room.GameRoom{
				ID:          firstRoomID,
				SchedulerID: schedulerName,
				Status:      game_room.GameStatusReady,
			}
			roomsStorage.EXPECT().GetRoom(gomock.Any(), schedulerName, firstRoomID).Return(room, nil)
			roomsStorage.EXPECT().GetRoom(gomock.Any(), schedulerName, secondRoomID).Return(secondRoom, nil)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), room).Return(nil)

			roomsManager.EXPECT().DeleteRoom(gomock.Any(), secondRoom).Return(fmt.Errorf("Error on remove room"))
			operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), operation, gomock.Any())

			err := executor.Execute(ctx, operation, definition)
			require.ErrorContains(t, err, "failed to remove room by ids:")
		})

		t.Run("when any room failed to delete with timeout error it returns with error", func(t *testing.T) {
			executor, roomsStorage, roomsManager, operationManager := testSetup(t)

			firstRoomID := "first-room-id"
			secondRoomID := "second-room-id"

			schedulerName := uuid.NewString()
			definition := &RemoveRoomsDefinition{RoomsIDs: []string{firstRoomID, secondRoomID}}
			operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}
			ctx := context.Background()

			room := &game_room.GameRoom{
				ID:          firstRoomID,
				SchedulerID: schedulerName,
				Status:      game_room.GameStatusReady,
			}
			secondRoom := &game_room.GameRoom{
				ID:          secondRoomID,
				SchedulerID: schedulerName,
				Status:      game_room.GameStatusReady,
			}
			roomsStorage.EXPECT().GetRoom(gomock.Any(), schedulerName, firstRoomID).Return(room, nil)
			roomsStorage.EXPECT().GetRoom(gomock.Any(), schedulerName, secondRoomID).Return(secondRoom, nil)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), room).Return(nil)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), secondRoom).Return(serviceerrors.NewErrGameRoomStatusWaitingTimeout("some error"))
			operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), operation, gomock.Any())

			err := executor.Execute(ctx, operation, definition)

			require.ErrorContains(t, err, "failed to remove room")
		})
	})

	t.Run("should succeed - no rooms to be removed => returns without error", func(t *testing.T) {
		executor, _, _, _ := testSetup(t)

		schedulerName := uuid.NewString()
		definition := &RemoveRoomsDefinition{}
		operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

		ctx := context.Background()

		err := executor.Execute(ctx, operation, definition)
		require.Nil(t, err)
	})

	t.Run("should succeed - there are ids and amount => return without error", func(t *testing.T) {
		executor, roomsStorage, roomsManager, _ := testSetup(t)

		firstRoomID := "first-room-id"
		secondRoomID := "second-room-id"
		thirdRoomID := "third-room-id"
		fourthRoomID := "fourth-room-id"

		schedulerName := uuid.NewString()
		definition := &RemoveRoomsDefinition{
			RoomsIDs: []string{firstRoomID, secondRoomID},
			Amount:   2,
		}
		operation := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

		ctx := context.Background()

		room := &game_room.GameRoom{
			ID:          firstRoomID,
			SchedulerID: schedulerName,
			Status:      game_room.GameStatusReady,
		}
		secondRoom := &game_room.GameRoom{
			ID:          secondRoomID,
			SchedulerID: schedulerName,
			Status:      game_room.GameStatusReady,
		}
		thirdRoom := &game_room.GameRoom{
			ID:          thirdRoomID,
			SchedulerID: schedulerName,
			Status:      game_room.GameStatusReady,
		}
		fourthRoom := &game_room.GameRoom{
			ID:          fourthRoomID,
			SchedulerID: schedulerName,
			Status:      game_room.GameStatusReady,
		}
		roomsStorage.EXPECT().GetRoom(gomock.Any(), schedulerName, firstRoomID).Return(room, nil)
		roomsStorage.EXPECT().GetRoom(gomock.Any(), schedulerName, secondRoomID).Return(secondRoom, nil)
		roomsManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any()).Return(nil).Times(2)

		availableRooms := []*game_room.GameRoom{thirdRoom, fourthRoom}
		roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(availableRooms, nil)
		roomsManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any()).Return(nil).Times(2)
		err := executor.Execute(ctx, operation, definition)
		require.Nil(t, err)
	})

}

func testSetup(t *testing.T) (*RemoveRoomsExecutor, *mockports.MockRoomStorage, *mockports.MockRoomManager, *mockports.MockOperationManager) {
	mockCtrl := gomock.NewController(t)

	roomsStorage := mockports.NewMockRoomStorage(mockCtrl)
	roomsManager := mockports.NewMockRoomManager(mockCtrl)
	operationManager := mockports.NewMockOperationManager(mockCtrl)
	executor := NewExecutor(roomsManager, roomsStorage, operationManager)
	return executor, roomsStorage, roomsManager, operationManager
}
