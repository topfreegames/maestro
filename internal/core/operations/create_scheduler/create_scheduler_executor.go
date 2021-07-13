package create_scheduler

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

type CreateSchedulerExecutor struct {
	definition CreateSchedulerDefinition
}

func NewExecutor() *CreateSchedulerExecutor {
	return &CreateSchedulerExecutor{}
}

func (e CreateSchedulerExecutor) Execute(ctx context.Context, op operation.Operation, definition operations.Definition) error {
	return nil
}

func (e CreateSchedulerExecutor) OnError(ctx context.Context, op operation.Operation, definition operations.Definition, executeErr error) error {
	return nil
}
