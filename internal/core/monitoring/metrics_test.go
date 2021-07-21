//+build unit

package monitoring

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestCounterCreation(t *testing.T) {

	t.Run("successfully fetch counter metric", func(t *testing.T) {

		counter := CreateCounterMetric(&MetricOpts{
			Namespace: "maestro",
			Subsystem: "test",
			Name:      "counter_test",
			Help:      "Test Counter",
			Labels:    []string{"success"},
		})

		counter.WithLabelValues("true").Inc()

		metrics, _ := prometheus.DefaultGatherer.Gather()
		metricFamily := filterMetric(metrics, "maestro_test_counter_test_counter")
		trueMetric := metricFamily.GetMetric()[0]
		require.Equal(t, float64(1), trueMetric.GetCounter().GetValue())

		trueLabel := trueMetric.GetLabel()[0]
		require.Equal(t, "success", trueLabel.GetName())
		require.Equal(t, "true", trueLabel.GetValue())

		counter.WithLabelValues("false").Inc()
		counter.WithLabelValues("true").Inc()

		metrics, _ = prometheus.DefaultGatherer.Gather()
		metricFamily = filterMetric(metrics, "maestro_test_counter_test_counter")

		trueMetric = metricFamily.GetMetric()[1]
		require.Equal(t, float64(2), trueMetric.GetCounter().GetValue())

		falseMetric := metricFamily.GetMetric()[0]
		require.Equal(t, float64(1), falseMetric.GetCounter().GetValue())

		falseLabel := falseMetric.GetLabel()[0]
		require.Equal(t, "success", falseLabel.GetName())
		require.Equal(t, "false", falseLabel.GetValue())

		trueLabel = trueMetric.GetLabel()[0]
		require.Equal(t, "success", trueLabel.GetName())
		require.Equal(t, "true", trueLabel.GetValue())

	})

	t.Run("successfully fetch gauge metric", func(t *testing.T) {

		gauge := CreateGaugeMetric(&MetricOpts{
			Namespace: "maestro",
			Subsystem: "test",
			Name:      "gauge_test",
			Help:      "Test Gauge",
			Labels:    []string{"success"},
		})

		gauge.WithLabelValues("true").Inc()

		metrics, _ := prometheus.DefaultGatherer.Gather()
		metricFamily := filterMetric(metrics, "maestro_test_gauge_test_gauge")
		trueMetric := metricFamily.GetMetric()[0]
		require.Equal(t, float64(1), trueMetric.GetGauge().GetValue())

		trueLabel := trueMetric.GetLabel()[0]
		require.Equal(t, "success", trueLabel.GetName())
		require.Equal(t, "true", trueLabel.GetValue())

		gauge.WithLabelValues("false").Inc()
		gauge.WithLabelValues("true").Dec()

		metrics, _ = prometheus.DefaultGatherer.Gather()
		metricFamily = filterMetric(metrics, "maestro_test_gauge_test_gauge")

		trueMetric = metricFamily.GetMetric()[1]
		require.Equal(t, float64(0), trueMetric.GetGauge().GetValue())

		falseMetric := metricFamily.GetMetric()[0]
		require.Equal(t, float64(1), falseMetric.GetGauge().GetValue())

		falseLabel := falseMetric.GetLabel()[0]
		require.Equal(t, "success", falseLabel.GetName())
		require.Equal(t, "false", falseLabel.GetValue())

		trueLabel = trueMetric.GetLabel()[0]
		require.Equal(t, "success", trueLabel.GetName())
		require.Equal(t, "true", trueLabel.GetValue())

	})

	t.Run("successfully fetch latency metric", func(t *testing.T) {

		latency := CreateLatencyMetric(&MetricOpts{
			Namespace: "maestro",
			Subsystem: "test",
			Name:      "latency_test",
			Help:      "Test Latency",
			Labels:    []string{"success"},
		})

		ReportLatencyMetricInMillis(latency, time.Now().Add(time.Second*-1), "true")

		metrics, _ := prometheus.DefaultGatherer.Gather()
		metricFamily := filterMetric(metrics, "maestro_test_latency_test_latency")
		trueMetric := metricFamily.GetMetric()[0]
		require.Equal(t, uint64(1), trueMetric.GetHistogram().GetSampleCount())
		require.Equal(t, float64(1000), trueMetric.GetHistogram().GetSampleSum())

		trueLabel := trueMetric.GetLabel()[0]
		require.Equal(t, "success", trueLabel.GetName())
		require.Equal(t, "true", trueLabel.GetValue())

		ReportLatencyMetricInMillis(latency, time.Now().Add(time.Second*-2), "false")
		ReportLatencyMetricInMillis(latency, time.Now().Add(time.Second*-5), "true")

		metrics, _ = prometheus.DefaultGatherer.Gather()
		metricFamily = filterMetric(metrics, "maestro_test_latency_test_latency")
		trueMetric = metricFamily.GetMetric()[1]
		require.Equal(t, uint64(2), trueMetric.GetHistogram().GetSampleCount())
		require.Equal(t, float64(6000), trueMetric.GetHistogram().GetSampleSum())

		trueLabel = trueMetric.GetLabel()[0]
		require.Equal(t, "success", trueLabel.GetName())
		require.Equal(t, "true", trueLabel.GetValue())

		falseMetric := metricFamily.GetMetric()[0]
		require.Equal(t, uint64(1), falseMetric.GetHistogram().GetSampleCount())
		require.Equal(t, float64(2000), falseMetric.GetHistogram().GetSampleSum())

		falseLabel := falseMetric.GetLabel()[0]
		require.Equal(t, "success", falseLabel.GetName())
		require.Equal(t, "false", falseLabel.GetValue())
	})

}

func filterMetric(metrics []*dto.MetricFamily, metricName string) *dto.MetricFamily {
	for _, m := range metrics {
		if m.GetName() == metricName {
			return m
		}
	}
	return nil
}
