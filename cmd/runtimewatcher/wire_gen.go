// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package runtimewatcher

import (
	"github.com/google/wire"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/services/events"
	"github.com/topfreegames/maestro/internal/core/services/workers"
	"github.com/topfreegames/maestro/internal/core/worker"
	config2 "github.com/topfreegames/maestro/internal/core/worker/config"
	"github.com/topfreegames/maestro/internal/core/worker/runtimewatcher"
	"github.com/topfreegames/maestro/internal/service"
)

// Injectors from wire.go:

func initializeRuntimeWatcher(c config.Config) (*workers.WorkersManager, error) {
	workerBuilder := provideRuntimeWatcherBuilder()
	schedulerStorage, err := service.NewSchedulerStoragePg(c)
	if err != nil {
		return nil, err
	}
	runtime, err := service.NewRuntimeKubernetes(c)
	if err != nil {
		return nil, err
	}
	roomStorage, err := service.NewRoomStorageRedis(c)
	if err != nil {
		return nil, err
	}
	clock := service.NewClockTime()
	portAllocator, err := service.NewPortAllocatorRandom(c)
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
	eventsService := events.NewEventsForwarderService(eventsForwarder, schedulerStorage, gameRoomInstanceStorage, roomStorage, schedulerCache, eventsForwarderConfig)
	roomManagerConfig, err := service.NewRoomManagerConfig(c)
	if err != nil {
		return nil, err
	}
	roomManager := service.NewRoomManager(clock, portAllocator, roomStorage, schedulerStorage, gameRoomInstanceStorage, runtime, eventsService, roomManagerConfig)
	runtimeWatcherConfig := provideRuntimeWatcherConfig(c)
	workerOptions := &worker.WorkerOptions{
		Runtime:              runtime,
		RoomStorage:          roomStorage,
		RoomManager:          roomManager,
		RuntimeWatcherConfig: runtimeWatcherConfig,
	}
	workersManager := workers.NewWorkersManager(workerBuilder, c, schedulerStorage, workerOptions)
	return workersManager, nil
}

// wire.go:

func provideRuntimeWatcherBuilder() *worker.WorkerBuilder {
	return &worker.WorkerBuilder{
		Func:          runtimewatcher.NewRuntimeWatcherWorker,
		ComponentName: runtimewatcher.WorkerName,
	}
}

func provideRuntimeWatcherConfig(c config.Config) *config2.RuntimeWatcherConfig {
	return &config2.RuntimeWatcherConfig{
		DisruptionWorkerIntervalSeconds: c.GetDuration("runtimeWatcher.disruptionWorker.intervalSeconds"),
		DisruptionSafetyPercentage:      c.GetFloat64("runtimeWatcher.disruptionWorker.safetyPercentage"),
	}
}

var WorkerOptionsSet = wire.NewSet(service.NewRuntimeKubernetes, service.NewRoomStorageRedis, RoomManagerSet,
	provideRuntimeWatcherConfig, wire.Struct(new(worker.WorkerOptions), "Runtime", "RoomStorage", "RoomManager", "RuntimeWatcherConfig"))

var RoomManagerSet = wire.NewSet(service.NewSchedulerStoragePg, service.NewClockTime, service.NewPortAllocatorRandom, service.NewGameRoomInstanceStorageRedis, service.NewSchedulerCacheRedis, service.NewRoomManagerConfig, service.NewRoomManager, service.NewEventsForwarder, events.NewEventsForwarderService, service.NewEventsForwarderServiceConfig)
