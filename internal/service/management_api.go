package service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/topfreegames/maestro/internal/config"
	"go.uber.org/zap"
)

// RunManagementServer starts HTTP server in other goroutine, and returns a
// shutdown function. It serves management API endpoints/handlers.
func RunManagementServer(ctx context.Context, configs config.Config, handlers map[string]http.Handler) func() error {
	if !configs.GetBool("management_api.enabled") {
		return func() error { return nil }
	}

	mux := http.NewServeMux()
	for path, handler := range handlers {
		mux.Handle(path, handler)
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", configs.GetString("management_api.port")),
		Handler: mux,
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
