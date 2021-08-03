package operation_execution_worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
	"go.uber.org/zap"
)

var _ workers.Worker = (*OperationExecutionWorker)(nil)

// Worker is the service responsible for implemeting the worker
// responsabilities.
type OperationExecutionWorker struct {
	schedulerName    string
	operationManager *operation_manager.OperationManager
	// TODO(gabrielcorado): check if we this is the right place to have all
	// executors.
	executorsByName     map[string]operations.Executor
	workerContext       context.Context
	cancelWorkerContext context.CancelFunc

	logger *zap.Logger
}

func NewOperationExecutionWorker(scheduler *entities.Scheduler, opts *workers.WorkerOptions) workers.Worker {
	executors := make(map[string]operations.Executor)
	for _, executor := range opts.Executors {
		// TODO(gabrielcorado): are we going to receive the executor
		// initialized?
		executors[executor.Name()] = executor
	}

	return &OperationExecutionWorker{
		operationManager: opts.OperationManager,
		executorsByName:  executors,
		schedulerName:    scheduler.Name,
		logger:           zap.L().With(zap.String("service", "worker"), zap.String("scheduler_name", scheduler.Name)),
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

		// TODO(gabrielcorado): when we introduce operation cancelation this is
		// the one to be cancelled. Right now it is only a placeholder.
		operationContext := context.Background()

		loopLogger := w.logger.With(
			zap.String("operation_id", op.ID),
			zap.String("operation_definition", def.Name()),
		)

		executor, hasExecutor := w.executorsByName[def.Name()]
		if !hasExecutor {
			loopLogger.Warn("operation definition has no executor")
			w.evictOperation(operationContext, loopLogger, op)
			reportOperationEvicted(w.schedulerName, op.DefinitionName, LabelNoOperationExecutorFound)
			continue
		}

		if !def.ShouldExecute(operationContext, []*operation.Operation{}) {
			w.evictOperation(operationContext, loopLogger, op)
			reportOperationEvicted(w.schedulerName, op.DefinitionName, LabelShouldNotExecute)
			continue
		}

		err = w.operationManager.StartOperation(operationContext, op)
		if err != nil {
			w.Stop(ctx)

			// NOTE: currently, we're not treating if the operation exists or
			// not. In this case, when there is error it will be a unexpected
			// error.
			reportOperationExecutionWorkerFailed(w.schedulerName, LabelStartOperationFailed)
			return fmt.Errorf("failed to start operation \"%s\" for the scheduler \"%s\"", op.ID, op.SchedulerName)
		}

		executeStartTime := time.Now()
		executionErr := executor.Execute(operationContext, op, def)
		reportOperationExecutionLatency(executeStartTime, w.schedulerName, op.DefinitionName, executionErr != nil)

		op.Status = operation.StatusFinished
		if executionErr != nil {
			op.Status = operation.StatusError
			if errors.Is(executionErr, context.Canceled) {
				op.Status = operation.StatusCanceled
			}

			loopLogger.Debug("operation execution failed", zap.Error(executionErr))

			onErrorStartTime := time.Now()
			onErrorErr := executor.OnError(operationContext, op, def, executionErr)
			reportOperationOnErrorLatency(onErrorStartTime, w.schedulerName, op.DefinitionName, onErrorErr != nil)

			if onErrorErr != nil {
				loopLogger.Error("operation OnError failed", zap.Error(onErrorErr))
			}
		}

		// TODO(gabrielcorado): we need to propagate the error reason.
		// TODO(gabrielcorado): consider handling the finish operation error.
		_ = w.operationManager.FinishOperation(operationContext, op)
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
	logger.Debug("operation evicted")
	op.Status = operation.StatusEvicted
	_ = w.operationManager.FinishOperation(ctx, op)
}
