package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/topfreegames/maestro/internal/config/viper"

	"github.com/topfreegames/maestro/internal/service"
	"go.uber.org/zap"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/monitoring"
)

var (
	logConfig  = flag.String("log-config", "development", "preset of configurations used by the logs. possible values are \"development\" or \"production\".")
	configPath = flag.String("config-path", "config/local.yaml", "path of the configuration YAML file")
)

func main() {
	flag.Parse()
	service.ConfigureLogging(*logConfig)

	ctx, cancelFn := context.WithCancel(context.Background())

	config, err := viper.NewViperConfig(*configPath)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("unabled to load config")
	}

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		<-sigs
		zap.L().Info("received termination")

		cancelFn()
	}()

	shutdownInternalServerFn := service.RunInternalServer(ctx, config)

	mux, err := initializeManagementMux(ctx)
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
