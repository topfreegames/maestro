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

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
	serviceerrors "github.com/topfreegames/maestro/internal/core/services/errors"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

func TestExecutor_Execute(t *testing.T) {

	t.Run("RemoveRoom by Amount", func(t *testing.T) {
		t.Run("should succeed - no rooms to be removed => returns without error", func(t *testing.T) {
			executor, _, roomsManager, _, schedulerManager := testSetup(t)

			schedulerName := uuid.NewString()
			definition := &Definition{Amount: 2}
			op := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			ctx := context.Background()

			emptyGameRoomSlice := []*game_room.GameRoom{}
			schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), schedulerName).Return(nil, errors.New("error getting active scheduler"))
			roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(emptyGameRoomSlice, nil)

			err := executor.Execute(ctx, op, definition)
			require.Nil(t, err)
		})

		t.Run("should succeed and sort rooms by active scheduler version", func(t *testing.T) {
			executor, _, roomsManager, _, schedulerManager := testSetup(t)

			schedulerName := uuid.NewString()
			schedulerV1 := &entities.Scheduler{
				Name: schedulerName,
				Spec: game_room.Spec{
					Version: "v1",
				},
			}
			schedulerV2 := &entities.Scheduler{
				Name: schedulerName,
				Spec: game_room.Spec{
					Version: "v2",
				},
			}
			definition := &Definition{Amount: 8}
			op := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}
			ctx := context.Background()
			/*
				Scheduler Versions and Rooms
					Current Rooms: [Err1v1, Err2v2, P1v1, P2v2, R1v1, R2v2, O1v1, O2v2]
					Expected List by Priority: [Err1v1, Err2v2, P1v1, R1v1, O1v1, P2v2, R2v2, O2v2]
			*/
			availableRooms := []*game_room.GameRoom{
				{ID: "Err1v1", SchedulerID: schedulerName, Status: game_room.GameStatusError, Version: schedulerV1.Spec.Version},
				{ID: "Err2v2", SchedulerID: schedulerName, Status: game_room.GameStatusError, Version: schedulerV2.Spec.Version},
				{ID: "P1v1", SchedulerID: schedulerName, Status: game_room.GameStatusPending, Version: schedulerV1.Spec.Version},
				{ID: "P2v2", SchedulerID: schedulerName, Status: game_room.GameStatusPending, Version: schedulerV2.Spec.Version},
				{ID: "R1v1", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Version: schedulerV1.Spec.Version},
				{ID: "R2v2", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Version: schedulerV2.Spec.Version},
				{ID: "O1v1", SchedulerID: schedulerName, Status: game_room.GameStatusOccupied, Version: schedulerV1.Spec.Version},
				{ID: "O2v2", SchedulerID: schedulerName, Status: game_room.GameStatusOccupied, Version: schedulerV2.Spec.Version},
			}
			expectedSortedRoomsOrder := []*game_room.GameRoom{
				availableRooms[0], // Err1v1
				availableRooms[1], // Err2v2
				availableRooms[2], // P1v1
				availableRooms[4], // R1v1
				availableRooms[6], // O1v1
				availableRooms[3], // P2v2
				availableRooms[5], // R2v2
				availableRooms[7], // P2v2
			}
			schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), schedulerName).Return(schedulerV2, nil)
			roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(availableRooms, nil)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), expectedSortedRoomsOrder[0], gomock.Any()).Times(1)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), expectedSortedRoomsOrder[1], gomock.Any()).Times(1)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), expectedSortedRoomsOrder[2], gomock.Any()).Times(1)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), expectedSortedRoomsOrder[3], gomock.Any()).Times(1)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), expectedSortedRoomsOrder[4], gomock.Any()).Times(1)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), expectedSortedRoomsOrder[5], gomock.Any()).Times(1)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), expectedSortedRoomsOrder[6], gomock.Any()).Times(1)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), expectedSortedRoomsOrder[7], gomock.Any()).Times(1)
			err := executor.Execute(ctx, op, definition)

			require.Nil(t, err)
		})

		t.Run("when any room failed to delete with unexpected error it returns with error", func(t *testing.T) {
			executor, _, roomsManager, operationManager, schedulerManager := testSetup(t)

			schedulerName := uuid.NewString()
			definition := &Definition{Amount: 2, Reason: "reason"}
			op := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			availableRooms := []*game_room.GameRoom{
				{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Metadata: map[string]interface{}{}},
				{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Metadata: map[string]interface{}{}},
			}

			schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), schedulerName).Return(nil, errors.New("error getting active scheduler"))
			roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(availableRooms, nil)

			roomsManager.EXPECT().DeleteRoom(gomock.Any(), availableRooms[0], definition.Reason).Return(nil)

			roomsManager.EXPECT().DeleteRoom(gomock.Any(), availableRooms[1], definition.Reason).Return(errors.New("failed to remove instance on the runtime: some error"))
			operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), op, gomock.Any())

			err := executor.Execute(context.Background(), op, definition)
			require.ErrorContains(t, err, "error removing rooms by amount: failed to remove instance on the runtime: some error")
		})

		t.Run("when any room failed to delete with timeout error it returns with error", func(t *testing.T) {
			executor, _, roomsManager, operationManager, schedulerManager := testSetup(t)

			schedulerName := uuid.NewString()
			definition := &Definition{Amount: 2, Reason: "reason"}
			op := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			availableRooms := []*game_room.GameRoom{
				{ID: "first-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Metadata: map[string]interface{}{}},
				{ID: "second-room", SchedulerID: schedulerName, Status: game_room.GameStatusReady, Metadata: map[string]interface{}{}},
			}

			schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), schedulerName).Return(nil, errors.New("error getting active scheduler"))
			roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(availableRooms, nil)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), availableRooms[0], definition.Reason).Return(nil)

			roomsManager.EXPECT().DeleteRoom(gomock.Any(), availableRooms[1], definition.Reason).Return(serviceerrors.NewErrGameRoomStatusWaitingTimeout("some error"))
			operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), op, gomock.Any())

			err := executor.Execute(context.Background(), op, definition)
			require.EqualError(t, err, "error removing rooms by amount: some error")
		})

		t.Run("when list rooms has error returns with error", func(t *testing.T) {
			executor, _, roomsManager, _, _ := testSetup(t)

			schedulerName := uuid.NewString()
			definition := &Definition{Amount: 2}
			op := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))

			err := executor.Execute(context.Background(), op, definition)
			require.NotNil(t, err)
			require.ErrorContains(t, err, "error removing rooms by amount: error")
		})
	})

	t.Run("RemoveRoom by RoomsIDs", func(t *testing.T) {
		t.Run("should succeed - no rooms to be removed => returns without error", func(t *testing.T) {
			executor, _, _, _, _ := testSetup(t)

			schedulerName := uuid.NewString()
			definition := &Definition{RoomsIDs: []string{}}
			op := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			err := executor.Execute(context.Background(), op, definition)
			require.Nil(t, err)
		})

		t.Run("should succeed - rooms are successfully removed => returns without error", func(t *testing.T) {
			executor, _, roomsManager, _, _ := testSetup(t)

			firstRoomID := "first-room-id"
			secondRoomID := "second-room-id"

			schedulerName := uuid.NewString()
			definition := &Definition{RoomsIDs: []string{firstRoomID, secondRoomID}, Reason: "reason"}
			op := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			room := &game_room.GameRoom{
				ID:          firstRoomID,
				SchedulerID: schedulerName,
			}
			secondRoom := &game_room.GameRoom{
				ID:          secondRoomID,
				SchedulerID: schedulerName,
			}

			roomsManager.EXPECT().DeleteRoom(gomock.Any(), room, definition.Reason).Return(nil)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), secondRoom, definition.Reason).Return(nil)

			err := executor.Execute(context.Background(), op, definition)
			require.Nil(t, err)
		})

		t.Run("when any room failed to delete with unexpected error it returns with error", func(t *testing.T) {
			executor, _, roomsManager, operationManager, _ := testSetup(t)

			firstRoomID := "first-room-id"
			secondRoomID := "second-room-id"

			schedulerName := uuid.NewString()
			definition := &Definition{RoomsIDs: []string{firstRoomID, secondRoomID}, Reason: "reason"}
			op := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			room := &game_room.GameRoom{
				ID:          firstRoomID,
				SchedulerID: schedulerName,
			}
			secondRoom := &game_room.GameRoom{
				ID:          secondRoomID,
				SchedulerID: schedulerName,
			}

			roomsManager.EXPECT().DeleteRoom(gomock.Any(), room, definition.Reason).Return(nil)

			roomsManager.EXPECT().DeleteRoom(gomock.Any(), secondRoom, definition.Reason).Return(fmt.Errorf("error on remove room"))
			operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), op, gomock.Any())

			err := executor.Execute(context.Background(), op, definition)
			require.ErrorContains(t, err, "error removing rooms by ids: error on remove room")
		})

		t.Run("when any room returns not found on delete it is ignored and returns without error", func(t *testing.T) {
			executor, _, roomsManager, operationManager, _ := testSetup(t)

			firstRoomID := "first-room-id"
			secondRoomID := "second-room-id"

			schedulerName := uuid.NewString()
			definition := &Definition{RoomsIDs: []string{firstRoomID, secondRoomID}}
			op := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			room := &game_room.GameRoom{
				ID:          firstRoomID,
				SchedulerID: schedulerName,
			}
			secondRoom := &game_room.GameRoom{
				ID:          secondRoomID,
				SchedulerID: schedulerName,
			}

			roomsManager.EXPECT().DeleteRoom(gomock.Any(), room, definition.Reason).Return(nil)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), secondRoom, definition.Reason).Return(porterrors.NewErrNotFound("not found"))
			operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), op, gomock.Any())

			err := executor.Execute(context.Background(), op, definition)
			require.NoError(t, err)
		})

		t.Run("when any room failed to delete with timeout error it returns with error", func(t *testing.T) {
			executor, _, roomsManager, operationManager, _ := testSetup(t)

			firstRoomID := "first-room-id"
			secondRoomID := "second-room-id"

			schedulerName := uuid.NewString()
			definition := &Definition{RoomsIDs: []string{firstRoomID, secondRoomID}, Reason: "reason"}
			op := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

			room := &game_room.GameRoom{
				ID:          firstRoomID,
				SchedulerID: schedulerName,
			}
			secondRoom := &game_room.GameRoom{
				ID:          secondRoomID,
				SchedulerID: schedulerName,
			}

			roomsManager.EXPECT().DeleteRoom(gomock.Any(), room, definition.Reason).Return(nil)
			roomsManager.EXPECT().DeleteRoom(gomock.Any(), secondRoom, definition.Reason).Return(serviceerrors.NewErrGameRoomStatusWaitingTimeout("some error"))
			operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), op, gomock.Any())

			err := executor.Execute(context.Background(), op, definition)

			require.ErrorContains(t, err, "error removing rooms by ids: some error")
		})
	})

	t.Run("should succeed - no rooms to be removed => returns without error", func(t *testing.T) {
		executor, _, _, _, _ := testSetup(t)

		schedulerName := uuid.NewString()
		definition := &Definition{}
		op := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

		err := executor.Execute(context.Background(), op, definition)
		require.Nil(t, err)
	})

	t.Run("should succeed - there are ids and amount => return without error", func(t *testing.T) {
		executor, _, roomsManager, _, schedulerManager := testSetup(t)

		firstRoomID := "first-room-id"
		secondRoomID := "second-room-id"
		thirdRoomID := "third-room-id"
		fourthRoomID := "fourth-room-id"

		schedulerName := uuid.NewString()
		definition := &Definition{
			RoomsIDs: []string{firstRoomID, secondRoomID},
			Amount:   2,
			Reason:   "reason",
		}
		op := &operation.Operation{ID: "random-uuid", SchedulerName: schedulerName}

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

		roomsManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any(), definition.Reason).Return(nil).Times(2)

		availableRooms := []*game_room.GameRoom{thirdRoom, fourthRoom}
		schedulerManager.EXPECT().GetActiveScheduler(gomock.Any(), schedulerName).Return(nil, errors.New("error getting active scheduler"))
		roomsManager.EXPECT().ListRoomsWithDeletionPriority(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(availableRooms, nil)
		roomsManager.EXPECT().DeleteRoom(gomock.Any(), gomock.Any(), definition.Reason).Return(nil).Times(2)

		err := executor.Execute(context.Background(), op, definition)
		require.Nil(t, err)
	})
}

func testSetup(t *testing.T) (*Executor, *mockports.MockRoomStorage, *mockports.MockRoomManager, *mockports.MockOperationManager, *mockports.MockSchedulerManager) {
	mockCtrl := gomock.NewController(t)

	roomsStorage := mockports.NewMockRoomStorage(mockCtrl)
	roomsManager := mockports.NewMockRoomManager(mockCtrl)
	operationManager := mockports.NewMockOperationManager(mockCtrl)
	schedulerManager := mockports.NewMockSchedulerManager(mockCtrl)
	executor := NewExecutor(roomsManager, roomsStorage, operationManager, schedulerManager)
	return executor, roomsStorage, roomsManager, operationManager, schedulerManager
}
