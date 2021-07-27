package service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/topfreegames/maestro/internal/config"
	"go.uber.org/zap"
)

// RunInternalServer start a internal server in other goroutine, and returns a
// shutdown function. The internal server contains handlers for:
// - health check
// - metrics
func RunInternalServer(ctx context.Context, configs config.Config) func() error {
	if !configs.GetBool("internal_api.enabled") {
		return func() error { return nil }
	}

	mux := http.NewServeMux()
	if configs.GetBool("internal_api.healthcheck.enabled") {
		zap.L().Info("adding healthcheck handler to internal API")
		mux.HandleFunc("/health", handleHealth)
		mux.HandleFunc("/healthz", handleHealth)
	}
	if configs.GetBool("internal_api.metrics.enabled") {
		zap.L().Info("adding metrics handler to internal API")
		mux.Handle("/metrics", promhttp.Handler())
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", configs.GetString("internal_api.port")),
		Handler: mux,
	}

	go func() {
		zap.L().Info(fmt.Sprintf("started HTTP internal at :%s", configs.GetString("internal_api.port")))
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			zap.L().With(zap.Error(err)).Fatal("failed to start HTTP internal server")
		}
	}()

	return func() error {
		shutdownCtx, cancelShutdownFn := context.WithTimeout(context.Background(), configs.GetDuration("internal_api.gracefulShutdownTimeout"))
		defer cancelShutdownFn()

		zap.L().Info("stopping HTTP internal server")
		return httpServer.Shutdown(shutdownCtx)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
