package service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/topfreegames/maestro/internal/config"
	"go.uber.org/zap"
)

// runMetricsServer start a metrics server in other goroutine, and returns a
// shutdown function.
func RunMetricsServer(ctx context.Context, configs config.Config) func() error {
	if !configs.GetBool("metrics.enabled") {
		return func() error { return nil }
	}

	mux := http.NewServeMux()
	mux.Handle("/", promhttp.Handler())

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", configs.GetString("metrics.port")),
		Handler: mux,
	}

	go func() {
		zap.L().Info(fmt.Sprintf("started HTTP Metrics at :%s", configs.GetString("metrics.port")))
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			zap.L().With(zap.Error(err)).Fatal("failed to start HTTP metrics server")
		}
	}()

	return func() error {
		shutdownCtx, cancelShutdownFn := context.WithTimeout(context.Background(), configs.GetDuration("metrics.gracefulShutdownTimeout"))
		defer cancelShutdownFn()

		zap.L().Info("stopping HTTP metrics server")
		return httpServer.Shutdown(shutdownCtx)
	}
}
