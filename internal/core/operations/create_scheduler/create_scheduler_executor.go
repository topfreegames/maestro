package create_scheduler

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports"
)

type CreateSchedulerExecutor struct {
	runtime ports.Runtime
	storage ports.SchedulerStorage
}

func NewExecutor(runtime ports.Runtime, storage ports.SchedulerStorage) *CreateSchedulerExecutor {
	return &CreateSchedulerExecutor{
		runtime: runtime,
		storage: storage,
	}
}

func (e *CreateSchedulerExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {

	err := e.runtime.CreateScheduler(ctx, &entities.Scheduler{Name: op.SchedulerName})
	if err != nil {
		return fmt.Errorf("failed to create scheduler in runtime: %w", err)
	}

	return nil
}

func (e *CreateSchedulerExecutor) OnError(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	scheduler, err := e.storage.GetScheduler(ctx, op.SchedulerName)
	if err != nil {
		return fmt.Errorf("failed to find scheduler by id: %w", err)
	}

	scheduler.State = entities.StateOnError

	return e.storage.UpdateScheduler(ctx, scheduler)
}

func (e *CreateSchedulerExecutor) Name() string {
	return operationName
}
