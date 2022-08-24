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

package runtimewatcher

import (
	"github.com/google/wire"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/services/events"
	"github.com/topfreegames/maestro/internal/core/services/worker"
	"github.com/topfreegames/maestro/internal/core/workers"
	"github.com/topfreegames/maestro/internal/core/workers/runtime_watcher_worker"
	"github.com/topfreegames/maestro/internal/service"
)

func provideRuntimeWatcherBuilder() *workers.WorkerBuilder {
	return &workers.WorkerBuilder{
		Func:          runtime_watcher_worker.NewRuntimeWatcherWorker,
		ComponentName: runtime_watcher_worker.WorkerName,
	}
}

var WorkerOptionsSet = wire.NewSet(
	service.NewRuntimeKubernetes,
	RoomManagerSet,
	wire.Struct(new(workers.WorkerOptions), "RoomManager", "Runtime"))

var RoomManagerSet = wire.NewSet(
	service.NewSchedulerStoragePg,
	service.NewClockTime,
	service.NewPortAllocatorRandom,
	service.NewRoomStorageRedis,
	service.NewGameRoomInstanceStorageRedis,
	service.NewSchedulerCacheRedis,
	service.NewRoomManagerConfig,
	service.NewRoomManager,
	service.NewEventsForwarder,
	events.NewEventsForwarderService,
	service.NewEventsForwarderServiceConfig,
)

func initializeRuntimeWatcher(c config.Config) (*worker.WorkersManager, error) {
	wire.Build(
		// workers options
		WorkerOptionsSet,

		// watcher builder
		provideRuntimeWatcherBuilder,

		worker.NewWorkersManager,
	)

	return &worker.WorkersManager{}, nil
}
