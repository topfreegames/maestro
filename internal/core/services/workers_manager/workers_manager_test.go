package workers_manager

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	opflow "github.com/topfreegames/maestro/internal/adapters/operation_flow/mock"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	schedulerStorageMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	configMock "github.com/topfreegames/maestro/internal/config/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
)

func TestStart(t *testing.T) {
	t.Run("with success", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage)

		configs.EXPECT().GetInt(syncOperationWorkersIntervalPath).Return(10)
		configs.EXPECT().GetInt(workers.SyncPeriodPath).Return(10)
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

		workersManager.Start(context.Background())

		require.Contains(t, workersManager.CurrentWorkers, "zooba-us")

	})
}
