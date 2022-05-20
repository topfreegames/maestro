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

package monitoring

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	dto "github.com/prometheus/client_model/go"
)

var (
	// DefBucketsMs is similar to prometheus.DefBuckets, but tailored for milliseconds instead of seconds.
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
