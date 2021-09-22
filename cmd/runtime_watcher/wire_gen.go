// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//+build !wireinject

package main

import (
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/operations/providers"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
	"github.com/topfreegames/maestro/internal/core/services/workers_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
	"github.com/topfreegames/maestro/internal/core/workers/runtime_watcher_worker"
	"github.com/topfreegames/maestro/internal/service"
)

// Injectors from wire.go:

func initializeRuntimeWatcher(c config.Config) (*workers_manager.WorkersManager, error) {
	workerBuilder := provideRuntimeWatcherBuilder()
	schedulerStorage, err := service.NewSchedulerStoragePg(c)
	if err != nil {
		return nil, err
	}
	operationFlow, err := service.NewOperationFlowRedis(c)
	if err != nil {
		return nil, err
	}
	clock := service.NewClockTime()
	operationStorage, err := service.NewOperationStorageRedis(clock, c)
	if err != nil {
		return nil, err
	}
	v := providers.ProvideDefinitionConstructors()
	operationManager := operation_manager.New(operationFlow, operationStorage, v)
	runtime, err := service.NewRuntimeKubernetes(c)
	if err != nil {
		return nil, err
	}
	portAllocator, err := service.NewPortAllocatorRandom(c)
	if err != nil {
		return nil, err
	}
	roomStorage, err := service.NewRoomStorageRedis(c)
	if err != nil {
		return nil, err
	}
	gameRoomInstanceStorage, err := service.NewGameRoomInstanceStorageRedis(c)
	if err != nil {
		return nil, err
	}
	roomManagerConfig, err := service.NewRoomManagerConfig(c)
	if err != nil {
		return nil, err
	}
	roomManager := room_manager.NewRoomManager(clock, portAllocator, roomStorage, gameRoomInstanceStorage, runtime, roomManagerConfig)
	v2 := providers.ProvideExecutors(runtime, schedulerStorage, roomManager)
	workerOptions := workers.ProvideWorkerOptions(operationManager, v2)
	workersManager := workers_manager.NewWorkersManager(workerBuilder, c, schedulerStorage, workerOptions)
	return workersManager, nil
}

// wire.go:

func provideRuntimeWatcherBuilder() workers.WorkerBuilder {
	return runtime_watcher_worker.NewRuntimeWatcherWorker
}
