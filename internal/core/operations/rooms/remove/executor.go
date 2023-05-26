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

package remove

import (
	"context"
	"fmt"
	"sync"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Executor struct {
	roomManager      ports.RoomManager
	roomStorage      ports.RoomStorage
	operationManager ports.OperationManager
}

var _ operations.Executor = (*Executor)(nil)

// NewExecutor creates a new RemoveRoomExecutor
func NewExecutor(roomManager ports.RoomManager, roomStorage ports.RoomStorage, operationManager ports.OperationManager) *Executor {
	return &Executor{
		roomManager,
		roomStorage,
		operationManager,
	}
}

// Execute execute operation RemoveRoom
func (e *Executor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, definition.Name()),
		zap.String(logs.LogFieldOperationPhase, "Execute"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)

	removeDefinition := definition.(*Definition)

	if len(removeDefinition.RoomsIDs) > 0 {
		logger.Info("start removing rooms", zap.Strings("RoomIDs", removeDefinition.RoomsIDs))
		err := e.removeRoomsByIDs(ctx, op.SchedulerName, removeDefinition.RoomsIDs, op)
		if err != nil {
			reportDeletionFailedTotal(op.SchedulerName, op.ID)
			logger.Warn("error removing rooms", zap.Error(err))

			return fmt.Errorf("error removing rooms by ids: %w", err)
		}
		var deadRoomsIDs []string
		for _, id := range removeDefinition.RoomsIDs {
			deadRoomsIDs = append(deadRoomsIDs, id)
		}
		logger.Info(fmt.Sprintf("[wps-3544] RemoveDeadRoom - Scheduler %s, GameRooms: %s",
			op.SchedulerName, deadRoomsIDs))
	}

	if removeDefinition.Amount > 0 {
		logger.Info("start removing rooms", zap.Int("amount", removeDefinition.Amount))
		err := e.removeRoomsByAmount(ctx, op.SchedulerName, removeDefinition.Amount, op)
		if err != nil {
			reportDeletionFailedTotal(op.SchedulerName, op.ID)
			logger.Warn("error removing rooms", zap.Error(err))

			return fmt.Errorf("error removing rooms by amount: %w", err)
		}
	}

	logger.Info("finished deleting rooms")
	return nil
}

func (e *Executor) removeRoomsByIDs(ctx context.Context, schedulerName string, roomsIDs []string, op *operation.Operation) error {
	rooms := make([]*game_room.GameRoom, 0, len(roomsIDs))
	for _, roomID := range roomsIDs {
		gameRoom, err := e.roomStorage.GetRoom(ctx, schedulerName, roomID)
		if err != nil {
			return err
		}

		rooms = append(rooms, gameRoom)
	}

	err := e.deleteRooms(ctx, rooms, op)
	if err != nil {
		return err
	}

	return nil
}

func (e *Executor) removeRoomsByAmount(ctx context.Context, schedulerName string, amount int, op *operation.Operation) error {
	rooms, err := e.roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", amount, &sync.Map{})
	if err != nil {
		return err
	}

	err = e.deleteRooms(ctx, rooms, op)
	if err != nil {
		return err
	}

	return nil
}

func (e *Executor) deleteRooms(ctx context.Context, rooms []*game_room.GameRoom, op *operation.Operation) error {
	errs, ctx := errgroup.WithContext(ctx)

	for i := range rooms {
		room := rooms[i]
		errs.Go(func() error {
			err := e.roomManager.DeleteRoom(ctx, room)
			if err != nil {
				msg := fmt.Sprintf("error removing room \"%v\". Reason => %v", room.ID, err.Error())
				e.operationManager.AppendOperationEventToExecutionHistory(ctx, op, msg)
			}
			return err
		})
	}

	if err := errs.Wait(); err != nil {
		return err
	}

	return nil
}

// Rollback applies the correct rollback to RemoveRoom
func (e *Executor) Rollback(_ context.Context, _ *operation.Operation, _ operations.Definition, _ error) error {
	return nil
}

// Name returns the operation name
func (e *Executor) Name() string {
	return OperationName
}
