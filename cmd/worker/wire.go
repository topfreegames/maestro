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

//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/operations/providers"
	"github.com/topfreegames/maestro/internal/core/services/events_forwarder"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"
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
		service.NewSchedulerCacheRedis,
		service.NewOperationLeaseStorageRedis,
		service.NewPortAllocatorRandom,
		service.NewRoomStorageRedis,
		service.NewGameRoomInstanceStorageRedis,
		service.NewRoomManagerConfig,
		service.NewOperationManagerConfig,
		service.NewEventsForwarder,
		service.NewEventsForwarderServiceConfig,

		// scheduler operations
		providers.ProvideDefinitionConstructors,
		providers.ProvideExecutors,

		// services
		events_forwarder.NewEventsForwarderService,
		room_manager.NewRoomManager,
		service.NewOperationManager,
		workers.ProvideWorkerOptions,
		workers_manager.NewWorkersManager,
		scheduler_manager.NewSchedulerManager,
	)

	return &workers_manager.WorkersManager{}, nil
}
