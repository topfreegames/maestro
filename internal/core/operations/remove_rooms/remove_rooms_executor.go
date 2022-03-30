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
	"sync"

	serviceerrors "github.com/topfreegames/maestro/internal/core/services/errors"

	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"go.uber.org/zap"
)

type RemoveRoomsExecutor struct {
	roomManager ports.RoomManager
}

var _ operations.Executor = (*RemoveRoomsExecutor)(nil)

func NewExecutor(roomManager ports.RoomManager) *RemoveRoomsExecutor {
	return &RemoveRoomsExecutor{roomManager}
}

func (e *RemoveRoomsExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) operations.ExecutionError {
	removeDefinition := definition.(*RemoveRoomsDefinition)
	rooms, err := e.roomManager.ListRoomsWithDeletionPriority(ctx, op.SchedulerName, "", removeDefinition.Amount, &sync.Map{})
	if err != nil {
		return operations.NewErrUnexpected(fmt.Errorf("failed to list rooms to delete: %w", err))
	}

	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, definition.Name()),
		zap.String(logs.LogFieldOperationID, op.ID),
	)

	logger.Info("start deleting rooms", zap.Int("amount", len(rooms)))

	for _, room := range rooms {
		err = e.roomManager.DeleteRoomAndWaitForRoomTerminated(ctx, room)
		if err != nil {
			reportDeletionFailedTotal(op.SchedulerName, op.ID)
			logger.Warn("failed to remove rooms", zap.Error(err))
			deleteErr := fmt.Errorf("failed to remove room: %w", err)

			if errors.Is(err, serviceerrors.ErrGameRoomStatusWaitingTimeout) {
				return operations.NewErrTerminatingPingTimeout(deleteErr)
			}
			return operations.NewErrUnexpected(deleteErr)
		}
	}

	logger.Info("finished deleting rooms")
	return nil
}

func (e *RemoveRoomsExecutor) Rollback(_ context.Context, _ *operation.Operation, _ operations.Definition, _ operations.ExecutionError) error {
	return nil
}

func (e *RemoveRoomsExecutor) Name() string {
	return OperationName
}
