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

package metricsreporter

import (
	"context"
	"time"

	"github.com/topfreegames/maestro/internal/core/worker/config"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/logs"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/worker"
	"go.uber.org/zap"
)

var _ worker.Worker = (*MetricsReporterWorker)(nil)

const WorkerName = "metrics_reporter"

// MetricsReporterWorker is the service responsible producing periodic metrics.
type MetricsReporterWorker struct {
	scheduler           *entities.Scheduler
	schedulerCache      ports.SchedulerCache
	config              *config.MetricsReporterConfig
	roomStorage         ports.RoomStorage
	instanceStorage     ports.GameRoomInstanceStorage
	workerContext       context.Context
	cancelWorkerContext context.CancelFunc
	logger              *zap.Logger
}

func NewMetricsReporterWorker(scheduler *entities.Scheduler, opts *worker.WorkerOptions) worker.Worker {
	return &MetricsReporterWorker{
		scheduler:       scheduler,
		schedulerCache:  opts.SchedulerCache,
		config:          opts.MetricsReporterConfig,
		roomStorage:     opts.RoomStorage,
		instanceStorage: opts.InstanceStorage,
		logger:          zap.L().With(zap.String(logs.LogFieldServiceName, WorkerName), zap.String(logs.LogFieldSchedulerName, scheduler.Name)),
	}
}

// Start is responsible for starting a loop that will
// periodically report metrics for scheduler pods and game rooms.
func (w *MetricsReporterWorker) Start(ctx context.Context) error {
	defer w.Stop(ctx)
	w.workerContext, w.cancelWorkerContext = context.WithCancel(ctx)
	ticker := time.NewTicker(time.Millisecond * w.config.MetricsReporterIntervalMillis)
	defer ticker.Stop()

	for {
		select {
		case <-w.workerContext.Done():
			w.Stop(w.workerContext)
			return nil
		case <-ticker.C:
			w.syncScheduler(w.workerContext)
			w.reportInstanceMetrics()
			w.reportGameRoomMetrics()
			w.reportSchedulerMetrics()
		}
	}
}

func (w *MetricsReporterWorker) Stop(_ context.Context) {
	if w.workerContext == nil {
		return
	}

	w.cancelWorkerContext()
}

func (w *MetricsReporterWorker) IsRunning() bool {
	return w.workerContext != nil && w.workerContext.Err() == nil
}

func (w *MetricsReporterWorker) syncScheduler(ctx context.Context) {
	scheduler, err := w.schedulerCache.GetScheduler(ctx, w.scheduler.Name)
	if err != nil {
		w.logger.Error("Error loading scheduler", zap.Error(err))
		return
	}

	if w.scheduler.Spec.Version != scheduler.Spec.Version {
		w.scheduler = scheduler
	}
}

func (w *MetricsReporterWorker) reportInstanceMetrics() {
	w.logger.Debug("Reporting instance metrics")

	instances, err := w.instanceStorage.GetAllInstances(w.workerContext, w.scheduler.Name)
	if err != nil {
		w.logger.Error("Error getting pods", zap.Error(err))
		// Set all instance metrics to 0 on error
		reportInstanceReadyNumber(w.scheduler.Game, w.scheduler.Name, 0)
		reportInstancePendingNumber(w.scheduler.Game, w.scheduler.Name, 0)
		reportInstanceErrorNumber(w.scheduler.Game, w.scheduler.Name, 0)
		reportInstanceUnknownNumber(w.scheduler.Game, w.scheduler.Name, 0)
		reportInstanceTerminatingNumber(w.scheduler.Game, w.scheduler.Name, 0)
		return
	}
	readyInstances, pendingInstances, errorInstances, unknownInstances, terminatingInstances := 0, 0, 0, 0, 0
	for _, instance := range instances {
		switch instance.Status.Type {
		case game_room.InstanceReady:
			readyInstances++
		case game_room.InstancePending:
			pendingInstances++
		case game_room.InstanceError:
			errorInstances++
		case game_room.InstanceUnknown:
			unknownInstances++
		case game_room.InstanceTerminating:
			terminatingInstances++
		}
	}
	reportInstanceReadyNumber(w.scheduler.Game, w.scheduler.Name, readyInstances)
	reportInstancePendingNumber(w.scheduler.Game, w.scheduler.Name, pendingInstances)
	reportInstanceErrorNumber(w.scheduler.Game, w.scheduler.Name, errorInstances)
	reportInstanceUnknownNumber(w.scheduler.Game, w.scheduler.Name, unknownInstances)
	reportInstanceTerminatingNumber(w.scheduler.Game, w.scheduler.Name, terminatingInstances)

}

func (w *MetricsReporterWorker) reportGameRoomMetrics() {
	w.logger.Debug("Reporting game room metrics")
	w.reportReadyRooms()
	w.reportPendingRooms()
	w.reportErrorRooms()
	w.reportOccupiedRooms()
	w.reportTerminatingRooms()
	w.reportUnreadyRooms()
	w.reportActiveRooms()
	w.reportTotalRunningMatches()
}

func (w *MetricsReporterWorker) reportSchedulerMetrics() {
	w.logger.Debug("Reporting scheduler metrics")
	w.reportSchedulerAutoscale()
	w.reportSchedulerMaxMatches()
}

func (w *MetricsReporterWorker) reportSchedulerAutoscale() {
	if w.scheduler.Autoscaling == nil {
		return
	}
	if w.scheduler.Autoscaling.Policy.Parameters.RoomOccupancy != nil {
		reportSchedulerPolicyReadyTarget(w.scheduler.Game, w.scheduler.Name, w.scheduler.Autoscaling.Policy.Parameters.RoomOccupancy.ReadyTarget)
	}
}

func (w *MetricsReporterWorker) reportSchedulerMaxMatches() {
	if w.scheduler.MatchAllocation == nil {
		return
	}
	reportSchedulerMaxMatches(w.scheduler.Game, w.scheduler.Name, w.scheduler.MatchAllocation.MaxMatches)
}

func (w *MetricsReporterWorker) reportPendingRooms() {
	pendingRooms, err := w.roomStorage.GetRoomCountByStatus(w.workerContext, w.scheduler.Name, game_room.GameStatusPending)
	if err != nil {
		w.logger.Error("Error getting pending pods", zap.Error(err))
		reportGameRoomPendingNumber(w.scheduler.Game, w.scheduler.Name, 0)
		return
	}
	reportGameRoomPendingNumber(w.scheduler.Game, w.scheduler.Name, pendingRooms)
}

func (w *MetricsReporterWorker) reportReadyRooms() {
	readyRooms, err := w.roomStorage.GetRoomCountByStatus(w.workerContext, w.scheduler.Name, game_room.GameStatusReady)
	if err != nil {
		w.logger.Error("Error getting ready pods", zap.Error(err))
		reportGameRoomReadyNumber(w.scheduler.Game, w.scheduler.Name, 0)
		return
	}
	reportGameRoomReadyNumber(w.scheduler.Game, w.scheduler.Name, readyRooms)
}

func (w *MetricsReporterWorker) reportOccupiedRooms() {
	occupiedRooms, err := w.roomStorage.GetRoomCountByStatus(w.workerContext, w.scheduler.Name, game_room.GameStatusOccupied)
	if err != nil {
		w.logger.Error("Error getting occupied pods", zap.Error(err))
		reportGameRoomOccupiedNumber(w.scheduler.Game, w.scheduler.Name, 0)
		return
	}
	reportGameRoomOccupiedNumber(w.scheduler.Game, w.scheduler.Name, occupiedRooms)
}

func (w *MetricsReporterWorker) reportTerminatingRooms() {
	terminatingRooms, err := w.roomStorage.GetRoomCountByStatus(w.workerContext, w.scheduler.Name, game_room.GameStatusTerminating)
	if err != nil {
		w.logger.Error("Error getting terminating pods", zap.Error(err))
		reportGameRoomTerminatingNumber(w.scheduler.Game, w.scheduler.Name, 0)
		return
	}
	reportGameRoomTerminatingNumber(w.scheduler.Game, w.scheduler.Name, terminatingRooms)
}

func (w *MetricsReporterWorker) reportErrorRooms() {
	errorRooms, err := w.roomStorage.GetRoomCountByStatus(w.workerContext, w.scheduler.Name, game_room.GameStatusError)
	if err != nil {
		w.logger.Error("Error getting error pods", zap.Error(err))
		reportGameRoomErrorNumber(w.scheduler.Game, w.scheduler.Name, 0)
		return
	}
	reportGameRoomErrorNumber(w.scheduler.Game, w.scheduler.Name, errorRooms)
}

func (w *MetricsReporterWorker) reportUnreadyRooms() {
	unreadyRooms, err := w.roomStorage.GetRoomCountByStatus(w.workerContext, w.scheduler.Name, game_room.GameStatusUnready)
	if err != nil {
		w.logger.Error("Error getting unready pods", zap.Error(err))
		reportGameRoomUnreadyNumber(w.scheduler.Game, w.scheduler.Name, 0)
		return
	}
	reportGameRoomUnreadyNumber(w.scheduler.Game, w.scheduler.Name, unreadyRooms)
}

func (w *MetricsReporterWorker) reportActiveRooms() {
	activeRooms, err := w.roomStorage.GetRoomCountByStatus(w.workerContext, w.scheduler.Name, game_room.GameStatusActive)
	if err != nil {
		w.logger.Error("Error getting active pods", zap.Error(err))
		reportGameRoomActiveNumber(w.scheduler.Game, w.scheduler.Name, 0)
		return
	}

	reportGameRoomActiveNumber(w.scheduler.Game, w.scheduler.Name, activeRooms)
}

func (w *MetricsReporterWorker) reportTotalRunningMatches() {
	runningMatches, err := w.roomStorage.GetRunningMatchesCount(w.workerContext, w.scheduler.Name)
	if err != nil {
		w.logger.Error("Error getting running matches", zap.Error(err))
		reportTotalRunningMatches(w.scheduler.Game, w.scheduler.Name, 0)
		return
	}

	reportTotalRunningMatches(w.scheduler.Game, w.scheduler.Name, runningMatches)
}
