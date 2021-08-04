//+build unit

package scheduler_manager

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	opflow "github.com/topfreegames/maestro/internal/adapters/operation_flow/mock"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	schedulerStorageMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
)

func TestGetAllSchedulers(t *testing.T) {

	t.Run("when storage has single scheduler", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := operation_manager.New(nil, nil)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager)

		schedulerStorage.EXPECT().GetAllSchedulers(gomock.Any()).Return([]*entities.Scheduler{
			{
				Name:            "zooba-us",
				Game:            "zooba",
				State:           entities.StateInSync,
				RollbackVersion: "1.0.0",
				PortRange: &entities.PortRange{
					Start: 1,
					End:   2,
				},
			},
		}, nil)

		schedulers, err := schedulerManager.GetAllSchedulers(context.Background())
		require.NoError(t, err)
		require.Len(t, schedulers, 1)
	})
}

func TestCreateScheduler(t *testing.T) {

	t.Run("with success", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager)

		ctx := context.Background()
		scheduler := &entities.Scheduler{
			Name:            "scheduler",
			Game:            "game",
			State:           entities.StateCreating,
			RollbackVersion: "",
			Spec: game_room.Spec{
				Version:                "v1",
				TerminationGracePeriod: 60,
				Containers:             []game_room.Container{},
				Toleration:             "toleration",
				Affinity:               "affinity",
			},
			PortRange: &entities.PortRange{
				Start: 40000,
				End:   60000,
			},
		}

		schedulerStorage.EXPECT().CreateScheduler(ctx, gomock.Any()).Return(nil)
		operationStorage.EXPECT().CreateOperation(ctx, gomock.Any(), gomock.Any()).Return(nil)
		operationFlow.EXPECT().InsertOperationID(ctx, "scheduler", gomock.Any()).Return(nil)
		schedulerStorage.EXPECT().GetScheduler(ctx, "scheduler").Return(scheduler, nil)

		schedulerResult, err := schedulerManager.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)
		require.Equal(t, scheduler, schedulerResult)
	})

	t.Run("fails when scheduler already exists", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager)

		ctx := context.Background()
		scheduler := &entities.Scheduler{
			Name:            "scheduler",
			Game:            "game",
			State:           entities.StateCreating,
			RollbackVersion: "",
			Spec: game_room.Spec{
				Version:                "v1",
				TerminationGracePeriod: 60,
				Containers:             []game_room.Container{},
				Toleration:             "toleration",
				Affinity:               "affinity",
			},
			PortRange: &entities.PortRange{
				Start: 40000,
				End:   60000,
			},
		}

		schedulerStorage.EXPECT().CreateScheduler(ctx, gomock.Any()).Return(nil)
		operationStorage.EXPECT().CreateOperation(ctx, gomock.Any(), gomock.Any()).Return(nil)
		operationFlow.EXPECT().InsertOperationID(ctx, "scheduler", gomock.Any()).Return(nil)
		schedulerStorage.EXPECT().GetScheduler(ctx, "scheduler").Return(scheduler, nil)

		schedulerResult, err := schedulerManager.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)
		require.Equal(t, scheduler, schedulerResult)
	})
}
