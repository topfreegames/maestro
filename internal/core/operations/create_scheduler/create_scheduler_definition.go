package create_scheduler

import "github.com/topfreegames/maestro/internal/core/entities/operation"

type CreateSchedulerDefinition struct {}

func (d *CreateSchedulerDefinition) ShouldExecute(currentOperations []operation.Operation) bool {
	return true
}

func (d *CreateSchedulerDefinition) Name() string {
	return "create_scheduler"
}

func (d *CreateSchedulerDefinition) Marshal() []byte {		
	return make([]byte, 0)
}

func (d *CreateSchedulerDefinition) Unmarshal(raw []byte) error {
	return nil
}
