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

package runtimewatcher

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/topfreegames/maestro/internal/config/viper"
	"github.com/topfreegames/maestro/internal/service"
	"github.com/topfreegames/maestro/internal/validations"
	"go.uber.org/zap"
)

var (
	logConfig  string
	configPath string
)

var RuntimeWatcherCmd = &cobra.Command{
	Use:   "runtime-watcher",
	Short: "Starts maestro runtime-watcher service component",
	Long: "Starts maestro runtime-watcher service component, a service that monitors the runtime of the cluster," +
		"this component does not expose any means of external communication",
	Example: "maestro start runtime-watcher -c config.yaml -l production",
	Run: func(cmd *cobra.Command, args []string) {
		runRuntimeWatcher()
	},
}

func init() {
	RuntimeWatcherCmd.Flags().StringVarP(&logConfig, "log-config", "l", "development", "preset of configurations used by the logs. possible values are \"development\" or \"production\".")
	RuntimeWatcherCmd.Flags().StringVarP(&configPath, "config-path", "c", "config/runtime-watcher.local.yaml", "path of the configuration YAML file")
}

func runRuntimeWatcher() {
	err := service.ConfigureLogging(logConfig)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("unable to load logging configuration")
	}

	err = validations.RegisterValidations()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal(err.Error())
	}

	ctx, cancelFn := context.WithCancel(context.Background())

	config, err := viper.NewViperConfig(configPath)
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

	operationExecutionWorkerManager, err := initializeRuntimeWatcher(config)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to initialize operation execution worker manager")
	}

	go func() {
		zap.L().Info("operation execution worker manager initialized, starting...")
		err := operationExecutionWorkerManager.Start(ctx)
		if err != nil {
			zap.L().With(zap.Error(err)).Info("operation execution worker manager stopped with error")
			// enforce the cancellation
			cancelFn()
		}
	}()

	<-ctx.Done()

	err = shutdownInternalServerFn()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to shutdown internal server")
	}
}
