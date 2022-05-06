// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package worker

import (
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/operations/providers"
	"github.com/topfreegames/maestro/internal/core/services/events_forwarder"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"
	"github.com/topfreegames/maestro/internal/core/services/workers_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
	"github.com/topfreegames/maestro/internal/service"
)

// Injectors from wire.go:

func initializeWorker(c config.Config, builder workers.WorkerBuilder) (*workers_manager.WorkersManager, error) {
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
	operationLeaseStorage, err := service.NewOperationLeaseStorageRedis(clock, c)
	if err != nil {
		return nil, err
	}
	operationManagerConfig, err := service.NewOperationManagerConfig(c)
	if err != nil {
		return nil, err
	}
	operationManager := service.NewOperationManager(operationFlow, operationStorage, v, operationLeaseStorage, operationManagerConfig, schedulerStorage)
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
	eventsForwarder, err := service.NewEventsForwarder(c)
	if err != nil {
		return nil, err
	}
	schedulerCache, err := service.NewSchedulerCacheRedis(c)
	if err != nil {
		return nil, err
	}
	eventsForwarderConfig, err := service.NewEventsForwarderServiceConfig(c)
	if err != nil {
		return nil, err
	}
	eventsService := events_forwarder.NewEventsForwarderService(eventsForwarder, schedulerStorage, gameRoomInstanceStorage, roomStorage, schedulerCache, eventsForwarderConfig)
	roomManagerConfig, err := service.NewRoomManagerConfig(c)
	if err != nil {
		return nil, err
	}
	roomManager := service.NewRoomManager(clock, portAllocator, roomStorage, gameRoomInstanceStorage, runtime, eventsService, roomManagerConfig)
	schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)
	v2 := providers.ProvideExecutors(runtime, schedulerStorage, roomManager, roomStorage, schedulerManager)
	workerOptions := workers.ProvideWorkerOptions(operationManager, v2, roomManager, runtime)
	workersManager := workers_manager.NewWorkersManager(builder, c, schedulerStorage, workerOptions)
	return workersManager, nil
}
