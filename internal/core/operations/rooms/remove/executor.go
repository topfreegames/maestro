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
	"errors"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	AmountLimit int
}
type Executor struct {
	roomManager      ports.RoomManager
	roomStorage      ports.RoomStorage
	operationManager ports.OperationManager
	schedulerManager ports.SchedulerManager
	config           Config
}

var _ operations.Executor = (*Executor)(nil)

// NewExecutor creates a new RemoveRoomExecutor
func NewExecutor(roomManager ports.RoomManager, roomStorage ports.RoomStorage, operationManager ports.OperationManager, schedulerManager ports.SchedulerManager, config Config) *Executor {
	return &Executor{
		roomManager,
		roomStorage,
		operationManager,
		schedulerManager,
		config,
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
		err := e.removeRoomsByIDs(ctx, op.SchedulerName, removeDefinition.RoomsIDs, op, removeDefinition.Reason)
		if err != nil {
			reportDeletionFailedTotal(op.SchedulerName, op.ID)
			logger.Warn("error removing rooms", zap.Error(err))

			return fmt.Errorf("error removing rooms by ids: %w", err)
		}
		e.operationManager.AppendOperationEventToExecutionHistory(ctx, op, fmt.Sprintf("removed rooms: %v", removeDefinition.RoomsIDs))
	}

	if removeDefinition.Amount > e.config.AmountLimit {
		logger.Info("operation called with amount greater than limit, capping it", zap.Int("calledAmount", removeDefinition.Amount), zap.Int("limit", e.config.AmountLimit))
		removeDefinition.Amount = e.config.AmountLimit
	}

	if removeDefinition.Amount > 0 {
		logger.Info("start removing rooms", zap.Int("amount", removeDefinition.Amount))
		err := e.removeRoomsByAmount(ctx, logger, op.SchedulerName, removeDefinition.Amount, op, removeDefinition.Reason)
		if err != nil {
			reportDeletionFailedTotal(op.SchedulerName, op.ID)
			logger.Warn("error removing rooms", zap.Error(err))

			return fmt.Errorf("error removing rooms by amount: %w", err)
		}
		e.operationManager.AppendOperationEventToExecutionHistory(ctx, op, fmt.Sprintf("removed %d rooms", removeDefinition.Amount))
	}

	logger.Info("finished deleting rooms")
	return nil
}

func (e *Executor) removeRoomsByIDs(ctx context.Context, schedulerName string, roomsIDs []string, op *operation.Operation, reason string) error {
	rooms := make([]*game_room.GameRoom, 0, len(roomsIDs))
	for _, roomID := range roomsIDs {
		gameRoom := &game_room.GameRoom{ID: roomID, SchedulerID: schedulerName}
		rooms = append(rooms, gameRoom)
	}

	err := e.deleteRooms(ctx, rooms, op, reason)
	if err != nil {
		return err
	}

	return nil
}

func (e *Executor) removeRoomsByAmount(ctx context.Context, logger *zap.Logger, schedulerName string, amount int, op *operation.Operation, reason string) error {
	activeScheduler, err := e.schedulerManager.GetActiveScheduler(ctx, schedulerName)
	if err != nil {
		return err
	}

	rooms, err := e.roomManager.ListRoomsWithDeletionPriority(ctx, activeScheduler, amount)
	if err != nil {
		return err
	}

	logger.Info("removing rooms by amount sorting by version",
		zap.Array("rooms:", zapcore.ArrayMarshalerFunc(func(enc zapcore.ArrayEncoder) error {
			for _, room := range rooms {
				enc.AppendString(fmt.Sprintf("%s-%s-%s", room.ID, room.Version, room.Status.String()))
			}
			return nil
		})),
	)

	err = e.deleteRooms(ctx, rooms, op, reason)
	if err != nil {
		return err
	}

	return nil
}

func (e *Executor) deleteRooms(ctx context.Context, rooms []*game_room.GameRoom, op *operation.Operation, reason string) error {
	errs, ctx := errgroup.WithContext(ctx)

	for i := range rooms {
		room := rooms[i]
		errs.Go(func() error {
			err := e.roomManager.DeleteRoom(ctx, room, reason)
			if err != nil {
				msg := fmt.Sprintf("error removing room \"%v\". Reason => %v", room.ID, err.Error())
				e.operationManager.AppendOperationEventToExecutionHistory(ctx, op, msg)
			}
			if errors.Is(err, porterrors.ErrNotFound) {
				return nil
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
