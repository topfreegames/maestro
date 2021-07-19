package workers

import (
	"context"
	"time"

	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"go.uber.org/zap"
)

// configurations paths for the worker
const (
	// Sync period: waiting time window respected by workers in
	// order to control executions
	OperationWorkerIntervalPath = "operation.worker.interval"
)

// Operation worker aims to process all pending scheduler operations
type OperationWorker struct {
	run              bool
	syncPeriod       int
	scheduler        *entities.Scheduler
	operationManager operation_manager.OperationManager
}

// Default constructor of OperationWorker
func NewOperationWorker(
	scheduler *entities.Scheduler,
	configs config.Config,
	operationManager operation_manager.OperationManager,
) *OperationWorker {
	return &OperationWorker{
		run:              false,
		scheduler:        scheduler,
		operationManager: operationManager,
		syncPeriod:       configs.GetInt(OperationWorkerIntervalPath),
	}
}

// Start aims to execute periodically the next operation of a scheduler
func (w *OperationWorker) Start(ctx context.Context) error {

	w.run = true

	ticker := time.NewTicker(time.Duration(w.syncPeriod) * time.Second)
	defer ticker.Stop()

	for w.run == true {
		select {
		case <-ticker.C:
			zap.L().Info("Running operation worker", zap.String("scheduler_name", w.scheduler.Name))

		case <-ctx.Done():
			w.Stop(ctx)
			err := ctx.Err()
			if err != nil {
				zap.L().Error("loop to sync operation workers received an error context event", zap.Error(err))
			}
		}
	}

	return nil
}

// Stop aims to set the run attribute as false what stops the loop
func (w *OperationWorker) Stop(ctx context.Context) {
	w.run = false
	return
}

// Returns the property `run` used to control the exeuction loop
func (w *OperationWorker) IsRunning(ctx context.Context) bool {
	return w.run
}
