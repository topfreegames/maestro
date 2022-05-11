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

package remove_rooms

import (
	"context"
	"errors"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities/events"

	"sync"

	serviceerrors "github.com/topfreegames/maestro/internal/core/services/errors"

	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"go.uber.org/zap"
)

type RemoveRoomsExecutor struct {
	roomManager   ports.RoomManager
	roomStorage   ports.RoomStorage
	eventsService ports.EventsService
}

var _ operations.Executor = (*RemoveRoomsExecutor)(nil)

// NewExecutor creates a new RemoveRoomExecutor
func NewExecutor(roomManager ports.RoomManager, roomStorage ports.RoomStorage, eventsService ports.EventsService) *RemoveRoomsExecutor {
	return &RemoveRoomsExecutor{
		roomManager,
		roomStorage,
		eventsService,
	}
}

// Execute execute operation RemoveRoom
func (e *RemoveRoomsExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) operations.ExecutionError {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, definition.Name()),
		zap.String(logs.LogFieldOperationID, op.ID),
	)

	removeDefinition := definition.(*RemoveRoomsDefinition)

	if len(removeDefinition.RoomsIDs) > 0 {
		logger.Info("start removing rooms", zap.Strings("RoomIDs", removeDefinition.RoomsIDs))
		err := e.removeRoomsByIDs(ctx, logger, op.SchedulerName, removeDefinition.RoomsIDs)
		if err != nil {
			reportDeletionFailedTotal(op.SchedulerName, op.ID)
			logger.Warn("failed to remove rooms", zap.Error(err))
			deleteErr := fmt.Errorf("failed to remove room: %w", err)

			if errors.Is(err, serviceerrors.ErrGameRoomStatusWaitingTimeout) {
				return operations.NewErrTerminatingPingTimeout(deleteErr)
			}

			return operations.NewErrUnexpected(fmt.Errorf("failed to remove room by ids: %w", err))
		}
	}

	if removeDefinition.Amount > 0 {
		logger.Info("start removing rooms", zap.Int("amount", removeDefinition.Amount))
		err := e.removeRoomsByAmount(ctx, logger, op.SchedulerName, removeDefinition.Amount)
		if err != nil {
			reportDeletionFailedTotal(op.SchedulerName, op.ID)
			logger.Warn("failed to remove rooms", zap.Error(err))
			deleteErr := fmt.Errorf("failed to remove room: %w", err)

			if errors.Is(err, serviceerrors.ErrGameRoomStatusWaitingTimeout) {
				return operations.NewErrTerminatingPingTimeout(deleteErr)
			}

			return operations.NewErrUnexpected(fmt.Errorf("failed to remove room by amount: %w", err))
		}
	}

	logger.Info("finished deleting rooms")
	return nil
}

func (e *RemoveRoomsExecutor) removeRoomsByIDs(ctx context.Context, logger *zap.Logger, schedulerName string, roomsIDs []string) error {
	rooms := make([]*game_room.GameRoom, 0, len(roomsIDs))
	for _, roomID := range roomsIDs {
		gameRoom, err := e.roomStorage.GetRoom(ctx, schedulerName, roomID)
		if err != nil {
			return err
		}

		rooms = append(rooms, gameRoom)
	}

	err := e.deleteRooms(ctx, logger, rooms)
	if err != nil {
		return err
	}

	return nil
}

func (e *RemoveRoomsExecutor) removeRoomsByAmount(ctx context.Context, logger *zap.Logger, schedulerName string, amount int) error {
	rooms, err := e.roomManager.ListRoomsWithDeletionPriority(ctx, schedulerName, "", amount, &sync.Map{})
	if err != nil {
		return err
	}

	err = e.deleteRooms(ctx, logger, rooms)
	if err != nil {
		return err
	}

	return nil
}

func (e *RemoveRoomsExecutor) deleteRooms(ctx context.Context, logger *zap.Logger, rooms []*game_room.GameRoom) error {
	var err error
	for _, room := range rooms {
		err = e.roomManager.DeleteRoomAndWaitForRoomTerminated(ctx, room)
		if err != nil {
			return err
		}
		e.forwardStatusTerminatingEvent(ctx, logger, room)
	}

	return nil
}

// Rollback applies the correct rollback to RemoveRoom
func (e *RemoveRoomsExecutor) Rollback(_ context.Context, _ *operation.Operation, _ operations.Definition, _ operations.ExecutionError) error {
	return nil
}

// Name returns the operation name
func (e *RemoveRoomsExecutor) Name() string {
	return OperationName
}

func (e *RemoveRoomsExecutor) forwardStatusTerminatingEvent(ctx context.Context, logger *zap.Logger, room *game_room.GameRoom) {
	if room.Metadata == nil {
		room.Metadata = map[string]interface{}{}
	}
	room.Metadata["eventType"] = events.FromRoomEventTypeToString(events.Arbitrary)
	room.Metadata["roomEvent"] = game_room.GameStatusTerminating.String()

	err := e.eventsService.ProduceEvent(ctx, events.NewRoomEvent(room.SchedulerID, room.ID, room.Metadata))
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to forward ping event, error details: %s", err.Error()), zap.Error(err))
	}
}
