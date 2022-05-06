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

package healthcontroller

import (
	"context"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/logs"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"go.uber.org/zap"
)

type SchedulerHealthControllerExecutor struct {
	roomManager          ports.RoomManager
	schedulerManager     ports.SchedulerManager
	validationRoomIdsMap map[string]*game_room.GameRoom
}

var _ operations.Executor = (*SchedulerHealthControllerExecutor)(nil)

func NewExecutor(roomManager ports.RoomManager, schedulerManager ports.SchedulerManager) *SchedulerHealthControllerExecutor {
	return &SchedulerHealthControllerExecutor{
		roomManager:          roomManager,
		schedulerManager:     schedulerManager,
		validationRoomIdsMap: map[string]*game_room.GameRoom{},
	}
}

func (ex *SchedulerHealthControllerExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) operations.ExecutionError {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, op.DefinitionName),
		zap.String("operation_phase", "Execute"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	logger.Info("dale")
	return nil
}

func (ex *SchedulerHealthControllerExecutor) Rollback(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr operations.ExecutionError) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, op.DefinitionName),
		zap.String("operation_phase", "Rollback"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	logger.Info("dale")
	return nil
}

func (ex *SchedulerHealthControllerExecutor) Name() string {
	return OperationName
}
