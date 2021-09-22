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

package workers

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
)

// This interface aims to map all required functions of a worker
type Worker interface {
	// Starts the worker with its own execution
	// configuration details
	Start(ctx context.Context) error
	// Stops the worker
	Stop(ctx context.Context)
	// Returns if the worker is running
	IsRunning() bool
}

// WorkerOptions define all possible options that a worker can require during
// its construction. This struct is going to be used to inject the worker
// dependencies like ports.
type WorkerOptions struct {
	OperationManager   *operation_manager.OperationManager
	OperationExecutors map[string]operations.Executor
	RoomManager        *room_manager.RoomManager
	Runtime            ports.Runtime
}

func ProvideWorkerOptions(
	operationManager *operation_manager.OperationManager,
	operationExecutors map[string]operations.Executor,
	roomManager *room_manager.RoomManager,
	runtime ports.Runtime,
) *WorkerOptions {
	return &WorkerOptions{
		OperationManager:   operationManager,
		OperationExecutors: operationExecutors,
	}
}

// WorkerBuilder defines a function that nows how to construct a worker.
type WorkerBuilder func(scheduler *entities.Scheduler, options *WorkerOptions) Worker
