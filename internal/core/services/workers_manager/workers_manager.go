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

package workers_manager

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/workers"
	"go.uber.org/zap"
)

// configurations paths for the worker
const (
	// Sync period: waiting time window respected by workers in
	// order to control executions
	syncWorkersIntervalPath = "workers.syncInterval"
	// Workers stop timeout: duration that the workers have to stop their
	// execution until the context is canceled.
	workersStopTimeoutDurationPath = "workers.stopTimeoutDuration"
)

// WorkersManager is the default struct of WorkersManager service
type WorkersManager struct {
	builder                    *workers.WorkerBuilder
	configs                    config.Config
	schedulerStorage           ports.SchedulerStorage
	CurrentWorkers             map[string]workers.Worker
	syncWorkersInterval        time.Duration
	WorkerOptions              *workers.WorkerOptions
	workersStopTimeoutDuration time.Duration
	workersWaitGroup           sync.WaitGroup
	logger                     *zap.Logger
}

// NewWorkersManager is the default constructor of WorkersManager
func NewWorkersManager(builder *workers.WorkerBuilder, configs config.Config, schedulerStorage ports.SchedulerStorage, workerOptions *workers.WorkerOptions) *WorkersManager {

	return &WorkersManager{
		builder:                    builder,
		configs:                    configs,
		schedulerStorage:           schedulerStorage,
		CurrentWorkers:             map[string]workers.Worker{},
		syncWorkersInterval:        configs.GetDuration(syncWorkersIntervalPath),
		WorkerOptions:              workerOptions,
		workersStopTimeoutDuration: configs.GetDuration(workersStopTimeoutDurationPath),
		logger:                     zap.L().With(zap.String(logs.LogFieldComponent, "service"), zap.String(logs.LogFieldServiceName, "workers_manager")),
	}
}

// Start is a function to run a first sync and start a periodically sync worker. This
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
		reportWorkerStop(name, w.builder.ComponentName)
	}

	w.workersWaitGroup.Wait()
}

// SyncWorkers is responsible to run a single sync on operation workers. It will:
// - Get all schedulers
// - Discover and start all desirable workers (not running);
// - Discover and stop all dispensable workers (running);
func (w *WorkersManager) SyncWorkers(ctx context.Context) error {

	schedulers, err := w.schedulerStorage.GetAllSchedulers(ctx)
	if err != nil {
		return err
	}

	desirableWorkers := w.getDesirableWorkers(schedulers)
	for name, worker := range desirableWorkers {
		w.startWorker(ctx, name, worker)
		w.logger.Info("new operation worker running", zap.String("scheduler", name))
		reportWorkerStart(name, w.builder.ComponentName)
	}

	dispensableWorkers := w.getDispensableWorkers(schedulers)
	for name, worker := range dispensableWorkers {
		worker.Stop(ctx)
		w.logger.Info("canceling operation worker", zap.String("scheduler", name))
		reportWorkerStop(name, w.builder.ComponentName)
	}

	reportWorkersSynced()
	return nil
}

func (w *WorkersManager) startWorker(ctx context.Context, name string, wkr workers.Worker) {
	w.workersWaitGroup.Add(1)
	w.CurrentWorkers[name] = wkr
	go func() {
		err := wkr.Start(ctx)
		if err != nil {
			reportWorkerStop(name, w.builder.ComponentName)
			w.logger.
				With(zap.Error(err)).
				With(zap.String("scheduler", name)).
				Error("operation worker failed to start")
		}
		delete(w.CurrentWorkers, name)
		w.workersWaitGroup.Done()
	}()
}

// Gets all desirable operation workers, the ones that are not running
func (w *WorkersManager) getDesirableWorkers(schedulers []*entities.Scheduler) map[string]workers.Worker {

	desirableWorkers := map[string]workers.Worker{}
	for _, scheduler := range schedulers {
		desirableWorkers[scheduler.Name] = w.builder.Func(scheduler, w.WorkerOptions)
	}

	for k := range w.CurrentWorkers {
		delete(desirableWorkers, k)
	}

	return desirableWorkers

}

// Gets all dispensable operation workers, the ones that are running but no more required
func (w *WorkersManager) getDispensableWorkers(schedulers []*entities.Scheduler) map[string]workers.Worker {

	dispensableWorkers := map[string]workers.Worker{}
	for name, worker := range w.CurrentWorkers {
		dispensableWorkers[name] = worker
	}

	for _, scheduler := range schedulers {
		delete(dispensableWorkers, scheduler.Name)
	}

	return dispensableWorkers

}
