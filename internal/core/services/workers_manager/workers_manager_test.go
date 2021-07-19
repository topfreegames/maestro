package workers_manager

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	schedulerStorageMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	"github.com/topfreegames/maestro/internal/config"
	configMock "github.com/topfreegames/maestro/internal/config/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports/errors"

	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
	workerMock "github.com/topfreegames/maestro/internal/core/workers/mock"
)

func TestStart(t *testing.T) {

	t.Run("with success", func(t *testing.T) {

		core, recorded := observer.New(zap.InfoLevel)
		zl := zap.New(core)
		zap.ReplaceGlobals(zl)

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := operation_manager.New(nil, nil)
		worker := workerMock.NewMockWorker(mockCtrl)

		workerBuilder := func(scheduler *entities.Scheduler, configs config.Config, operation_manager operation_manager.OperationManager) workers.Worker {
			return worker
		}

		worker.EXPECT().Start(context.Background()).Return(nil)
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

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, *operationManager)

		err := workersManager.Start(context.Background())

		time.Sleep(time.Second * 1)

		require.NoError(t, err)
		require.True(t, workersManager.RunSyncWorkers)
		require.Contains(t, workersManager.CurrentWorkers, "zooba-us")

		assertLogMessages(t, recorded, map[zapcore.Level][]string{
			zap.InfoLevel: {"starting to sync operation workers",
				"new operation worker running"},
		})
	})

	t.Run("fails when schedulerStorage fails to list all schedulers", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := operation_manager.New(nil, nil)

		configs.EXPECT().GetInt(syncWorkersIntervalPath).Return(10)
		schedulerStorage.EXPECT().GetAllSchedulers(context.Background()).Return(nil, errors.ErrUnexpected)
		worker := workerMock.NewMockWorker(mockCtrl)
		workerBuilder := func(scheduler *entities.Scheduler, configs config.Config, operation_manager operation_manager.OperationManager) workers.Worker {
			return worker
		}

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, *operationManager)

		err := workersManager.Start(context.Background())

		time.Sleep(time.Second * 1)

		require.ErrorIs(t, errors.ErrUnexpected, err)
		require.Empty(t, workersManager.CurrentWorkers)
		require.False(t, workersManager.RunSyncWorkers)
	})

	t.Run("stops when context stops with no error", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		ctx, cancel := context.WithCancel(context.Background())

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := operation_manager.New(nil, nil)
		worker := workerMock.NewMockWorker(mockCtrl)
		workerBuilder := func(scheduler *entities.Scheduler, configs config.Config, operation_manager operation_manager.OperationManager) workers.Worker {
			return worker
		}

		worker.EXPECT().Start(ctx).Return(nil)
		worker.EXPECT().Stop(ctx).Do(func(cont context.Context) {})
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

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, *operationManager)

		err := workersManager.Start(ctx)

		time.Sleep(time.Second * 1)

		require.NoError(t, err)
		require.True(t, workersManager.RunSyncWorkers)
		require.Contains(t, workersManager.CurrentWorkers, "zooba-us")

		cancel()

		time.Sleep(time.Second * 1)

		require.Empty(t, workersManager.CurrentWorkers)
		require.False(t, workersManager.RunSyncWorkers)

	})

	t.Run("with success when scheduler added after initial sync", func(t *testing.T) {

		core, recorded := observer.New(zap.InfoLevel)
		zl := zap.New(core)
		zap.ReplaceGlobals(zl)

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := operation_manager.New(nil, nil)
		worker := workerMock.NewMockWorker(mockCtrl)
		workerBuilder := func(scheduler *entities.Scheduler, configs config.Config, operation_manager operation_manager.OperationManager) workers.Worker {
			return worker
		}

		worker.EXPECT().Start(context.Background()).Return(nil)
		configs.EXPECT().GetInt(syncWorkersIntervalPath).AnyTimes().Return(10)
		schedulerStorage.EXPECT().GetAllSchedulers(context.Background()).Return([]*entities.Scheduler{}, nil)

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, *operationManager)
		workersManager.Start(context.Background())
		require.Empty(t, workersManager.CurrentWorkers)

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
	})

	t.Run("with success when scheduler removed after bootstrap", func(t *testing.T) {

		core, recorded := observer.New(zap.InfoLevel)
		zl := zap.New(core)
		zap.ReplaceGlobals(zl)

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := operation_manager.New(nil, nil)
		worker := workerMock.NewMockWorker(mockCtrl)
		workerBuilder := func(scheduler *entities.Scheduler, configs config.Config, operation_manager operation_manager.OperationManager) workers.Worker {
			return worker
		}

		worker.EXPECT().Start(context.Background()).Return(nil)
		worker.EXPECT().Stop(context.Background()).Do(func(cont context.Context) {})
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

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, *operationManager)
		workersManager.Start(context.Background())

		// Has to sleep 1 second in order to start the goroutine
		time.Sleep(2 * time.Second)

		require.Contains(t, workersManager.CurrentWorkers, "zooba-us")

		assertLogMessages(t, recorded, map[zapcore.Level][]string{
			zap.InfoLevel: {"starting to sync operation workers",
				"new operation worker running"},
		})

		core, recorded = observer.New(zap.InfoLevel)
		zl = zap.New(core)
		zap.ReplaceGlobals(zl)

		schedulerStorage.EXPECT().GetAllSchedulers(context.Background()).Return([]*entities.Scheduler{}, nil)

		workersManager.SyncWorkers(context.Background())

		require.Empty(t, workersManager.CurrentWorkers)

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
