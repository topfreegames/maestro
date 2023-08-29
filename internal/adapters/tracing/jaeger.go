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

package tracing

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
)

const (
	tracingDisabledPath  = "api.tracing.jaeger.disabled"
	tracingAgentHostPath = "api.tracing.jaeger.agent_host"
	tracingAgentPortPath = "api.tracing.jaeger.agent_port"
	tracingSamplerPath   = "api.tracing.jaeger.sampler"
)

func ConfigureTracing(serviceName string, cfg config.Config) (func() error, error) {
	if IsTracingEnabled(cfg) {
		return configureJaeger(serviceName, cfg)
	}

	return func() error { return nil }, nil
}

func IsTracingEnabled(cfg config.Config) bool {
	return !cfg.GetBool(tracingDisabledPath)
}

func configureJaeger(serviceName string, configs config.Config) (func() error, error) {
	res := buildResource(serviceName)
	provider := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithSampler(trace.TraceIDRatioBased(configs.GetFloat64(tracingSamplerPath))),
	)

	endpointOptions := jaeger.WithAgentEndpoint(
		jaeger.WithAgentHost(configs.GetString(tracingAgentHostPath)),
		jaeger.WithAgentPort(configs.GetString(tracingAgentPortPath)),
	)

	exp, err := jaeger.New(endpointOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create jager collector: %w", err)
	}

	bsp := trace.NewBatchSpanProcessor(exp)
	provider.RegisterSpanProcessor(bsp)

	otel.SetTracerProvider(provider)

	return func() error {
		shutdownCtx, cancelShutdownFn := context.WithTimeout(context.Background(), configs.GetDuration("api.gracefulShutdownTimeout"))
		defer cancelShutdownFn()

		if err := provider.Shutdown(shutdownCtx); err != nil {
			return err
		}

		return nil
	}, nil
}

func buildResource(serviceName string) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNamespaceKey.String(serviceName),
		semconv.ServiceNameKey.String("maestro-next"),
	)
}
