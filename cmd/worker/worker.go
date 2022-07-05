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

package worker

import (
	"context"

	"github.com/topfreegames/maestro/cmd/commom"
	"github.com/topfreegames/maestro/internal/core/workers"

	"github.com/spf13/cobra"
	"github.com/topfreegames/maestro/internal/core/workers/operation_execution_worker"
	"go.uber.org/zap"
)

var (
	logConfig  string
	configPath string
)

var WorkerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Starts maestro worker service component",
	Long: "Starts maestro worker service component, a service that manages workers that executes operations for a " +
		"set of schedulers. This component does not expose any means of external communication",
	Example: "maestro start worker -c config.yaml -l production",
	Run: func(cmd *cobra.Command, args []string) {
		runWorker()
	},
}

func init() {
	WorkerCmd.Flags().StringVarP(&logConfig, "log-config", "l", "production", "preset of configurations used by the logs. possible values are \"development\" or \"production\".")
	WorkerCmd.Flags().StringVarP(&configPath, "config-path", "c", "config/config.yaml", "path of the configuration YAML file")
}

func runWorker() {
	ctx, cancelFn := context.WithCancel(context.Background())

	err, config, shutdownInternalServerFn := commom.ServiceSetup(ctx, cancelFn, logConfig, configPath)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("unable to setup service")
	}

	// TODO(gabrielcorado): support multiple workers.
	workerBuilder := &workers.WorkerBuilder{
		Func:          operation_execution_worker.NewOperationExecutionWorker,
		ComponentName: operation_execution_worker.WorkerName,
	}
	operationExecutionWorkerManager, err := initializeWorker(config, workerBuilder)
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
