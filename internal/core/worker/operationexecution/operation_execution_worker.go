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

package operationexecution

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/m-lab/go/memoryless"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"
	"github.com/topfreegames/maestro/internal/core/operations/storagecleanup"
	workererrors "github.com/topfreegames/maestro/internal/core/worker/errors"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/worker"
	"go.uber.org/zap"
)

var _ worker.Worker = (*OperationExecutionWorker)(nil)

const (
	WorkerName = "operation_execution"
	// Size of channel storing operations to be aborted
	OperationsToBeAbortedChannelSize = 100
)

// OperationExecutionWorker is the service responsible for implementing the worker
// responsibilities.
type OperationExecutionWorker struct {
	scheduler                         *entities.Scheduler
	healthControllerExecutionInterval time.Duration
	storagecleanupExecutionInterval   time.Duration
	workersStopTimeoutDuration        time.Duration
	operationManager                  ports.OperationManager
	// TODO(gabrielcorado): check if we this is the right place to have all
	// executors.
	executorsByName         map[string]operations.Executor
	workerContext           context.Context
	cancelWorkerContext     context.CancelFunc
	operationsToAbort       chan OperationExecutionInstance
	isStopping              *atomic.Bool
	abortingOperationsGroup *sync.WaitGroup
	addRoomsLimit           int

	logger *zap.Logger
}

type OperationExecutionInstance struct {
	op       *operation.Operation
	def      operations.Definition
	executor operations.Executor
}

// NewOperationExecutionWorker instantiate a new OperationExecutionWorker to a specified scheduler.
func NewOperationExecutionWorker(scheduler *entities.Scheduler, opts *worker.WorkerOptions) worker.Worker {
	return &OperationExecutionWorker{
		addRoomsLimit:                     opts.Configuration.AddRoomsLimit,
		healthControllerExecutionInterval: opts.Configuration.HealthControllerExecutionInterval,
		storagecleanupExecutionInterval:   opts.Configuration.StorageCleanupExecutionInterval,
		operationManager:                  opts.OperationManager,
		executorsByName:                   opts.OperationExecutors,
		scheduler:                         scheduler,
		logger:                            zap.L().With(zap.String(logs.LogFieldServiceName, WorkerName), zap.String(logs.LogFieldSchedulerName, scheduler.Name)),
		operationsToAbort:                 make(chan OperationExecutionInstance, OperationsToBeAbortedChannelSize),
		isStopping:                        &atomic.Bool{},
		abortingOperationsGroup:           &sync.WaitGroup{},
		workersStopTimeoutDuration:        opts.Configuration.WorkersStopTimeoutDuration,
	}
}

// Start is responsible for starting a loop that will
// constantly look to execute operations, and this loop can be canceled using
// the provided context. NOTE: It is a blocking function.
func (w *OperationExecutionWorker) Start(ctx context.Context) error {
	w.workerContext, w.cancelWorkerContext = context.WithCancel(ctx)
	defer w.Stop(ctx)

	pendingOpsChan := w.operationManager.PendingOperationsChan(w.workerContext, w.scheduler.Name)

	healthControllerTicker := time.NewTicker(w.healthControllerExecutionInterval)
	defer healthControllerTicker.Stop()

	storagecleanupTicker, err := memoryless.NewTicker(context.Background(), memoryless.Config{
		Expected: w.storagecleanupExecutionInterval,
		// This minimum value was took from the memoryless documentation https://pkg.go.dev/github.com/m-lab/go@v0.1.53/memoryless#Ticker
		Min: time.Duration(0.1 * float64(w.storagecleanupExecutionInterval)),
		// This maximum value was took from the memoryless documentation https://pkg.go.dev/github.com/m-lab/go@v0.1.53/memoryless#Ticker
		Max: time.Duration(2.5 * float64(w.storagecleanupExecutionInterval)),
	})
	if err != nil {
		w.logger.Error("Error creating ticker to storage clean-up operation", zap.Error(err))
		return fmt.Errorf("Error creating ticker to storage clean-up operation: %w", err)
	}

	defer storagecleanupTicker.Stop()

	defer close(w.operationsToAbort)
	for {
		select {
		case <-w.workerContext.Done():
			return nil
		case opID, ok := <-pendingOpsChan:
			if !ok {
				reportOperationExecutionWorkerFailed(w.scheduler.Game, w.scheduler.Name, LabelNextOperationFailed)
				return fmt.Errorf("failed to get next operation, channel closed")
			}

			err := w.executeOperationFlow(opID)
			if err != nil {
				w.logger.Error("Error executing operation", zap.Error(err))
			}
		case <-healthControllerTicker.C:
			err := w.createOperation(w.workerContext, &healthcontroller.Definition{})
			if err != nil {
				w.logger.Error("Error enqueueing new health_controller operation", zap.Error(err))
			}
		case <-storagecleanupTicker.C:
			err := w.createOperation(w.workerContext, &storagecleanup.Definition{})
			if err != nil {
				w.logger.Error("Error enqueueing new storage clean up operation", zap.Error(err))
			}
		}
	}
}

func (w *OperationExecutionWorker) executeOperationFlow(operationID string) error {
	op, def, err := w.operationManager.GetOperation(w.workerContext, w.scheduler.Name, operationID)
	if err != nil {
		return err
	}

	loopLogger := w.logger.With(
		zap.String(logs.LogFieldOperationID, op.ID),
		zap.String(logs.LogFieldOperationDefinition, def.Name()),
	)

	if op.Status != operation.StatusPending {
		loopLogger.Warn("operation is at an invalid status to proceed")

		return workererrors.NewErrOperationWithInvalidStatus("operation is at an invalid status to proceed")
	}

	executor, hasExecutor := w.executorsByName[def.Name()]
	if w.shouldEvictOperation(op, def, hasExecutor, loopLogger) {
		return nil
	}

	operationContext, operationCancellationFunction, err := w.prepareExecutionAndLease(op, def, loopLogger)
	defer operationCancellationFunction()

	if err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- w.executeOperationWithLease(operationContext, op, def, executor)
	}()
	select {
	case <-w.workerContext.Done():
		w.operationsToAbort <- OperationExecutionInstance{op, def, executor}
	case executionErr := <-done:
		if executionErr != nil {
			w.handleExecutionError(op, def, executionErr, loopLogger, executor)
		} else {
			op.Status = operation.StatusFinished
			w.finishOperationAndLease(w.workerContext, op, def, loopLogger)
		}
	}

	return nil
}

func (w *OperationExecutionWorker) rollbackOperation(ctx context.Context, op *operation.Operation, def operations.Definition, executionErr error, loopLogger *zap.Logger, executor operations.Executor) {
	loopLogger.Info("rolling back operation")
	w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Starting operation rollback")
	rollbackErr := w.executeRollbackCollectingLatencyMetrics(op.DefinitionName, func() error {
		return executor.Rollback(ctx, op, def, executionErr)
	})

	if rollbackErr != nil {
		loopLogger.Error("operation rollback failed", zap.Error(rollbackErr))
		w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, fmt.Sprintf("Operation rollback flow execution failed, reason: %s", rollbackErr.Error()))
	} else {
		loopLogger.Info("successfully rolled back operation")
		w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Operation rollback flow execution finished with success")
	}
}

func (w *OperationExecutionWorker) handleExecutionError(op *operation.Operation, def operations.Definition, executionErr error, loopLogger *zap.Logger, executor operations.Executor) {
	op.Status = operation.StatusError
	msg := "operation execution failed"
	loopLogger.Error(msg, zap.Error(executionErr))
	w.operationManager.AppendOperationEventToExecutionHistory(w.workerContext, op, msg)
	w.rollbackOperation(w.workerContext, op, def, executionErr, loopLogger, executor)
	w.finishOperationAndLease(w.workerContext, op, def, loopLogger)
}

func (w *OperationExecutionWorker) finishOperationAndLease(ctx context.Context, op *operation.Operation, def operations.Definition, loopLogger *zap.Logger) {
	// TODO(gabrielcorado): we need to propagate the error reason.
	// TODO(gabrielcorado): consider handling the finish operation error.
	err := w.operationManager.FinishOperation(ctx, op, def)
	if err != nil {
		loopLogger.Error("failed to finish operation", zap.Error(err))
	}
	err = w.operationManager.RevokeLease(ctx, op)
	if err != nil {
		loopLogger.Error("failed to revoke operation lease", zap.Error(err))
	}
	w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Operation finished")
}

func (w OperationExecutionWorker) executeOperationWithLease(operationContext context.Context, op *operation.Operation, def operations.Definition, executor operations.Executor) error {
	return w.executeCollectingLatencyMetrics(op.DefinitionName, func() error {
		return executor.Execute(operationContext, op, def)
	})
}

func (w *OperationExecutionWorker) prepareExecutionAndLease(op *operation.Operation, def operations.Definition, loopLogger *zap.Logger) (operationContext context.Context, operationCancellationFunction context.CancelFunc, err error) {
	loopLogger.Debug("Starting operation")

	w.operationManager.AppendOperationEventToExecutionHistory(w.workerContext, op, "Starting operation")

	operationContext, operationCancellationFunction = context.WithCancel(w.workerContext)
	err = w.operationManager.GrantLease(w.workerContext, op)
	if err != nil {
		reportOperationExecutionWorkerFailed(w.scheduler.Game, w.scheduler.Name, LabelStartOperationFailed)
		operationCancellationFunction()

		op.Status = operation.StatusError
		w.operationManager.AppendOperationEventToExecutionHistory(w.workerContext, op, fmt.Sprintf("Failed to grant lease to operation, reason: %s", err.Error()))

		err = w.operationManager.FinishOperation(w.workerContext, op, def)
		if err != nil {
			loopLogger.Error("failed to finish operation", zap.Error(err))
		}

		return operationContext, operationCancellationFunction, workererrors.NewErrGrantLeaseFailed("failed to grant lease to operation \"%s\" for the scheduler \"%s\"", op.ID, op.SchedulerName)
	}

	err = w.operationManager.StartOperation(operationContext, op, operationCancellationFunction)
	if err != nil {
		operationCancellationFunction()

		op.Status = operation.StatusError
		w.operationManager.AppendOperationEventToExecutionHistory(w.workerContext, op, fmt.Sprintf("Failed to start operation, reason: %s", err.Error()))
		err = w.operationManager.FinishOperation(w.workerContext, op, def)
		if err != nil {
			loopLogger.Error("failed to start operation", zap.Error(err))
		}

		reportOperationExecutionWorkerFailed(w.scheduler.Game, w.scheduler.Name, LabelStartOperationFailed)

		return operationContext, operationCancellationFunction, workererrors.NewErrStartOperationFailed("failed to start operation \"%s\" for the scheduler \"%s\"", op.ID, op.SchedulerName)
	}

	w.operationManager.StartLeaseRenewGoRoutine(operationContext, op)

	return operationContext, operationCancellationFunction, err
}

func (w *OperationExecutionWorker) shouldEvictOperation(op *operation.Operation, def operations.Definition, hasExecutor bool, loopLogger *zap.Logger) bool {
	if !hasExecutor {
		loopLogger.Warn("operation definition has no executor")
		w.evictOperation(w.workerContext, loopLogger, op, def)
		reportOperationEvicted(w.scheduler.Game, w.scheduler.Name, op.DefinitionName, LabelNoOperationExecutorFound)

		return true
	}

	if !def.ShouldExecute(w.workerContext, []*operation.Operation{}) {
		w.evictOperation(w.workerContext, loopLogger, op, def)
		reportOperationEvicted(w.scheduler.Game, w.scheduler.Name, op.DefinitionName, LabelShouldNotExecute)

		return true
	}
	return false
}

func (w *OperationExecutionWorker) createOperation(ctx context.Context, operationDefinition operations.Definition) error {
	_, err := w.operationManager.CreateOperation(ctx, w.scheduler.Name, operationDefinition)
	if err != nil {
		return fmt.Errorf("not able to schedule the '%s' operation: %w", operationDefinition.Name(), err)
	}

	return nil
}

func (w *OperationExecutionWorker) AbortOngoingOperations(ctx context.Context) {
	w.logger.Info("aborting operations that were in progress", zap.Int("operationsToAbort", len(w.operationsToAbort)))
	for {
		instance, ok := <-w.operationsToAbort
		if !ok {
			return
		}
		logger := w.logger.With(
			zap.String(logs.LogFieldOperationID, instance.op.ID),
			zap.String(logs.LogFieldOperationDefinition, instance.def.Name()),
		)
		instance.op.Status = operation.StatusCanceled
		w.operationManager.AppendOperationEventToExecutionHistory(ctx, instance.op, context.Canceled.Error())
		w.rollbackOperation(ctx, instance.op, instance.def, context.Canceled, logger, instance.executor)
		w.finishOperationAndLease(ctx, instance.op, instance.def, logger)
	}
}

func (w *OperationExecutionWorker) Stop(_ context.Context) {
	w.logger.Info("stopping operation execution worker")
	defer w.isStopping.Store(false)
	defer w.cancelWorkerContext()

	if w.isStopping.Load() {
		w.logger.Debug("already aborting operations, waiting for them to finish")
		// Worker can be stopped by multiple callers: WorkerManager and
		// OperationExecutinWorker on the second call, we need to wait all
		// operations to be aborted
		w.abortingOperationsGroup.Wait()
		w.isStopping.Store(false)
		return
	}

	w.isStopping.Store(true)
	w.abortingOperationsGroup.Add(1)
	defer w.abortingOperationsGroup.Done()

	if len(w.operationsToAbort) == 0 {
		w.logger.Info("no operations to abort, worker stopping")
		return
	}

	w.AbortOngoingOperations(context.Background())
	w.logger.Info("operations aborted, worker stopping")
}

func (w *OperationExecutionWorker) IsRunning() bool {
	return w.workerContext != nil && w.workerContext.Err() == nil
}

// TODO(gabrielcorado): consider handling the finish operation error.
func (w *OperationExecutionWorker) evictOperation(ctx context.Context, logger *zap.Logger, op *operation.Operation, def operations.Definition) {
	logger.Info("operation evicted")
	op.Status = operation.StatusEvicted
	_ = w.operationManager.FinishOperation(ctx, op, def)
	w.operationManager.AppendOperationEventToExecutionHistory(ctx, op, "Operation evicted")
}

func (w *OperationExecutionWorker) executeCollectingLatencyMetrics(definitionName string, f func() error) (err error) {
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
