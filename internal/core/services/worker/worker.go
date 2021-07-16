package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/services/operations_registry"
	"go.uber.org/zap"
)

// Worker is the service responsible for implemeting the worker
// responsabilities.
type Worker struct {
	opManager *operation_manager.OperationManager
	// TODO(gabrielcorado): check if we this is the right place to have all
	// executors.
	executors map[string]operations.Executor

	logger *zap.Logger
}

type WorkerOptions struct {
	OperationStorage   ports.OperationStorage
	OperationFlow      ports.OperationFlow
	Executors          []operations.Executor
	OperationsRegistry *operations_registry.Registry
}

func NewWorker(opts *WorkerOptions) *Worker {
	opManager := operation_manager.New(opts.OperationFlow, opts.OperationStorage)
	if opts.OperationsRegistry != nil {
		opManager = operation_manager.NewWithRegistry(opts.OperationFlow, opts.OperationStorage, *opts.OperationsRegistry)
	}

	executors := make(map[string]operations.Executor)
	for _, executor := range opts.Executors {
		// TODO(gabrielcorado): are we going to receive the executor
		// initialized?
		executors[executor.Name()] = executor
	}

	return &Worker{
		opManager: opManager,
		executors: executors,
		logger:    zap.L().With(zap.String("service", "worker")),
	}
}

// ShouldExecuteOperaiton is responsible for starting a loop that will
// constantly look to execute operations, and this loop can be canceled using
// the provided context. NOTE: It is a blocking function.
func (w *Worker) SchedulerOperationsExecutionLoop(ctx context.Context, schedulerName string) error {
	for {
		op, def, err := w.opManager.NextSchedulerOperation(ctx, schedulerName)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}

			return fmt.Errorf("failed to get next operation: %w", err)
		}

		loopLogger := w.logger.With(
			zap.String("scheduler", schedulerName),
			zap.String("operation_id", op.ID),
			zap.String("operation_definition", def.Name()),
		)

		executor, hasExecutor := w.executors[def.Name()]
		if !hasExecutor {
			loopLogger.Warn("operation definition has no executor")
			w.evictOperation(ctx, loopLogger, op)
			continue
		}

		if !def.ShouldExecute(ctx, []*operation.Operation{}) {
			w.evictOperation(ctx, loopLogger, op)
			continue
		}

		err = w.opManager.StartOperation(ctx, op)
		if err != nil {
			// NOTE: currently, we're not treating if the operation exists or
			// not. In this case, when there is error it will be a unexpected
			// error.
			return fmt.Errorf("failed to start operation \"%s\" for the scheduler \"%s\"", op.ID, op.SchedulerName)
		}

		executionErr := executor.Execute(ctx, op, def)
		op.Status = operation.StatusFinished
		if executionErr != nil {
			op.Status = operation.StatusError
			if errors.Is(executionErr, context.Canceled) {
				op.Status = operation.StatusCanceled
			}

			loopLogger.Debug("operation execution failed", zap.Error(executionErr))
			onErrorErr := executor.OnError(ctx, op, def, executionErr)
			if onErrorErr != nil {
				loopLogger.Error("operation OnError failed", zap.Error(onErrorErr))
			}
		}

		// TODO(gabrielcorado): we need to propagate the error reason.
		// TODO(gabrielcorado): consider handling the finish operation error.
		_ = w.opManager.FinishOperation(ctx, op)
	}

	return nil
}

// TODO(gabrielcorado): consider handling the finish operation error.
func (w *Worker) evictOperation(ctx context.Context, logger *zap.Logger, op *operation.Operation) {
	logger.Debug("operation evicted")
	op.Status = operation.StatusEvicted
	_ = w.opManager.FinishOperation(ctx, op)
}
