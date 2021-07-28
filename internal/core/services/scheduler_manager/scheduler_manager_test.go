//+build unit

package scheduler_manager

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	schedulerStorageMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
)

func TestGetAllSchedulers(t *testing.T) {

	t.Run("when storage has single scheduler", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage)

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
