package providers

import (
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/operations/create_scheduler"
	"github.com/topfreegames/maestro/internal/core/ports"
)

var createSchedulerOperationName = "create_scheduler"

func ProvideDefinitionConstructors() map[string]operations.DefinitionConstructor {

	definitionConstructors := map[string]operations.DefinitionConstructor{}
	definitionConstructors[createSchedulerOperationName] = func() operations.Definition {
		return &create_scheduler.CreateSchedulerDefinition{}
	}

	return definitionConstructors

}

func ProvideExecutors(runtime ports.Runtime, schedulerStorage ports.SchedulerStorage) map[string]operations.Executor {

	executors := map[string]operations.Executor{}
	executors[createSchedulerOperationName] = create_scheduler.NewExecutor(runtime, schedulerStorage)

	return executors

}
