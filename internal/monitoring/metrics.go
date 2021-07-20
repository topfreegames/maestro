package monitoring

import (
	"time"

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

func CreateGaugeMetric(options *MetricOpts) prometheus.Gauge {
	return promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: options.Namespace,
			Subsystem: options.Subsystem,
			Name:      options.Name + "_gauge",
			Help:      options.Help + " (gauge)",
		},
	)
}

func ReportLatencyMetric(metric *prometheus.HistogramVec, start time.Time, labels ...string) {
	timeElapsed := time.Since(start)

	metric.
		WithLabelValues(labels...).
		Observe(float64(timeElapsed.Nanoseconds() / int64(time.Millisecond)))
}
