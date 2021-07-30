package scheduler_manager

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports"
)

type SchedulerManager struct {
	schedulerStorage ports.SchedulerStorage
}

func NewSchedulerManager(schedulerStorage ports.SchedulerStorage) *SchedulerManager {
	return &SchedulerManager{
		schedulerStorage: schedulerStorage,
	}
}

func (s *SchedulerManager) GetAllSchedulers(ctx context.Context) ([]*entities.Scheduler, error) {
	return s.schedulerStorage.GetAllSchedulers(ctx)
}
