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

package metricsreporter

import (
	"github.com/google/wire"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/services/workers_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
	workerconfigs "github.com/topfreegames/maestro/internal/core/workers/config"
	"github.com/topfreegames/maestro/internal/core/workers/metricsreporter"
	"github.com/topfreegames/maestro/internal/service"
)

func provideMetricsReporterBuilder() workers.WorkerBuilder {
	return metricsreporter.NewMetricsReporterWorker
}

func provideMetricsReporterConfig(c config.Config) *workerconfigs.MetricsReporterConfig {
	return &workerconfigs.MetricsReporterConfig{MetricsReporterIntervalMillis: c.GetDuration("reporter.metrics.intervalMillis")}

}

var WorkerOptionsSet = wire.NewSet(
	service.NewRoomStorageRedis,
	service.NewGameRoomInstanceStorageRedis,
	provideMetricsReporterConfig,
	wire.Struct(new(workers.WorkerOptions), "RoomStorage", "InstanceStorage", "MetricsReporterConfig"))

func initializeMetricsReporter(c config.Config) (*workers_manager.WorkersManager, error) {
	wire.Build(
		// workers options
		WorkerOptionsSet,

		// watcher builder
		provideMetricsReporterBuilder,

		service.NewSchedulerStoragePg,

		workers_manager.NewWorkersManager,
	)

	return &workers_manager.WorkersManager{}, nil
}
