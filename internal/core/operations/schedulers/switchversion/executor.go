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

package switchversion

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"go.uber.org/zap"
)

type Executor struct {
	schedulerManager ports.SchedulerManager
	operationManager ports.OperationManager
}

var _ operations.Executor = (*Executor)(nil)

func NewExecutor(schedulerManager ports.SchedulerManager, operationManager ports.OperationManager) *Executor {
	return &Executor{
		schedulerManager: schedulerManager,
		operationManager: operationManager,
	}
}

func (ex *Executor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, definition.Name()),
		zap.String(logs.LogFieldOperationPhase, "Execute"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	logger.Info("start switching scheduler active version")

	updateDefinition, ok := definition.(*Definition)
	if !ok {
		return fmt.Errorf("the definition is invalid. Should be type Definition")
	}

	scheduler, err := ex.schedulerManager.GetSchedulerByVersion(ctx, op.SchedulerName, updateDefinition.NewActiveVersion)
	if err != nil {
		logger.Error("error fetching scheduler version to be switched to", zap.Error(err))
		getSchedulerErr := fmt.Errorf("error fetching scheduler version to be switched to: %w", err)
		ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, getSchedulerErr.Error())
		return getSchedulerErr
	}

	logger.Sugar().Debugf("switching version to %v", scheduler.Spec.Version)
	scheduler.State = entities.StateInSync
	err = ex.schedulerManager.UpdateScheduler(ctx, scheduler)
	if err != nil {
		logger.Error("error updating scheduler with new active version")
		updateSchedulerErr := fmt.Errorf("error updating scheduler with new active version: %w", err)
		ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, updateSchedulerErr.Error())
		return updateSchedulerErr
	}

	logger.Info("scheduler update finishes with success")
	return nil
}

func (ex *Executor) Rollback(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	return nil
}

func (ex *Executor) Name() string {
	return OperationName
}
