//+build integration

package monitoring

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
)

func TestCounterCreation(t *testing.T) {

	t.Run("successfully fetch counter metric", func(t *testing.T) {

		req, err := http.NewRequest("GET", "/metrics", nil)
		if err != nil {
			t.Fatal(err)
		}

		counter := CreateCounterMetric(&MetricOpts{
			Namespace: "maestro",
			Subsystem: "test",
			Name:      "counter_test",
			Help:      "Test Counter",
			Labels:    []string{"success"},
		})

		counter.WithLabelValues("true").Inc()

		rr := httptest.NewRecorder()
		promhttp.Handler().ServeHTTP(rr, req)

		require.Equal(t, rr.Code, http.StatusOK)
		require.Contains(t, rr.Body.String(), "maestro_test_counter_test_counter{success=\"true\"} 1")

		counter.WithLabelValues("false").Inc()
		counter.WithLabelValues("true").Inc()

		rr = httptest.NewRecorder()
		promhttp.Handler().ServeHTTP(rr, req)

		require.Equal(t, rr.Code, http.StatusOK)
		require.Contains(t, rr.Body.String(), "maestro_test_counter_test_counter{success=\"true\"} 2")
		require.Contains(t, rr.Body.String(), "maestro_test_counter_test_counter{success=\"false\"} 1")
	})

	t.Run("successfully fetch gauge metric", func(t *testing.T) {

		req, err := http.NewRequest("GET", "/metrics", nil)
		if err != nil {
			t.Fatal(err)
		}

		gauge := CreateGaugeMetric(&MetricOpts{
			Namespace: "maestro",
			Subsystem: "test",
			Name:      "gauge_test",
			Help:      "Test Gauge",
			Labels:    []string{"success"},
		})

		gauge.WithLabelValues("true").Inc()

		rr := httptest.NewRecorder()
		promhttp.Handler().ServeHTTP(rr, req)

		require.Equal(t, rr.Code, http.StatusOK)
		require.Contains(t, rr.Body.String(), "maestro_test_gauge_test_gauge{success=\"true\"} 1")

		gauge.WithLabelValues("false").Inc()
		gauge.WithLabelValues("true").Dec()

		rr = httptest.NewRecorder()
		promhttp.Handler().ServeHTTP(rr, req)

		require.Equal(t, rr.Code, http.StatusOK)
		require.Contains(t, rr.Body.String(), "maestro_test_gauge_test_gauge{success=\"true\"} 0")
		require.Contains(t, rr.Body.String(), "maestro_test_gauge_test_gauge{success=\"false\"} 1")
	})

	t.Run("successfully fetch latency metric", func(t *testing.T) {

		req, err := http.NewRequest("GET", "/metrics", nil)
		if err != nil {
			t.Fatal(err)
		}

		latency := CreateLatencyMetric(&MetricOpts{
			Namespace: "maestro",
			Subsystem: "test",
			Name:      "latency_test",
			Help:      "Test Latency",
			Labels:    []string{"success"},
		})

		ReportLatencyMetric(latency, time.Now().Add(time.Second*-1), "true")

		rr := httptest.NewRecorder()
		promhttp.Handler().ServeHTTP(rr, req)

		require.Equal(t, rr.Code, http.StatusOK)
		require.Contains(t, rr.Body.String(), "# HELP maestro_test_latency_test_latency Test Latency (latency)")
		require.Contains(t, rr.Body.String(), "# TYPE maestro_test_latency_test_latency histogram")
		require.Contains(t, rr.Body.String(), "maestro_test_latency_test_latency_bucket{success=\"true\",le=\"+Inf\"} 1")
		require.Contains(t, rr.Body.String(), "maestro_test_latency_test_latency_sum{success=\"true\"} 1000")
		require.Contains(t, rr.Body.String(), "maestro_test_latency_test_latency_count{success=\"true\"} 1")

		ReportLatencyMetric(latency, time.Now().Add(time.Second*-2), "false")
		ReportLatencyMetric(latency, time.Now().Add(time.Second*-5), "true")

		rr = httptest.NewRecorder()
		promhttp.Handler().ServeHTTP(rr, req)

		require.Equal(t, rr.Code, http.StatusOK)

		require.Contains(t, rr.Body.String(), "# HELP maestro_test_latency_test_latency Test Latency (latency)")
		require.Contains(t, rr.Body.String(), "# TYPE maestro_test_latency_test_latency histogram")
		require.Contains(t, rr.Body.String(), "maestro_test_latency_test_latency_bucket{success=\"false\",le=\"+Inf\"} 1")
		require.Contains(t, rr.Body.String(), "maestro_test_latency_test_latency_sum{success=\"false\"} 2000")
		require.Contains(t, rr.Body.String(), "maestro_test_latency_test_latency_count{success=\"false\"} 1")

		require.Contains(t, rr.Body.String(), "maestro_test_latency_test_latency_bucket{success=\"true\",le=\"+Inf\"} 2")
		require.Contains(t, rr.Body.String(), "maestro_test_latency_test_latency_sum{success=\"true\"} 6000")
		require.Contains(t, rr.Body.String(), "maestro_test_latency_test_latency_count{success=\"true\"} 2")
	})

}
