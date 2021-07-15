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

	w.startSchedulerOperationWorkers(ctx)
	go w.syncOperationWorkers(ctx)

	return nil
}

func (w *WorkersManager) startSchedulerOperationWorkers(ctx context.Context) error {

	zap.L().Info("starting scheduler operation workers")

	schedulers, err := w.schedulerStorage.GetAllSchedulers(ctx)
	if err != nil {
		return err
	}

	zap.L().Info("schedulers found, starting operation workers", zap.Int("count", len(schedulers)))

	for _, scheduler := range schedulers {
		worker := workers.NewOperationWorker(scheduler, w.configs, w.operationManager)
		w.CurrentWorkers[scheduler.Name] = worker
		go worker.Start(ctx)
	}

	zap.L().Info("operation workers running", zap.Int("count", len(schedulers)))

	return nil
}

func (w *WorkersManager) syncOperationWorkers(ctx context.Context) error {

	w.runSyncOperationWorkers = true
	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(w.syncOperationWorkersInterval) * time.Second)
	defer ticker.Stop()

	for w.runSyncOperationWorkers == true {
		select {
		case <-ticker.C:
			zap.L().Info("Checking operation workers")

			schedulers, err := w.schedulerStorage.GetAllSchedulers(ctx)
			if err != nil {
				return err
			}

			for name, worker := range w.getDesirableOperationWorkers(ctx, schedulers) {
				go worker.Start(ctx)
				w.CurrentWorkers[name] = worker
			}

			for name, worker := range w.getUnecessaryOperationWorkers(ctx, schedulers) {
				worker.Stop(ctx)
				delete(w.CurrentWorkers, name)
			}

		case sig := <-sigchan:
			zap.L().Warn("caught signal: terminating\n", zap.String("signal", sig.String()))
			w.runSyncOperationWorkers = false
		}
	}

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

	unecessaryWorkers := w.CurrentWorkers
	for _, scheduler := range schedulers {
		delete(unecessaryWorkers, scheduler.Name)
	}

	return unecessaryWorkers

}
