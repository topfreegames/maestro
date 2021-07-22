package monitoring

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	dto "github.com/prometheus/client_model/go"
)

// DefBucketsMs is similar to prometheus.DefBuckets, but tailored for milliseconds instead of seconds.
var (
	DefBucketsMs = []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}
)

type MetricOpts struct {
	Namespace string
	Subsystem string
	Name      string
	Help      string
	Labels    []string
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
			Buckets:   DefBucketsMs,
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

func ReportLatencyMetricInMillis(metric *prometheus.HistogramVec, start time.Time, labels ...string) {
	timeElapsed := time.Since(start)

	metric.
		WithLabelValues(labels...).
		Observe(float64(timeElapsed.Nanoseconds() / int64(time.Millisecond)))
}

func FilterMetric(metrics []*dto.MetricFamily, metricName string) *dto.MetricFamily {
	for _, m := range metrics {
		if m.GetName() == metricName {
			return m
		}
	}
	return nil
}
