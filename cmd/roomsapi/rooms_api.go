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

package roomsapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"
	"github.com/spf13/cobra"
	"github.com/topfreegames/maestro/cmd/commom"
	"github.com/topfreegames/maestro/internal/config"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
)

var (
	logConfig  string
	configPath string
)

const serviceName string = "rooms-api"

var RoomsAPICmd = &cobra.Command{
	Use:   "rooms-api",
	Short: "Starts maestro rooms-api service component",
	Long: "Starts maestro rooms-api service component, a component that provides a REST API and a GRPC service for" +
		"sending rooms messages",
	Example: "maestro start rooms-api -c config.yaml -l production",
	Run: func(cmd *cobra.Command, args []string) {
		runRoomsAPI()
	},
}

func init() {
	RoomsAPICmd.Flags().StringVarP(&logConfig, "log-config", "l", "production", "preset of configurations used by the logs. possible values are \"development\" or \"production\".")
	RoomsAPICmd.Flags().StringVarP(&configPath, "config-path", "c", "config/config.yaml", "path of the configuration YAML file")
}

func runRoomsAPI() {
	ctx, cancelFn := context.WithCancel(context.Background())

	err, config, shutdownInternalServerFn := commom.ServiceSetup(ctx, cancelFn, logConfig, configPath)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("unable to setup service")
	}

	closeTracer, err := commom.ConfigureTracer(serviceName, config)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to configure tracer")
	}

	mux, err := initializeRoomsMux(ctx, config)
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to initialize rooms mux")
	}
	shutdownRoomsServerFn := runRoomsServer(config, mux)

	<-ctx.Done()

	err = closeTracer()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to shutdown tracing server")
	}

	err = shutdownInternalServerFn()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to shutdown metrics server")
	}

	err = shutdownRoomsServerFn()
	if err != nil {
		zap.L().With(zap.Error(err)).Fatal("failed to shutdown management server")
	}
}

// runRoomsServer starts HTTP server in other goroutine, and returns a
// shutdown function. It serves rooms API endpoints/handlers.
func runRoomsServer(configs config.Config, mux *runtime.ServeMux) func() error {
	// Prometheus go-http-metrics middleware
	mdlw := middleware.New(middleware.Config{
		Service: serviceName,
		Recorder: metrics.NewRecorder(metrics.Config{
			DurationBuckets: prometheus.DefBuckets,
		}),
	})

	muxHandler := buildMuxWithMetricsMdlw(mdlw, mux)

	if configs.GetBool("api.tracing.jaeger.enabled") {
		muxHandler = buildMuxWithTracing(muxHandler)
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", configs.GetString("api.port")),
		Handler: muxHandler,
	}

	go func() {
		zap.L().Info(fmt.Sprintf("started HTTP rooms server at :%s", configs.GetString("api.port")))
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			zap.L().With(zap.Error(err)).Fatal("failed to start HTTP rooms server")
		}
	}()

	return func() error {
		shutdownCtx, cancelShutdownFn := context.WithTimeout(context.Background(), configs.GetDuration("api.gracefulShutdownTimeout"))
		defer cancelShutdownFn()

		zap.L().Info("stopping HTTP rooms server")
		return httpServer.Shutdown(shutdownCtx)
	}
}

func buildMuxWithMetricsMdlw(mdlw middleware.Middleware, mux *runtime.ServeMux) http.Handler {
	muxHandlerWithMetricsMdlw := http.NewServeMux()

	// ensure we are keeping metric cardinality to a minimum
	muxHandlerWithMetricsMdlw.Handle("/", http.HandlerFunc(func(respWriter http.ResponseWriter, request *http.Request) {
		path := request.URL.Path

		var handler http.Handler
		anyWordRegex := "[^/]+?"

		switch {
		// Rooms handler
		case commom.MatchPath(path, fmt.Sprintf("^/scheduler/%s/rooms/%s/ping$", anyWordRegex, anyWordRegex)):
			handler = std.Handler("/scheduler/:schedulerName/rooms/:roomID/ping", mdlw, mux)
		case commom.MatchPath(path, fmt.Sprintf("^/scheduler/%s/rooms/%s/roomevent", anyWordRegex, anyWordRegex)):
			handler = std.Handler("/scheduler/:schedulerName/rooms/:roomID/roomevent", mdlw, mux)
		case commom.MatchPath(path, fmt.Sprintf("^/scheduler/%s/rooms/%s/playerevent", anyWordRegex, anyWordRegex)):
			handler = std.Handler("/scheduler/:schedulerName/rooms/:roomID/playerevent", mdlw, mux)
		case commom.MatchPath(path, fmt.Sprintf("^/scheduler/%s/rooms/%s/status", anyWordRegex, anyWordRegex)):
			handler = std.Handler("/scheduler/:schedulerName/rooms/:roomID/status", mdlw, mux)
		case commom.MatchPath(path, fmt.Sprintf("^/scheduler/%s/rooms/%s/address", anyWordRegex, anyWordRegex)):
			handler = std.Handler("/scheduler/:schedulerName/rooms/:roomID/address", mdlw, mux)
		default:
			handler = std.Handler("", mdlw, mux)
		}
		handler.ServeHTTP(respWriter, request)
	}),
	)
	return muxHandlerWithMetricsMdlw
}

func buildMuxWithTracing(hndl http.Handler) http.Handler {
	muxHandlerWithTracing := http.NewServeMux()
	muxHandlerWithTracing.Handle("/", http.HandlerFunc(func(respWriter http.ResponseWriter, request *http.Request) {
		path := request.URL.Path

		var handler http.Handler
		anyWordRegex := "[^/]+?"

		switch {
		// Rooms handler
		case commom.MatchPath(path, fmt.Sprintf("^/scheduler/%s/rooms/%s/ping$", anyWordRegex, anyWordRegex)):
			handler = otelhttp.NewHandler(hndl, "/scheduler/:schedulerName/rooms/:roomID/ping")
		case commom.MatchPath(path, fmt.Sprintf("^/scheduler/%s/rooms/%s/roomevent", anyWordRegex, anyWordRegex)):
			handler = otelhttp.NewHandler(hndl, "/scheduler/:schedulerName/rooms/:roomID/roomevent")
		case commom.MatchPath(path, fmt.Sprintf("^/scheduler/%s/rooms/%s/playerevent", anyWordRegex, anyWordRegex)):
			handler = otelhttp.NewHandler(hndl, "/scheduler/:schedulerName/rooms/:roomID/playerevent")
		case commom.MatchPath(path, fmt.Sprintf("^/scheduler/%s/rooms/%s/status", anyWordRegex, anyWordRegex)):
			handler = otelhttp.NewHandler(hndl, "/scheduler/:schedulerName/rooms/:roomID/status")
		case commom.MatchPath(path, fmt.Sprintf("^/scheduler/%s/rooms/%s/address", anyWordRegex, anyWordRegex)):
			handler = otelhttp.NewHandler(hndl, "/scheduler/:schedulerName/rooms/:roomID/address")
		default:
			handler = otelhttp.NewHandler(hndl, "")
		}
		handler.ServeHTTP(respWriter, request)
	}),
	)
	return muxHandlerWithTracing
}
