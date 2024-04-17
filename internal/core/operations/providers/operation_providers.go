// MIT License
//
// Copyright (c) 01 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package providers

import (
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"
	addrooms "github.com/topfreegames/maestro/internal/core/operations/rooms/add"
	removerooms "github.com/topfreegames/maestro/internal/core/operations/rooms/remove"
	createscheduler "github.com/topfreegames/maestro/internal/core/operations/schedulers/create"
	deletescheduler "github.com/topfreegames/maestro/internal/core/operations/schedulers/delete"
	newversion "github.com/topfreegames/maestro/internal/core/operations/schedulers/newversion"
	"github.com/topfreegames/maestro/internal/core/operations/schedulers/switchversion"
	"github.com/topfreegames/maestro/internal/core/operations/storagecleanup"
	"github.com/topfreegames/maestro/internal/core/operations/test"
	"github.com/topfreegames/maestro/internal/core/ports"
)

// ProvideDefinitionConstructors create definition constructors.
func ProvideDefinitionConstructors() map[string]operations.DefinitionConstructor {

	definitionConstructors := map[string]operations.DefinitionConstructor{}
	definitionConstructors[createscheduler.OperationName] = func() operations.Definition {
		return &createscheduler.Definition{}
	}
	definitionConstructors[addrooms.OperationName] = func() operations.Definition {
		return &addrooms.Definition{}
	}
	definitionConstructors[removerooms.OperationName] = func() operations.Definition {
		return &removerooms.Definition{}
	}
	definitionConstructors[test.OperationName] = func() operations.Definition {
		return &test.Definition{}
	}
	definitionConstructors[newversion.OperationName] = func() operations.Definition {
		return &newversion.Definition{}
	}
	definitionConstructors[switchversion.OperationName] = func() operations.Definition {
		return &switchversion.Definition{}
	}
	definitionConstructors[healthcontroller.OperationName] = func() operations.Definition {
		return &healthcontroller.Definition{}
	}
	definitionConstructors[deletescheduler.OperationName] = func() operations.Definition {
		return &deletescheduler.Definition{}
	}
	definitionConstructors[storagecleanup.OperationName] = func() operations.Definition {
		return &storagecleanup.Definition{}
	}

	return definitionConstructors

}

// ProvideExecutors create providerMap to operations.
func ProvideExecutors(
	runtime ports.Runtime,
	schedulerStorage ports.SchedulerStorage,
	roomManager ports.RoomManager,
	roomStorage ports.RoomStorage,
	schedulerManager ports.SchedulerManager,
	instanceStorage ports.GameRoomInstanceStorage,
	schedulerCache ports.SchedulerCache,
	operationStorage ports.OperationStorage,
	operationManager ports.OperationManager,
	autoscaler ports.Autoscaler,
	newSchedulerVersionConfig newversion.Config,
	healthControllerConfig healthcontroller.Config,
) map[string]operations.Executor {

	executors := map[string]operations.Executor{}
	executors[createscheduler.OperationName] = createscheduler.NewExecutor(runtime, schedulerManager, operationManager)
	executors[addrooms.OperationName] = addrooms.NewExecutor(roomManager, schedulerStorage, operationManager)
	executors[removerooms.OperationName] = removerooms.NewExecutor(roomManager, roomStorage, operationManager)
	executors[test.OperationName] = test.NewExecutor()
	executors[switchversion.OperationName] = switchversion.NewExecutor(schedulerManager, operationManager)
	executors[newversion.OperationName] = newversion.NewExecutor(roomManager, schedulerManager, operationManager, newSchedulerVersionConfig)
	executors[healthcontroller.OperationName] = healthcontroller.NewExecutor(roomStorage, roomManager, instanceStorage, schedulerStorage, operationManager, autoscaler, healthControllerConfig)
	executors[storagecleanup.OperationName] = storagecleanup.NewExecutor(operationStorage)
	executors[deletescheduler.OperationName] = deletescheduler.NewExecutor(schedulerStorage, schedulerCache, instanceStorage, operationStorage, operationManager, runtime)

	return executors

}
