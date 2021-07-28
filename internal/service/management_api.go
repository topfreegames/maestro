package service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/monitoring"
	"github.com/topfreegames/maestro/internal/handlers"
	api "github.com/topfreegames/maestro/protogen/api/v1"
	"go.uber.org/zap"
)

func ProvideManagementMux(ctx context.Context, pingHandler *handlers.PingHandler) *runtime.ServeMux {
	mux := runtime.NewServeMux()
	api.RegisterPingHandlerServer(ctx, mux, pingHandler)

	return mux
}

// RunManagementServer starts HTTP server in other goroutine, and returns a
// shutdown function. It serves management API endpoints/handlers.
func RunManagementServer(ctx context.Context, configs config.Config, mux *runtime.ServeMux) func() error {
	if !configs.GetBool("management_api.enabled") {
		return func() error { return nil }
	}

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
