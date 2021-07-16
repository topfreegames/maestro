package workers_manager

import (
	"context"
	"os"
	"os/signal"
	"syscall"
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
	syncOperationWorkersIntervalPath = "check.operation.workers.interval"
)

type WorkersManager struct {
	configs                      config.Config
	schedulerStorage             ports.SchedulerStorage
	operationManager             operation_manager.OperationManager
	CurrentWorkers               map[string]workers.Worker
	runSyncOperationWorkers      bool
	syncOperationWorkersInterval int
}

func NewWorkersManager(configs config.Config, schedulerStorage ports.SchedulerStorage, operationManager operation_manager.OperationManager) *WorkersManager {
	return &WorkersManager{
		configs:                      configs,
		schedulerStorage:             schedulerStorage,
		operationManager:             operationManager,
		CurrentWorkers:               map[string]workers.Worker{},
		syncOperationWorkersInterval: configs.GetInt(syncOperationWorkersIntervalPath),
	}
}

func (w *WorkersManager) Start(ctx context.Context) error {

	err := w.SyncOperationWorkers(ctx)
	if err != nil {
		zap.L().Error("initial sync operation workers failed", zap.Error(err))
	}

	go w.startSyncOperationWorkers(ctx)

	return nil
}

func (w *WorkersManager) startSyncOperationWorkers(ctx context.Context) error {

	w.runSyncOperationWorkers = true
	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(w.syncOperationWorkersInterval) * time.Second)
	defer ticker.Stop()

	for w.runSyncOperationWorkers == true {
		select {
		case <-ticker.C:
			err := w.SyncOperationWorkers(ctx)
			if err != nil {
				zap.L().Error("scheduled sync operation workers failed", zap.Error(err))
			}

		case sig := <-sigchan:
			zap.L().Warn("caught signal: terminating\n", zap.String("signal", sig.String()))
			w.runSyncOperationWorkers = false
		}
	}

	return nil

}

func (w *WorkersManager) SyncOperationWorkers(ctx context.Context) error {

	zap.L().Info("starting to sync operation workers")

	schedulers, err := w.schedulerStorage.GetAllSchedulers(ctx)
	if err != nil {
		return err
	}

	zap.L().Info("schedulers found, syncing operation workers", zap.Int("count", len(schedulers)))

	for name, worker := range w.getDesirableOperationWorkers(ctx, schedulers) {
		go worker.Start(ctx)
		w.CurrentWorkers[name] = worker
		zap.L().Info("new operation worker running", zap.Int("scheduler", len(name)))
	}

	for name, worker := range w.getUnecessaryOperationWorkers(ctx, schedulers) {
		worker.Stop(ctx)
		delete(w.CurrentWorkers, name)
		zap.L().Info("canceling operation worker", zap.Int("scheduler", len(name)))
	}

	zap.L().Info("all operation workers in sync")

	return nil
}

func (w *WorkersManager) getDesirableOperationWorkers(ctx context.Context, schedulers []*entities.Scheduler) map[string]workers.Worker {

	desirableWorkers := map[string]workers.Worker{}
	for _, scheduler := range schedulers {
		desirableWorkers[scheduler.Name] = workers.NewOperationWorker(scheduler, w.configs, w.operationManager)
	}

	for k := range w.CurrentWorkers {
		delete(desirableWorkers, k)
	}

	return desirableWorkers

}

func (w *WorkersManager) getUnecessaryOperationWorkers(ctx context.Context, schedulers []*entities.Scheduler) map[string]workers.Worker {

	unecessaryWorkers := map[string]workers.Worker{}
	for name, worker := range w.CurrentWorkers {
		unecessaryWorkers[name] = worker
	}

	for _, scheduler := range schedulers {
		delete(unecessaryWorkers, scheduler.Name)
	}

	return unecessaryWorkers

}
