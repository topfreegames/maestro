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

	opflow "github.com/topfreegames/maestro/internal/adapters/operation_flow/mock"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	schedulerStorageMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	configMock "github.com/topfreegames/maestro/internal/config/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
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
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage)

		configs.EXPECT().GetInt(syncOperationWorkersIntervalPath).Return(10)
		configs.EXPECT().GetInt(workers.OperationWorkerIntervalPath).Return(10)
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

		workersManager := NewWorkersManager(configs, schedulerStorage, *operationManager)

		err := workersManager.Start(context.Background())

		time.Sleep(time.Second * 1)

		require.NoError(t, err)
		require.True(t, workersManager.RunSyncOperationWorkers)
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
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage)

		configs.EXPECT().GetInt(syncOperationWorkersIntervalPath).Return(10)
		schedulerStorage.EXPECT().GetAllSchedulers(context.Background()).Return(nil, errors.ErrUnexpected)

		workersManager := NewWorkersManager(configs, schedulerStorage, *operationManager)

		err := workersManager.Start(context.Background())

		time.Sleep(time.Second * 1)

		require.ErrorIs(t, errors.ErrUnexpected, err)
		require.Empty(t, workersManager.CurrentWorkers)
		require.False(t, workersManager.RunSyncOperationWorkers)
	})

	t.Run("stops when context stops with no error", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		ctx, cancel := context.WithCancel(context.Background())

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage)

		configs.EXPECT().GetInt(syncOperationWorkersIntervalPath).AnyTimes().Return(10)
		configs.EXPECT().GetInt(workers.OperationWorkerIntervalPath).AnyTimes().Return(10)
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
		workersManager := NewWorkersManager(configs, schedulerStorage, *operationManager)

		err := workersManager.Start(ctx)

		time.Sleep(time.Second * 1)

		require.NoError(t, err)
		require.True(t, workersManager.RunSyncOperationWorkers)
		require.Contains(t, workersManager.CurrentWorkers, "zooba-us")
		operationWorker := workersManager.CurrentWorkers["zooba-us"]
		require.True(t, operationWorker.IsRunning(ctx))

		cancel()

		time.Sleep(time.Second * 1)

		require.Empty(t, workersManager.CurrentWorkers)
		require.False(t, workersManager.RunSyncOperationWorkers)
		require.False(t, operationWorker.IsRunning(ctx))

	})

	t.Run("with success when scheduler added after initial sync", func(t *testing.T) {

		core, recorded := observer.New(zap.InfoLevel)
		zl := zap.New(core)
		zap.ReplaceGlobals(zl)

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage)

		configs.EXPECT().GetInt(syncOperationWorkersIntervalPath).Return(10)
		configs.EXPECT().GetInt(workers.OperationWorkerIntervalPath).Return(10)
		schedulerStorage.EXPECT().GetAllSchedulers(context.Background()).Return([]*entities.Scheduler{}, nil)

		workersManager := NewWorkersManager(configs, schedulerStorage, *operationManager)
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

		workersManager.SyncOperationWorkers(context.Background())

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
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage)

		configs.EXPECT().GetInt(syncOperationWorkersIntervalPath).AnyTimes().Return(1)
		configs.EXPECT().GetInt(workers.OperationWorkerIntervalPath).AnyTimes().Return(1)

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

		workersManager := NewWorkersManager(configs, schedulerStorage, *operationManager)
		workersManager.Start(context.Background())

		// Has to sleep 1 second in order to start the goroutine
		time.Sleep(2 * time.Second)

		require.Contains(t, workersManager.CurrentWorkers, "zooba-us")
		operationWorker := workersManager.CurrentWorkers["zooba-us"]
		require.Equal(t, true, operationWorker.IsRunning(context.Background()))

		assertLogMessages(t, recorded, map[zapcore.Level][]string{
			zap.InfoLevel: {"starting to sync operation workers",
				"new operation worker running"},
		})

		core, recorded = observer.New(zap.InfoLevel)
		zl = zap.New(core)
		zap.ReplaceGlobals(zl)

		schedulerStorage.EXPECT().GetAllSchedulers(context.Background()).Return([]*entities.Scheduler{}, nil)

		workersManager.SyncOperationWorkers(context.Background())

		require.Empty(t, workersManager.CurrentWorkers)
		require.Equal(t, false, operationWorker.IsRunning(context.Background()))

		assertLogMessages(t, recorded, map[zapcore.Level][]string{
			zap.InfoLevel: {"starting to sync operation workers"},
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
