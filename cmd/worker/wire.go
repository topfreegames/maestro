//+build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/operations/providers"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/services/workers_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
	"github.com/topfreegames/maestro/internal/service"
)

func initializeWorker(c config.Config, builder workers.WorkerBuilder) (*workers_manager.WorkersManager, error) {
	wire.Build(
		// ports + adapters
		service.NewRuntimeKubernetes,
		service.NewSchedulerStoragePg,
		service.NewOperationFlowRedis,
		service.NewClockTime,
		service.NewOperationStorageRedis,

		// scheduler operations
		providers.ProvideDefinitionConstructors,
		providers.ProvideExecutors,

		// services
		operation_manager.New,
		workers.ProvideWorkerOptions,
		workers_manager.NewWorkersManager,
	)

	return &workers_manager.WorkersManager{}, nil
}
