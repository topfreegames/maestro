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

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/topfreegames/maestro/internal/config/viper"
	"github.com/topfreegames/maestro/internal/core/workers/operation_execution_worker"
	"github.com/topfreegames/maestro/internal/service"
	"go.uber.org/zap"
)

var (
	logConfig  = flag.String("log-config", "development", "preset of configurations used by the logs. possible values are \"development\" or \"production\".")
	configPath = flag.String("config-path", "config/worker.local.yaml", "path of the configuration YAML file")
)

func main() {
	flag.Parse()
	err := service.ConfigureLogging(*logConfig)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("unable to load logging configuration")
	}

	ctx, cancelFn := context.WithCancel(context.Background())

	config, err := viper.NewViperConfig(*configPath)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("unable to load config")
	}

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		<-sigs
		zap.L().Info("received termination")

		cancelFn()
	}()

	shutdownInternalServerFn := service.RunInternalServer(ctx, config)

	// TODO(gabrielcorado): support multiple workers.
	operationExecutionWorkerManager, err := initializeWorker(config, operation_execution_worker.NewOperationExecutionWorker)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to initialize operation execution worker manager")
	}

	go func() {
		zap.L().Info("starting operation cancellation request watcher")
		err = operationExecutionWorkerManager.WorkerOptions.OperationManager.WatchOperationCancellationRequests(ctx)
		if err != nil {
			zap.L().With(zap.Error(err)).Info("operation cancellation watcher stopped with error")
			// enforce the cancellation
			cancelFn()
		}
		zap.L().Info("operation cancellation request watcher stopped")
	}()

	go func() {
		zap.L().Info("operation execution worker manager initialized, starting...")
		err := operationExecutionWorkerManager.Start(ctx)
		if err != nil {
			zap.L().With(zap.Error(err)).Info("operation execution worker manager stopped with error")
			// enforce the cancellation
			cancelFn()
		}
		zap.L().Info("operation execution worker manager stopped")
	}()

	<-ctx.Done()

	err = shutdownInternalServerFn()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to shutdown internal server")
	}
}
