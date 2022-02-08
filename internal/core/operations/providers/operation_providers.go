// MIT License
//
// Copyright (c) 2021 TFG Co
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
	"github.com/topfreegames/maestro/internal/core/operations/add_rooms"
	"github.com/topfreegames/maestro/internal/core/operations/create_scheduler"
	"github.com/topfreegames/maestro/internal/core/operations/newschedulerversion"
	"github.com/topfreegames/maestro/internal/core/operations/remove_rooms"
	"github.com/topfreegames/maestro/internal/core/operations/switch_active_version"
	"github.com/topfreegames/maestro/internal/core/operations/test_operation"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"
)

func ProvideDefinitionConstructors() map[string]operations.DefinitionConstructor {

	definitionConstructors := map[string]operations.DefinitionConstructor{}
	definitionConstructors[create_scheduler.OperationName] = func() operations.Definition {
		return &create_scheduler.CreateSchedulerDefinition{}
	}
	definitionConstructors[add_rooms.OperationName] = func() operations.Definition {
		return &add_rooms.AddRoomsDefinition{}
	}
	definitionConstructors[remove_rooms.OperationName] = func() operations.Definition {
		return &remove_rooms.RemoveRoomsDefinition{}
	}
	definitionConstructors[test_operation.OperationName] = func() operations.Definition {
		return &test_operation.TestOperationDefinition{}
	}
	definitionConstructors[newschedulerversion.OperationName] = func() operations.Definition {
		return &newschedulerversion.CreateNewSchedulerVersionDefinition{}
	}
	definitionConstructors[switch_active_version.OperationName] = func() operations.Definition {
		return &switch_active_version.SwitchActiveVersionDefinition{}
	}

	return definitionConstructors

}

func ProvideExecutors(
	runtime ports.Runtime,
	schedulerStorage ports.SchedulerStorage,
	roomManager *room_manager.RoomManager,
	schedulerManager *scheduler_manager.SchedulerManager,
) map[string]operations.Executor {

	executors := map[string]operations.Executor{}
	executors[create_scheduler.OperationName] = create_scheduler.NewExecutor(runtime, schedulerStorage)
	executors[add_rooms.OperationName] = add_rooms.NewExecutor(roomManager, schedulerStorage)
	executors[remove_rooms.OperationName] = remove_rooms.NewExecutor(roomManager)
	executors[test_operation.OperationName] = test_operation.NewExecutor()
	executors[switch_active_version.OperationName] = switch_active_version.NewExecutor(roomManager, schedulerManager)
	executors[newschedulerversion.OperationName] = newschedulerversion.NewExecutor(roomManager, schedulerManager)

	return executors

}
