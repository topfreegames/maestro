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

package metricsreporter

import (
	"context"

	"github.com/topfreegames/maestro/cmd/commom"

	"github.com/spf13/cobra"

	"go.uber.org/zap"
)

var (
	logConfig  string
	configPath string
)

var MetricsReporterCmd = &cobra.Command{
	Use:   "metrics-reporter",
	Short: "Starts maestro metrics-reporter service component",
	Long: "Starts maestro metrics-reporter service component, a service that keeps collecting and reporting room " +
		"and instance metrics, this component does not expose any means of external communication",
	Example: "maestro start metrics-reporter -c config.yaml -l production",
	Run: func(cmd *cobra.Command, args []string) {
		runMetricsReporter()
	},
}

func init() {
	MetricsReporterCmd.Flags().StringVarP(&logConfig, "log-config", "l", "production", "preset of configurations used by the logs. possible values are \"development\" or \"production\".")
	MetricsReporterCmd.Flags().StringVarP(&configPath, "config-path", "c", "config/metrics-reporter.local.yaml", "path of the configuration YAML file")
}

func runMetricsReporter() {
	ctx, cancelFn := context.WithCancel(context.Background())

	err, config, shutdownInternalServerFn := commom.ServiceSetup(ctx, cancelFn, logConfig, configPath)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("unable to setup service")
	}

	metricsReporterWorkerManager, err := initializeMetricsReporter(config)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to initialize metrics reporter worker manager")
	}

	go func() {
		zap.L().Info("metrics reporter worker manager initialized, starting...")
		err := metricsReporterWorkerManager.Start(ctx)
		if err != nil {
			zap.L().With(zap.Error(err)).Info("metrics reporter worker manager stopped with error")
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
