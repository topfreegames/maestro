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
func RunInternalServer(ctx context.Context, configs config.Config, healthDeps *HealthDependencies) func() error {
	mux := http.NewServeMux()
	if configs.GetBool("internalApi.healthcheck.enabled") {
		zap.L().Info("adding healthcheck handler to internal API")
		if configs.GetBool("internalApi.healthcheck.skipPostgresCheck") {
			zap.L().Warn("PostgreSQL health check is DISABLED - /readyz will not check database connectivity")
		}
		mux.HandleFunc("/health", handleHealth)
		mux.HandleFunc("/healthz", handleHealth)
		mux.HandleFunc("/readyz", handleReadyz(configs, healthDeps))
	}
	if configs.GetBool("internalApi.metrics.enabled") {
		zap.L().Info("adding metrics handler to internal API")
		mux.Handle("/metrics", promhttp.Handler())
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", configs.GetString("internalApi.port")),
		Handler: mux,
	}

	go func() {
		zap.L().Info(fmt.Sprintf("started HTTP internal at :%s", configs.GetString("internalApi.port")))
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			zap.L().With(zap.Error(err)).Fatal("failed to start HTTP internal server")
		}
	}()

	return func() error {
		shutdownCtx, cancelShutdownFn := context.WithTimeout(context.Background(), configs.GetDuration("internalApi.gracefulShutdownTimeout"))
		defer cancelShutdownFn()

		zap.L().Info("stopping HTTP internal server")
		return httpServer.Shutdown(shutdownCtx)
	}
}

func handleReadyz(c config.Config, deps *HealthDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		healthy := true

		// Skip PostgreSQL check if configured
		skipPostgresCheck := c.GetBool("internalApi.healthcheck.skipPostgresCheck")

		if !skipPostgresCheck && deps.PostgresDB != nil {
			if err := checkPostgres(r.Context(), deps.PostgresDB); err != nil {
				healthy = false
			}
		}

		if deps.RedisClient != nil {
			if err := checkRedis(r.Context(), deps.RedisClient); err != nil {
				healthy = false
			}
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		if !healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("Service Unavailable"))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
