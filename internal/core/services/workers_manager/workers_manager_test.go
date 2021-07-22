//+build unit

package workers_manager

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	schedulerStorageMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	configMock "github.com/topfreegames/maestro/internal/config/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/monitoring"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	workerMock "github.com/topfreegames/maestro/internal/core/workers/mock"

	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
)

var (
	recorded *observer.ObservedLogs
	mockCtrl *gomock.Controller
)

func BeforeTest(t *testing.T) {
	clearMetrics()
	initMetrics()
	core, observer := observer.New(zap.InfoLevel)
	zl := zap.New(core)
	zap.ReplaceGlobals(zl)
	recorded = observer

	mockCtrl = gomock.NewController(t)
	defer mockCtrl.Finish()
}

func TestStart(t *testing.T) {

	t.Run("with success", func(t *testing.T) {

		BeforeTest(t)

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := operation_manager.New(nil, nil)

		workerBuilder := func(_ *entities.Scheduler, _ *workers.WorkerOptions) workers.Worker {
			return &workerMock.MockWorker{Run: false}
		}

		configs.EXPECT().GetInt(syncWorkersIntervalPath).Return(10)
		schedulerStorage.EXPECT().GetAllSchedulers(context.Background()).Return([]*entities.Scheduler{
			{
				Name:            "zooba-us",
				Game:            "zooba",
				State:           entities.StateCreating,
				RollbackVersion: "1.0.0",
				PortRange: &entities.PortRange{
					Start: 1,
					End:   10000,
				},
			},
		}, nil)

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, operationManager)

		err := workersManager.Start(context.Background())

		time.Sleep(time.Second * 1)

		require.NoError(t, err)
		require.True(t, workersManager.RunSyncWorkers)
		require.Contains(t, workersManager.CurrentWorkers, "zooba-us")

		assertLogMessages(t, recorded, map[zapcore.Level][]string{
			zap.InfoLevel: {"starting to sync operation workers",
				"new operation worker running"},
		})

		metrics, _ := prometheus.DefaultGatherer.Gather()
		currentWorkersGauge := monitoring.FilterMetric(metrics, "maestro_worker_current_workers_gauge").GetMetric()[0]
		require.Equal(t, float64(1), currentWorkersGauge.GetGauge().GetValue())
		workerSyncCounter := monitoring.FilterMetric(metrics, "maestro_worker_workers_sync_counter").GetMetric()[0]
		require.Equal(t, float64(1), workerSyncCounter.GetCounter().GetValue())
	})

	t.Run("fails when schedulerStorage fails to list all schedulers", func(t *testing.T) {

		BeforeTest(t)

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := operation_manager.New(nil, nil)

		configs.EXPECT().GetInt(syncWorkersIntervalPath).Return(10)
		schedulerStorage.EXPECT().GetAllSchedulers(context.Background()).Return(nil, errors.ErrUnexpected)
		workerBuilder := func(_ *entities.Scheduler, _ *workers.WorkerOptions) workers.Worker {
			return &workerMock.MockWorker{Run: false}
		}

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, operationManager)

		err := workersManager.Start(context.Background())

		time.Sleep(time.Second * 1)

		require.ErrorIs(t, errors.ErrUnexpected, err)
		require.Empty(t, workersManager.CurrentWorkers)
		require.False(t, workersManager.RunSyncWorkers)

		metrics, _ := prometheus.DefaultGatherer.Gather()
		require.Nil(t, monitoring.FilterMetric(metrics, "maestro_worker_current_workers_gauge").GetMetric())
		require.Empty(t, monitoring.FilterMetric(metrics, "maestro_worker_workers_sync_counter").GetMetric())
	})

	t.Run("stops when context stops with no error", func(t *testing.T) {

		BeforeTest(t)

		ctx, cancel := context.WithCancel(context.Background())

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := operation_manager.New(nil, nil)
		workerBuilder := func(_ *entities.Scheduler, _ *workers.WorkerOptions) workers.Worker {
			return &workerMock.MockWorker{Run: false}
		}

		configs.EXPECT().GetInt(syncWorkersIntervalPath).AnyTimes().Return(10)
		schedulerStorage.EXPECT().GetAllSchedulers(ctx).AnyTimes().Return([]*entities.Scheduler{
			{
				Name:            "zooba-us",
				Game:            "zooba",
				State:           entities.StateCreating,
				RollbackVersion: "1.0.0",
				PortRange: &entities.PortRange{
					Start: 1,
					End:   10000,
				},
			},
		}, nil)

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, operationManager)

		err := workersManager.Start(ctx)

		time.Sleep(time.Second * 1)

		require.NoError(t, err)
		require.True(t, workersManager.RunSyncWorkers)
		require.Contains(t, workersManager.CurrentWorkers, "zooba-us")

		metrics, _ := prometheus.DefaultGatherer.Gather()
		currentWorkersGauge := monitoring.FilterMetric(metrics, "maestro_worker_current_workers_gauge").GetMetric()[0]
		require.Equal(t, float64(1), currentWorkersGauge.GetGauge().GetValue())

		cancel()

		time.Sleep(time.Second * 1)

		require.Empty(t, workersManager.CurrentWorkers)
		require.False(t, workersManager.RunSyncWorkers)

		metrics, _ = prometheus.DefaultGatherer.Gather()
		currentWorkersGauge = monitoring.FilterMetric(metrics, "maestro_worker_current_workers_gauge").GetMetric()[0]
		require.Equal(t, float64(0), currentWorkersGauge.GetGauge().GetValue())
		workerSyncCounter := monitoring.FilterMetric(metrics, "maestro_worker_workers_sync_counter").GetMetric()[0]
		require.Equal(t, float64(1), workerSyncCounter.GetCounter().GetValue())

	})

	t.Run("with success when scheduler added after initial sync", func(t *testing.T) {

		BeforeTest(t)

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := operation_manager.New(nil, nil)
		workerBuilder := func(_ *entities.Scheduler, _ *workers.WorkerOptions) workers.Worker {
			return &workerMock.MockWorker{Run: false}
		}

		configs.EXPECT().GetInt(syncWorkersIntervalPath).AnyTimes().Return(10)
		schedulerStorage.EXPECT().GetAllSchedulers(context.Background()).Return([]*entities.Scheduler{}, nil)

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, operationManager)
		workersManager.Start(context.Background())
		require.Empty(t, workersManager.CurrentWorkers)

		metrics, _ := prometheus.DefaultGatherer.Gather()
		require.Nil(t, monitoring.FilterMetric(metrics, "maestro_worker_current_workers_gauge").GetMetric())

		schedulerStorage.EXPECT().GetAllSchedulers(context.Background()).Return([]*entities.Scheduler{
			{
				Name:            "zooba-us",
				Game:            "zooba",
				State:           entities.StateCreating,
				RollbackVersion: "1.0.0",
				PortRange: &entities.PortRange{
					Start: 1,
					End:   10000,
				},
			},
		}, nil)

		workersManager.SyncWorkers(context.Background())

		time.Sleep(time.Second * 1)

		require.Contains(t, workersManager.CurrentWorkers, "zooba-us")

		assertLogMessages(t, recorded, map[zapcore.Level][]string{
			zap.InfoLevel: {"starting to sync operation workers",
				"new operation worker running"},
		})

		metrics, _ = prometheus.DefaultGatherer.Gather()
		currentWorkersGauge := monitoring.FilterMetric(metrics, "maestro_worker_current_workers_gauge").GetMetric()[0]
		require.Equal(t, float64(1), currentWorkersGauge.GetGauge().GetValue())
	})

	t.Run("with success when scheduler removed after bootstrap", func(t *testing.T) {

		BeforeTest(t)

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := operation_manager.New(nil, nil)
		workerBuilder := func(_ *entities.Scheduler, _ *workers.WorkerOptions) workers.Worker {
			return &workerMock.MockWorker{Run: false}
		}

		configs.EXPECT().GetInt(syncWorkersIntervalPath).AnyTimes().Return(1)
		schedulerStorage.EXPECT().GetAllSchedulers(context.Background()).Times(3).Return([]*entities.Scheduler{
			{
				Name:            "zooba-us",
				Game:            "zooba",
				State:           entities.StateCreating,
				RollbackVersion: "1.0.0",
				PortRange: &entities.PortRange{
					Start: 1,
					End:   10000,
				},
			},
		}, nil)

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, operationManager)
		workersManager.Start(context.Background())

		// Has to sleep 1 second in order to start the goroutine
		time.Sleep(2 * time.Second)

		require.Contains(t, workersManager.CurrentWorkers, "zooba-us")

		metrics, _ := prometheus.DefaultGatherer.Gather()
		currentWorkersGauge := monitoring.FilterMetric(metrics, "maestro_worker_current_workers_gauge").GetMetric()[0]
		require.Equal(t, float64(1), currentWorkersGauge.GetGauge().GetValue())

		assertLogMessages(t, recorded, map[zapcore.Level][]string{
			zap.InfoLevel: {"starting to sync operation workers",
				"new operation worker running"},
		})

		core, recorded := observer.New(zap.InfoLevel)
		zl := zap.New(core)
		zap.ReplaceGlobals(zl)

		schedulerStorage.EXPECT().GetAllSchedulers(context.Background()).Return([]*entities.Scheduler{}, nil)

		workersManager.SyncWorkers(context.Background())

		require.Empty(t, workersManager.CurrentWorkers)

		metrics, _ = prometheus.DefaultGatherer.Gather()
		currentWorkersGauge = monitoring.FilterMetric(metrics, "maestro_worker_current_workers_gauge").GetMetric()[0]
		require.Equal(t, float64(0), currentWorkersGauge.GetGauge().GetValue())

		assertLogMessages(t, recorded, map[zapcore.Level][]string{
			zap.InfoLevel: {"canceling operation worker"},
		})

	})
}

func assertLogMessages(t *testing.T, recorded *observer.ObservedLogs, messages map[zapcore.Level][]string) {
	for level, values := range messages {

		levelRecords := recorded.FilterLevelExact(level)
		for _, message := range values {
			require.NotEmpty(t, levelRecords.Filter(func(le observer.LoggedEntry) bool {
				return le.Message == message
			}))
		}
	}
}
