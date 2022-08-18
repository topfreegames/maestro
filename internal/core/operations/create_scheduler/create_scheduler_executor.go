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

package create_scheduler

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

type CreateSchedulerExecutor struct {
	runtime          ports.Runtime
	schedulerManager ports.SchedulerManager
}

var _ operations.Executor = (*CreateSchedulerExecutor)(nil)

func NewExecutor(runtime ports.Runtime, schedulerManager ports.SchedulerManager) *CreateSchedulerExecutor {
	return &CreateSchedulerExecutor{
		runtime:          runtime,
		schedulerManager: schedulerManager,
	}
}

func (e *CreateSchedulerExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, op.DefinitionName),
		zap.String(logs.LogFieldOperationPhase, "Execute"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)

	err := e.runtime.CreateScheduler(ctx, &entities.Scheduler{Name: op.SchedulerName})
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create scheduler %s in runtime", op.SchedulerName), zap.Error(err))
		return err
	}

	logger.Info(fmt.Sprintf("%s operation succeded, %s scheduler was created", definition.Name(), op.SchedulerName))
	return nil
}

func (e *CreateSchedulerExecutor) Rollback(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, op.DefinitionName),
		zap.String(logs.LogFieldOperationPhase, "Rollback"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)

	err := e.schedulerManager.DeleteScheduler(ctx, op.SchedulerName)
	if err != nil {
		logger.Error(fmt.Sprintf("error deleting scheduler %s", op.SchedulerName), zap.Error(err))
		return fmt.Errorf("error in Rollback function execution: %w", err)
	}
	return nil
}

func (e *CreateSchedulerExecutor) Name() string {
	return OperationName
}
