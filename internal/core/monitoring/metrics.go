package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/topfreegames/maestro/internal/config"
	"go.uber.org/zap"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type MetricOpts struct {
	Namespace  string
	Subsystem  string
	Name       string
	Help       string
	Labels     []string
	Buckets    []float64
	Objectives map[float64]float64
}

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

	return func() error { return httpServer.Shutdown(ctx) }
}

func CreateCounterMetric(options *MetricOpts) *prometheus.CounterVec {
	return promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: options.Namespace,
			Subsystem: options.Subsystem,
			Name:      options.Name + "_counter",
			Help:      options.Help + " (counter)",
		},
		options.Labels,
	)
}

func CreateLatencyMetric(options *MetricOpts) *prometheus.HistogramVec {
	return promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: options.Namespace,
			Subsystem: options.Subsystem,
			Name:      options.Name + "_latency",
			Help:      options.Help + " (latency)",
			Buckets:   options.Buckets,
		},
		options.Labels,
	)
}

func CreateGaugeMetric(options *MetricOpts) *prometheus.GaugeVec {
	return promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: options.Namespace,
			Subsystem: options.Subsystem,
			Name:      options.Name + "_gauge",
			Help:      options.Help + " (gauge)",
		},
		options.Labels,
	)
}

func ReportLatencyMetric(metric *prometheus.HistogramVec, start time.Time, labels ...string) {
	timeElapsed := time.Since(start)

	metric.
		WithLabelValues(labels...).
		Observe(float64(timeElapsed.Nanoseconds() / int64(time.Millisecond)))
}
