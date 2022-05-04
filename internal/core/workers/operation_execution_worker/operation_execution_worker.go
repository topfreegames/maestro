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
	workererrors "github.com/topfreegames/maestro/internal/core/workers/errors"

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
	scheduler        *entities.Scheduler
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
		scheduler:        scheduler,
		logger:           zap.L().With(zap.String(logs.LogFieldServiceName, "worker"), zap.String(logs.LogFieldSchedulerName, scheduler.Name)),
	}
}

// Start is responsible for starting a loop that will
// constantly look to execute operations, and this loop can be canceled using
// the provided context. NOTE: It is a blocking function.
func (w *OperationExecutionWorker) Start(ctx context.Context) error {
	defer w.Stop(ctx)

	w.workerContext, w.cancelWorkerContext = context.WithCancel(ctx)

	for {
		op, def, err := w.operationManager.NextSchedulerOperation(w.workerContext, w.scheduler.Name)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			reportOperationExecutionWorkerFailed(w.scheduler.Game, w.scheduler.Name, LabelNextOperationFailed)
			return fmt.Errorf("failed to get next operation: %w", err)
		}

		loopLogger := w.logger.With(
			zap.String(logs.LogFieldOperationID, op.ID),
			zap.String(logs.LogFieldOperationDefinition, def.Name()),
		)

		err = w.executeOperation(ctx, op, loopLogger, def)
		if err != nil {
			switch {
			case errors.Is(err, workererrors.ErrStartOperationFailed) ||
				errors.Is(err, workererrors.ErrGrantLeaseFailed) ||
				errors.Is(err, workererrors.ErrOperationWithInvalidStatus):
				return err
			default:
				continue
			}
		}

	}
}

func (w *OperationExecutionWorker) executeOperation(ctx context.Context, op *operation.Operation, loopLogger *zap.Logger, def operations.Definition) error {
	if op.Status != operation.StatusPending {
		loopLogger.Warn("operation is at an invalid status to proceed")

		return workererrors.NewErrOperationWithInvalidStatus("operation is at and invalid status to proceed")
	}

	executor, hasExecutor := w.executorsByName[def.Name()]
	if !hasExecutor {
		loopLogger.Warn("operation definition has no executor")

		w.evictOperation(ctx, loopLogger, op)
		reportOperationEvicted(w.scheduler.Game, w.scheduler.Name, op.DefinitionName, LabelNoOperationExecutorFound)

		return nil
	}

	if !def.ShouldExecute(ctx, []*operation.Operation{}) {
		w.evictOperation(ctx, loopLogger, op)
		reportOperationEvicted(w.scheduler.Game, w.scheduler.Name, op.DefinitionName, LabelShouldNotExecute)

		return nil
	}

	loopLogger.Info("Starting operation")

	w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Starting operation")

	operationContext, operationCancellationFunction := context.WithCancel(ctx)
	err := w.operationManager.GrantLease(operationContext, op)
	if err != nil {
		reportOperationExecutionWorkerFailed(w.scheduler.Game, w.scheduler.Name, LabelStartOperationFailed)
		operationCancellationFunction()

		op.Status = operation.StatusError
		w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, fmt.Sprintf("Failed to grant lease to operation, reason: %s", err.Error()))

		err = w.operationManager.FinishOperation(ctx, op)
		if err != nil {
			loopLogger.Error("failed to finish operation", zap.Error(err))
		}

		return workererrors.NewErrGrantLeaseFailed("failed to grant lease to operation \"%s\" for the scheduler \"%s\"", op.ID, op.SchedulerName)
	}

	err = w.operationManager.StartOperation(operationContext, op, operationCancellationFunction)
	if err != nil {
		operationCancellationFunction()

		op.Status = operation.StatusError
		w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, fmt.Sprintf("Failed to start operation, reason: %s", err.Error()))
		err = w.operationManager.FinishOperation(ctx, op)
		if err != nil {
			loopLogger.Error("failed to start operation", zap.Error(err))
		}

		reportOperationExecutionWorkerFailed(w.scheduler.Game, w.scheduler.Name, LabelStartOperationFailed)

		return workererrors.NewErrStartOperationFailed("failed to start operation \"%s\" for the scheduler \"%s\"", op.ID, op.SchedulerName)
	}
	w.operationManager.StartLeaseRenewGoRoutine(operationContext, op)

	executionErr := w.executeCollectingLatencyMetrics(op.DefinitionName, func() operations.ExecutionError {
		return executor.Execute(operationContext, op, def)
	})

	op.Status = operation.StatusFinished
	if executionErr != nil {
		if executionErr.IsContextCanceled() {
			op.Status = operation.StatusCanceled

			loopLogger.Info("operation execution canceled")
			w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Operation canceled by the user")
		} else {
			op.Status = operation.StatusError

			loopLogger.Error("operation execution failed", zap.Error(executionErr.Error()))
			w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, fmt.Sprintf("Operation execution failed : %s", executionErr.FormattedMessage()))
		}

		w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Starting operation rollback")
		rollbackErr := w.executeRollbackCollectingLatencyMetrics(op.DefinitionName, func() error {
			return executor.Rollback(w.workerContext, op, def, executionErr)
		})

		if rollbackErr != nil {
			loopLogger.Error("operation rollback failed", zap.Error(rollbackErr))
			w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, fmt.Sprintf("Operation rollback flow execution failed, reason: %s", rollbackErr.Error()))
		} else {
			w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Operation rollback flow execution finished with success")
		}
	}

	loopLogger.Info("Finishing operation")

	// TODO(gabrielcorado): we need to propagate the error reason.
	// TODO(gabrielcorado): consider handling the finish operation error.
	err = w.operationManager.FinishOperation(ctx, op)
	if err != nil {
		loopLogger.Error("failed to finish operation", zap.Error(err))
	}
	err = w.operationManager.RevokeLease(ctx, op)
	if err != nil {
		loopLogger.Error("failed to revoke operation lease", zap.Error(err))
	}
	w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Operation finished")
	return nil
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
	w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Operation evicted")
}

func (w *OperationExecutionWorker) executeCollectingLatencyMetrics(definitionName string, f func() operations.ExecutionError) (err operations.ExecutionError) {
	executeStartTime := time.Now()
	err = f()
	reportOperationExecutionLatency(executeStartTime, w.scheduler.Game, w.scheduler.Name, definitionName, err == nil)
	return err
}

func (w *OperationExecutionWorker) executeRollbackCollectingLatencyMetrics(definitionName string, f func() error) (err error) {
	rollbackStartTime := time.Now()
	err = f()
	reportOperationRollbackLatency(rollbackStartTime, w.scheduler.Game, w.scheduler.Name, definitionName, err == nil)
	return err
}
