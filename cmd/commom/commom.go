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

package commom

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/config/viper"
	"github.com/topfreegames/maestro/internal/service"
	"github.com/topfreegames/maestro/internal/validations"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.uber.org/zap"
)

func ServiceSetup(ctx context.Context, cancelFn context.CancelFunc, logConfig, configPath string) (error, config.Config, func() error) {
	err := service.ConfigureLogging(logConfig)
	if err != nil {
		return fmt.Errorf("unable to configure logging: %w", err), nil, nil
	}

	err = validations.RegisterValidations()
	if err != nil {
		return fmt.Errorf("unable to register validations: %w", err), nil, nil
	}

	viperConfig, err := viper.NewViperConfig(configPath)
	if err != nil {
		return fmt.Errorf("unable to load config: %w", err), nil, nil
	}

	launchTerminatingListenerGoroutine(cancelFn)

	shutdownInternalServerFn := service.RunInternalServer(ctx, viperConfig)

	return nil, viperConfig, shutdownInternalServerFn
}

func launchTerminatingListenerGoroutine(cancelFunc context.CancelFunc) {
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		<-sigs
		zap.L().Info("received termination")

		cancelFunc()
	}()
}

func MatchPath(path, pattern string) bool {
	match, err := regexp.MatchString(pattern, path)
	if err != nil {
		return false
	}
	return match
}

func ConfigureTracer(ctx context.Context, serviceName string) (func() error, error) {
	tracerUrl := os.Getenv("OTEL_EXPORTER_JAEGER_ENDPOINT")
	switch {
	case tracerUrl != "":
		return configureJaeger(ctx, serviceName)
	default:
		return configureStdout(ctx, serviceName)
	}
}

func configureJaeger(ctx context.Context, serviceName string) (func() error, error) {
	res := buildResource(serviceName)
	provider := trace.NewTracerProvider(trace.WithResource(res))

	exp, err := jaeger.New(jaeger.WithCollectorEndpoint())
	if err != nil {
		return nil, fmt.Errorf("failed to create jager collector: %w", err)
	}

	bsp := trace.NewBatchSpanProcessor(exp)
	provider.RegisterSpanProcessor(bsp)

	otel.SetTracerProvider(provider)

	return func() error {
		if err := provider.Shutdown(ctx); err != nil {
			return err
		}

		return nil
	}, nil
}

func configureStdout(ctx context.Context, serviceName string) (func() error, error) {
	res := buildResource(serviceName)
	provider := trace.NewTracerProvider(trace.WithResource(res))
	otel.SetTracerProvider(provider)

	exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	bsp := trace.NewBatchSpanProcessor(exp)
	provider.RegisterSpanProcessor(bsp)

	return func() error {
		if err := provider.Shutdown(ctx); err != nil {
			return err
		}

		return nil
	}, nil
}

func buildResource(serviceName string) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNamespaceKey.String("maestro-next"),
		semconv.ServiceNameKey.String(serviceName),
	)
}
