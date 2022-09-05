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

package storagecleanup

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports"
	"go.uber.org/zap"
)

// Executor implements the interface operations.Executor with the storage clean up operation.
type Executor struct {
	operationStorage ports.OperationStorage
}

var _ operations.Executor = (*Executor)(nil)

// NewExecutor returns a new instance of storagecleanup.Executor.
func NewExecutor(operationStorage ports.OperationStorage) *Executor {
	return &Executor{
		operationStorage: operationStorage,
	}
}

// Execute runs the operation storage clean up.
func (e *Executor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, op.DefinitionName),
		zap.String(logs.LogFieldOperationPhase, "Execute"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)

	if err := e.operationStorage.CleanExpiredOperations(ctx, op.SchedulerName); err != nil {
		logger.Warn("failed to clean expired operations references on storage clean up operation", zap.Error(err))
		return fmt.Errorf("failed to clean expired operations references on storage clean up operation: %w", err)
	}

	return nil
}

// Rollback will do nothing.
func (e *Executor) Rollback(_ context.Context, _ *operation.Operation, _ operations.Definition, _ error) error {
	return nil
}

// Name returns the name of the operation.
func (e *Executor) Name() string {
	return OperationName
}
