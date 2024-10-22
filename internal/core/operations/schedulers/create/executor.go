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

package create

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports"
	"go.uber.org/zap"
)

type Executor struct {
	runtime          ports.Runtime
	schedulerManager ports.SchedulerManager
	operationManager ports.OperationManager
}

var _ operations.Executor = (*Executor)(nil)

func NewExecutor(runtime ports.Runtime, schedulerManager ports.SchedulerManager, operationManager ports.OperationManager) *Executor {
	return &Executor{
		runtime:          runtime,
		schedulerManager: schedulerManager,
		operationManager: operationManager,
	}
}

func (e *Executor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, op.DefinitionName),
		zap.String(logs.LogFieldOperationPhase, "Execute"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)

	opDef, ok := definition.(*Definition)
	if !ok {
		return fmt.Errorf("invalid operation definition for %s operation", e.Name())
	}

	err := e.runtime.CreateScheduler(ctx, &entities.Scheduler{Name: op.SchedulerName})
	if err != nil {
		logger.Error("error creating scheduler in runtime", zap.Error(err))
		createSchedulerErr := fmt.Errorf("error creating scheduler in runtime: %w", err)
		e.operationManager.AppendOperationEventToExecutionHistory(ctx, op, createSchedulerErr.Error())
		return createSchedulerErr
	}

	newScheduler := opDef.NewScheduler
	newScheduler.State = entities.StateInSync

	err = e.schedulerManager.UpdateScheduler(ctx, newScheduler)
	if err != nil {
		logger.Error("error updating scheduler state", zap.Error(err))
		updateSchedulerErr := fmt.Errorf("error updating scheduler state: %w", err)
		e.operationManager.AppendOperationEventToExecutionHistory(ctx, op, updateSchedulerErr.Error())
		return updateSchedulerErr
	}

	logger.Info("operation succeeded, scheduler was created")
	return nil
}

func (e *Executor) Rollback(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, op.DefinitionName),
		zap.String(logs.LogFieldOperationPhase, "Rollback"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)

	err := e.schedulerManager.DeleteScheduler(ctx, op.SchedulerName)
	if err != nil {
		logger.Error("error deleting newly created scheduler", zap.Error(err))
		deleteSchedulerErr := fmt.Errorf("error deleting newly created scheduler: %w", err)
		e.operationManager.AppendOperationEventToExecutionHistory(ctx, op, deleteSchedulerErr.Error())
		return deleteSchedulerErr
	}
	return nil
}

func (e *Executor) Name() string {
	return OperationName
}
