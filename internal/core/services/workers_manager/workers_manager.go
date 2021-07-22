package workers_manager

import (
	"context"
	"time"

	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
	"go.uber.org/zap"
)

// configurations paths for the worker
const (
	// Sync period: waiting time window respected by workers in
	// order to control executions
	syncWorkersIntervalPath = "check.workers.interval"
)

// Default struct of WorkersManager service
type WorkersManager struct {
	builder             workers.WorkerBuilder
	configs             config.Config
	schedulerStorage    ports.SchedulerStorage
	operationManager    *operation_manager.OperationManager
	CurrentWorkers      map[string]workers.Worker
	RunSyncWorkers      bool
	syncWorkersInterval int
	workerOptions       *workers.WorkerOptions
}

// Default constructor of WorkersManager
func NewWorkersManager(builder workers.WorkerBuilder, configs config.Config, schedulerStorage ports.SchedulerStorage, operationManager *operation_manager.OperationManager) *WorkersManager {
	// options passed to build the workers.
	// NOTE: we might need to add more options as the workers need.
	workerOptions := &workers.WorkerOptions{
		OperationManager: operationManager,
	}

	return &WorkersManager{
		builder:             builder,
		configs:             configs,
		schedulerStorage:    schedulerStorage,
		operationManager:    operationManager,
		CurrentWorkers:      map[string]workers.Worker{},
		syncWorkersInterval: configs.GetInt(syncWorkersIntervalPath),
		workerOptions:       workerOptions,
	}
}

// Function to run a first sync and start a periodically sync worker
func (w *WorkersManager) Start(ctx context.Context) error {

	err := w.SyncWorkers(ctx)
	if err != nil {
		return errors.NewErrUnexpected("initial sync operation workers failed").WithError(err)
	}

	go w.startSyncWorkers(ctx)

	return nil
}

// Function to start a infinite loop (ticker) that will call
// (periodically) the SyncWorkers func
func (w *WorkersManager) startSyncWorkers(ctx context.Context) {

	w.RunSyncWorkers = true
	ticker := time.NewTicker(time.Duration(w.syncWorkersInterval) * time.Second)
	defer ticker.Stop()

	for w.RunSyncWorkers == true {
		select {
		case <-ticker.C:
			err := w.SyncWorkers(ctx)
			if err != nil {
				w.stop(ctx)
				zap.L().Error("loop to sync operation workers failed", zap.Error(err))
			}

		case <-ctx.Done():
			w.stop(ctx)
			err := ctx.Err()
			if err != nil {
				zap.L().Error("loop to sync operation workers received an error context event", zap.Error(err))
			}
		}
	}

	return
}

// Stops all registered workers and stops sync operation workers loop
func (w *WorkersManager) stop(ctx context.Context) {
	for name, worker := range w.CurrentWorkers {
		worker.Stop(ctx)
		delete(w.CurrentWorkers, name)
		reportWorkerStop(name)
	}
	w.RunSyncWorkers = false
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
		go worker.Start(ctx)
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
		worker.Start(ctx)
		w.CurrentWorkers[name] = worker
		zap.L().Info("restarting dead operation worker", zap.Int("scheduler", len(name)))
		reportWorkerRestart(name)
	}

	reportWorkersSynced()
	return nil
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
		if worker.IsRunning(ctx) == false {
			deadWorkers[name] = worker
		}
	}

	return deadWorkers

}
