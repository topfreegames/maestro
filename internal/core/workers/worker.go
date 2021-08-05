package workers

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
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
}

func ProvideWorkerOptions(operationManager *operation_manager.OperationManager, operationExecutors map[string]operations.Executor) *WorkerOptions {
	return &WorkerOptions{
		OperationManager:   operationManager,
		OperationExecutors: operationExecutors,
	}
}

// WorkerBuilder defines a function that nows how to construct a worker.
type WorkerBuilder func(scheduler *entities.Scheduler, options *WorkerOptions) Worker
