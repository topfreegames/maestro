package scheduler_manager

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/operations/create_scheduler"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"go.uber.org/zap"
)

type SchedulerManager struct {
	schedulerStorage ports.SchedulerStorage
	operationManager *operation_manager.OperationManager
}

func NewSchedulerManager(schedulerStorage ports.SchedulerStorage, operationManager *operation_manager.OperationManager) *SchedulerManager {
	return &SchedulerManager{
		schedulerStorage: schedulerStorage,
		operationManager: operationManager,
	}
}

func (s *SchedulerManager) CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) (*entities.Scheduler, error) {
	scheduler.State = entities.StateCreating

	err := s.schedulerStorage.CreateScheduler(ctx, scheduler)
	if err != nil {
		return nil, err
	}

	operation, err := s.operationManager.CreateOperation(ctx, scheduler.Name, &create_scheduler.CreateSchedulerDefinition{})
	if err != nil {
		return nil, fmt.Errorf("failed to schedule 'create scheduler' operation: %w", err)
	}

	zap.L().Info("scheduler enqueued to be created", zap.String("scheduler", scheduler.Name), zap.String("operation", operation.ID))

	return s.schedulerStorage.GetScheduler(ctx, scheduler.Name)
}

func (s *SchedulerManager) GetAllSchedulers(ctx context.Context) ([]*entities.Scheduler, error) {
	return s.schedulerStorage.GetAllSchedulers(ctx)
}
