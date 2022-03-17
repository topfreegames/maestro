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

package operation_execution_worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/topfreegames/maestro/internal/core/logs"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/workers"
	"go.uber.org/zap"
)

var _ workers.Worker = (*OperationExecutionWorker)(nil)

// OperationExecutionWorker is the service responsible for implementing the worker
// responsibilities.
type OperationExecutionWorker struct {
	schedulerName    string
	operationManager ports.OperationManager
	// TODO(gabrielcorado): check if we this is the right place to have all
	// executors.
	executorsByName     map[string]operations.Executor
	workerContext       context.Context
	cancelWorkerContext context.CancelFunc

	logger *zap.Logger
}

func NewOperationExecutionWorker(scheduler *entities.Scheduler, opts *workers.WorkerOptions) workers.Worker {
	return &OperationExecutionWorker{
		operationManager: opts.OperationManager,
		executorsByName:  opts.OperationExecutors,
		schedulerName:    scheduler.Name,
		logger:           zap.L().With(zap.String(logs.LogFieldServiceName, "worker"), zap.String(logs.LogFieldSchedulerName, scheduler.Name)),
	}
}

// Start is responsible for starting a loop that will
// constantly look to execute operations, and this loop can be canceled using
// the provided context. NOTE: It is a blocking function.
func (w *OperationExecutionWorker) Start(ctx context.Context) error {
	w.workerContext, w.cancelWorkerContext = context.WithCancel(ctx)

	for {
		op, def, err := w.operationManager.NextSchedulerOperation(w.workerContext, w.schedulerName)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}

			w.Stop(ctx)
			reportOperationExecutionWorkerFailed(w.schedulerName, LabelNextOperationFailed)
			return fmt.Errorf("failed to get next operation: %w", err)
		}

		loopLogger := w.logger.With(
			zap.String(logs.LogFieldOperationID, op.ID),
			zap.String(logs.LogFieldOperationDefinition, def.Name()),
		)

		if op.Status != operation.StatusPending {
			err = w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Operation is at an invalid status to proceed")
			if err != nil {
				return fmt.Errorf("Error appending operation event to execution history: %w", err)
			}

			loopLogger.Warn("operation is at an invalid status to proceed")
			continue
		}

		executor, hasExecutor := w.executorsByName[def.Name()]
		if !hasExecutor {
			loopLogger.Warn("operation definition has no executor")

			err = w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Operation definition has no executor")
			if err != nil {
				return fmt.Errorf("Error appending operation event to execution history: %w", err)
			}

			w.evictOperation(ctx, loopLogger, op)
			reportOperationEvicted(w.schedulerName, op.DefinitionName, LabelNoOperationExecutorFound)
			continue
		}

		if !def.ShouldExecute(ctx, []*operation.Operation{}) {
			w.evictOperation(ctx, loopLogger, op)
			reportOperationEvicted(w.schedulerName, op.DefinitionName, LabelShouldNotExecute)
			continue
		}

		loopLogger.Info("Starting operation")

		err = w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Starting operation")
		if err != nil {
			return fmt.Errorf("Error appending operation event to execution history: %w", err)
		}

		operationContext, operationCancellationFunction := context.WithCancel(ctx)
		err = w.operationManager.GrantLease(operationContext, op)
		if err != nil {
			w.Stop(ctx)
			reportOperationExecutionWorkerFailed(w.schedulerName, LabelStartOperationFailed)
			operationCancellationFunction()

			err = w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Failed to grant lease to operation")
			if err != nil {
				return fmt.Errorf("Error appending operation event to execution history: %w", err)
			}

			op.Status = operation.StatusError
			err = w.operationManager.FinishOperation(ctx, op)
			if err != nil {
				loopLogger.Error("failed to finish operation", zap.Error(err))
			}

			return fmt.Errorf("failed to grant lease to operation \"%s\" for the scheduler \"%s\"", op.ID, op.SchedulerName)
		}

		err = w.operationManager.StartOperation(operationContext, op, operationCancellationFunction)
		if err != nil {
			w.Stop(ctx)
			operationCancellationFunction()

			err = w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Failed to finish operation")
			if err != nil {
				return fmt.Errorf("Error appending operation event to execution history: %w", err)
			}

			op.Status = operation.StatusError
			err = w.operationManager.FinishOperation(ctx, op)
			if err != nil {
				loopLogger.Error("failed to finish operation", zap.Error(err))
			}

			reportOperationExecutionWorkerFailed(w.schedulerName, LabelStartOperationFailed)
			return fmt.Errorf("failed to start operation \"%s\" for the scheduler \"%s\"", op.ID, op.SchedulerName)
		}
		w.operationManager.StartLeaseRenewGoRoutine(operationContext, op)

		executeStartTime := time.Now()
		executionErr := executor.Execute(operationContext, op, def)
		reportOperationExecutionLatency(executeStartTime, w.schedulerName, op.DefinitionName, executionErr != nil)

		op.Status = operation.StatusFinished
		if executionErr != nil {
			op.Status = operation.StatusError
			if errors.Is(executionErr, context.Canceled) {
				op.Status = operation.StatusCanceled
			}

			err = w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Operation execution failed")
			if err != nil {
				return fmt.Errorf("Error appending operation event to execution history: %w", err)
			}

			loopLogger.Error("operation execution failed", zap.Error(executionErr))

			onErrorStartTime := time.Now()
			onErrorErr := executor.OnError(operationContext, op, def, executionErr)
			reportOperationOnErrorLatency(onErrorStartTime, w.schedulerName, op.DefinitionName, onErrorErr != nil)

			if onErrorErr != nil {
				loopLogger.Error("operation OnError failed", zap.Error(onErrorErr))
			}
		}

		loopLogger.Info("Finishing operation")

		// TODO(gabrielcorado): we need to propagate the error reason.
		// TODO(gabrielcorado): consider handling the finish operation error.
		err = w.operationManager.FinishOperation(ctx, op)
		if err != nil {
			loopLogger.Error("failed to finish operation", zap.Error(err))
		}
		err = w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Starting operation")
		if err != nil {
			return fmt.Errorf("Error appending operation event to execution history: %w", err)
		}
		err = w.operationManager.RevokeLease(ctx, op)
		if err != nil {
			loopLogger.Error("failed to revoke operation lease", zap.Error(err))
		}
	}
}

func (w *OperationExecutionWorker) Stop(_ context.Context) {
	if w.workerContext == nil {
		return
	}

	w.cancelWorkerContext()
}

func (w *OperationExecutionWorker) IsRunning() bool {
	return w.workerContext != nil && w.workerContext.Err() == nil
}

// TODO(gabrielcorado): consider handling the finish operation error.
func (w *OperationExecutionWorker) evictOperation(ctx context.Context, logger *zap.Logger, op *operation.Operation) {
	logger.Info("operation evicted")
	op.Status = operation.StatusEvicted
	_ = w.operationManager.FinishOperation(ctx, op)
}
