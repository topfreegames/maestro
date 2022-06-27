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

package deletescheduler

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/operations"
	"go.uber.org/zap"
)

type DeleteSchedulerExecutor struct {
}

var _ operations.Executor = (*DeleteSchedulerExecutor)(nil)

func NewExecutor() *DeleteSchedulerExecutor {
	return &DeleteSchedulerExecutor{}
}

// Execute deletes the scheduler, cleaning all bounded resources in the runtime and on storages.
func (e *DeleteSchedulerExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) operations.ExecutionError {
	_ = zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, op.DefinitionName),
		zap.String(logs.LogFieldOperationPhase, "Execute"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)

	return nil
}

// Rollback does nothing.
func (e *DeleteSchedulerExecutor) Rollback(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr operations.ExecutionError) error {
	return nil
}

// Name returns the name of the Operation.
func (e *DeleteSchedulerExecutor) Name() string {
	return OperationName
}
