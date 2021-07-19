package workers

import (
	"context"

	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/entities"
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
	IsRunning(ctx context.Context) bool
}

type WorkerBuilder func(
	scheduler *entities.Scheduler,
	configs config.Config,
	operation_manager operation_manager.OperationManager,
) Worker
