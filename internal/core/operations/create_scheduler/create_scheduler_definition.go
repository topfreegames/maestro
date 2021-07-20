package create_scheduler

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

const operationName = "create_scheduler"

type CreateSchedulerDefinition struct{}

// ShouldExecute we're going to always perform the scheduler creation.
func (d *CreateSchedulerDefinition) ShouldExecute(_ context.Context, _ []*operation.Operation) bool {
	return true
}

func (d *CreateSchedulerDefinition) Name() string {
	return operationName
}

func (d *CreateSchedulerDefinition) Marshal() []byte {
	return make([]byte, 0)
}

func (d *CreateSchedulerDefinition) Unmarshal(raw []byte) error {
	return nil
}
