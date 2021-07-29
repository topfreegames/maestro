package workers_manager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
	"go.uber.org/zap"
)

// configurations paths for the worker
const (
	// Sync period: waiting time window respected by workers in
	// order to control executions
	syncWorkersIntervalPath = "workers.syncInterval"
	// Workers stop timeout: duration that the workes have to stop their
	// execution until the context is canceled.
	workersStopTimeoutDurationPath = "workers.stopTimeoutDuration"
)

// Default struct of WorkersManager service
type WorkersManager struct {
	builder                    workers.WorkerBuilder
	configs                    config.Config
	schedulerStorage           ports.SchedulerStorage
	operationManager           *operation_manager.OperationManager
	CurrentWorkers             map[string]workers.Worker
	syncWorkersInterval        time.Duration
	workerOptions              *workers.WorkerOptions
	workersStopTimeoutDuration time.Duration
}

// Default constructor of WorkersManager
func NewWorkersManager(builder workers.WorkerBuilder, configs config.Config, schedulerStorage ports.SchedulerStorage, operationManager *operation_manager.OperationManager) *WorkersManager {
	// options passed to build the workers.
	// NOTE: we might need to add more options as the workers need.
	workerOptions := &workers.WorkerOptions{
		OperationManager: operationManager,
	}

	return &WorkersManager{
		builder:                    builder,
		configs:                    configs,
		schedulerStorage:           schedulerStorage,
		operationManager:           operationManager,
		CurrentWorkers:             map[string]workers.Worker{},
		syncWorkersInterval:        configs.GetDuration(syncWorkersIntervalPath),
		workerOptions:              workerOptions,
		workersStopTimeoutDuration: configs.GetDuration(workersStopTimeoutDurationPath),
	}
}

// Function to run a first sync and start a periodically sync worker. This
// function blocks and returns an error if it happens during the
// execution. It is cancellable through the provided context.
func (w *WorkersManager) Start(ctx context.Context) error {
	err := w.SyncWorkers(ctx)
	if err != nil {
		return fmt.Errorf("initial sync operation workers failed: %w", err)
	}

	ticker := time.NewTicker(w.syncWorkersInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := w.SyncWorkers(ctx); err != nil {
				w.stop()
				return fmt.Errorf("loop to sync operation workers failed: %w", err)
			}
		case <-ctx.Done():
			w.stop()
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}

			return fmt.Errorf("loop to sync operation workers received an error context event: %w", err)
		}
	}
}

// Stops all registered workers.
func (w *WorkersManager) stop() {
	// create a context with timeout to stop the workers.
	ctx, cancelFn := context.WithTimeout(context.Background(), w.workersStopTimeoutDuration)
	defer cancelFn()

	for name, worker := range w.CurrentWorkers {
		worker.Stop(ctx)
		delete(w.CurrentWorkers, name)
		reportWorkerStop(name)
	}
}

// Function responsible to run a single sync on operation workers. It will:
// - Get all schedulers
// - Discover and start all desirable workers (not running);
// - Discover and stop all dispensable workers (running);
func (w *WorkersManager) SyncWorkers(ctx context.Context) error {
	zap.L().Info("starting to sync operation workers")

	schedulers, err := w.schedulerStorage.GetAllSchedulers(ctx)
	if err != nil {
		return err
	}

	for name, worker := range w.getDesirableWorkers(ctx, schedulers) {
		startWorker(ctx, name, worker)
		w.CurrentWorkers[name] = worker
		zap.L().Info("new operation worker running", zap.Int("scheduler", len(name)))
		reportWorkerStart(name)
	}

	for name, worker := range w.getDispensableWorkers(ctx, schedulers) {
		worker.Stop(ctx)
		delete(w.CurrentWorkers, name)
		zap.L().Info("canceling operation worker", zap.Int("scheduler", len(name)))
		reportWorkerStop(name)
	}

	for name, worker := range w.getDeadWorkers(ctx) {
		startWorker(ctx, name, worker)
		w.CurrentWorkers[name] = worker
		zap.L().Info("restarting dead operation worker", zap.Int("scheduler", len(name)))
		reportWorkerRestart(name)
	}

	reportWorkersSynced()
	return nil
}

func startWorker(ctx context.Context, name string, wkr workers.Worker) {
	go func() {
		err := wkr.Start(ctx)
		if err != nil {
			zap.L().With(zap.Error(err)).Error("operation worker failed to start", zap.Int("scheduler", len(name)))
		}
	}()
}

// Gets all desirable operation workers, the ones that are not running
func (w *WorkersManager) getDesirableWorkers(ctx context.Context, schedulers []*entities.Scheduler) map[string]workers.Worker {

	desirableWorkers := map[string]workers.Worker{}
	for _, scheduler := range schedulers {
		desirableWorkers[scheduler.Name] = w.builder(scheduler, w.workerOptions)
	}

	for k := range w.CurrentWorkers {
		delete(desirableWorkers, k)
	}

	return desirableWorkers

}

// Gets all dispensable operation workers, the ones that are running but no more required
func (w *WorkersManager) getDispensableWorkers(ctx context.Context, schedulers []*entities.Scheduler) map[string]workers.Worker {

	dispensableWorkers := map[string]workers.Worker{}
	for name, worker := range w.CurrentWorkers {
		dispensableWorkers[name] = worker
	}

	for _, scheduler := range schedulers {
		delete(dispensableWorkers, scheduler.Name)
	}

	return dispensableWorkers

}

// Gets all dead operation workers, the ones that are in currentWorkers but not running
func (w *WorkersManager) getDeadWorkers(ctx context.Context) map[string]workers.Worker {

	deadWorkers := map[string]workers.Worker{}
	for name, worker := range w.CurrentWorkers {
		if !worker.IsRunning(ctx) {
			deadWorkers[name] = worker
		}
	}

	return deadWorkers

}
