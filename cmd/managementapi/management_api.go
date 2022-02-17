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

package managementapi

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/cmd/commom"

	"github.com/spf13/cobra"

	"net/http"

	"go.uber.org/zap"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/monitoring"
)

var (
	logConfig  string
	configPath string
)

var ManagementApiCmd = &cobra.Command{
	Use:     "management-api",
	Short:   "Starts maestro management-api service component",
	Example: "maestro start management-api -c config.yaml -l production",
	Long: "Starts maestro management-api service component, a component that provides a REST API and a GRPC service for" +
		"managing schedulers, rooms, and operations",
	Run: func(cmd *cobra.Command, args []string) {
		runManagementApi()
	},
}

func init() {
	ManagementApiCmd.Flags().StringVarP(&logConfig, "log-config", "l", "development", "preset of configurations used by the logs. possible values are \"development\" or \"production\".")
	ManagementApiCmd.Flags().StringVarP(&configPath, "config-path", "c", "config/management-api.local.yaml", "path of the configuration YAML file")
}

func runManagementApi() {
	ctx, cancelFn := context.WithCancel(context.Background())

	err, config, shutdownInternalServerFn := commom.ServiceSetup(ctx, cancelFn, logConfig, configPath)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("unable to setup service")
	}

	mux, err := initializeManagementMux(ctx, config)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to initialize management mux")
	}
	shutdownManagementServerFn := runManagementServer(ctx, config, mux)

	<-ctx.Done()

	err = shutdownInternalServerFn()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to shutdown metrics server")
	}

	err = shutdownManagementServerFn()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to shutdown management server")
	}
}

// runManagementServer starts HTTP server in other goroutine, and returns a
// shutdown function. It serves management API endpoints/handlers.
func runManagementServer(ctx context.Context, configs config.Config, mux *runtime.ServeMux) func() error {
	// Prometheus go-http-metrics middleware
	mdlw := middleware.New(middleware.Config{
		Recorder: metrics.NewRecorder(metrics.Config{
			DurationBuckets: monitoring.DefBucketsMs,
		}),
	})
	muxWithMetrics := std.Handler("", mdlw, mux)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", configs.GetString("management_api.port")),
		Handler: muxWithMetrics,
	}

	go func() {
		zap.L().Info(fmt.Sprintf("started HTTP management server at :%s", configs.GetString("management_api.port")))
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			zap.L().With(zap.Error(err)).Fatal("failed to start HTTP management server")
		}
	}()

	return func() error {
		shutdownCtx, cancelShutdownFn := context.WithTimeout(context.Background(), configs.GetDuration("management_api.gracefulShutdownTimeout"))
		defer cancelShutdownFn()

		zap.L().Info("stopping HTTP management server")
		return httpServer.Shutdown(shutdownCtx)
	}
}