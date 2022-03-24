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

package metrics

import (
	"github.com/go-redis/redis/v8"
	"github.com/topfreegames/maestro/internal/core/monitoring"
	"time"
)

var (
	RedisFailsCounterMetric = monitoring.CreateCounterMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "redis_fails",
		Help:      "Redis fails counter metric",
		Labels: []string{
			monitoring.LabelStorage,
		},
	})

	RedisLatencyMetric = monitoring.CreateLatencyMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "redis_operation",
		Help:      "Redis latency metric",
		Labels: []string{
			monitoring.LabelStorage,
		},
	})
)

func RunWithMetrics(storage string, executionFunction func() error) {
	start := time.Now()
	err := executionFunction()
	if err != nil && err != redis.Nil {
		ReportOperationFlowStorageFailsCounterMetric(storage)
	}
	monitoring.ReportLatencyMetricInMillis(
		RedisLatencyMetric, start, storage,
	)
}

func ReportOperationFlowStorageFailsCounterMetric(storage string) {
	RedisFailsCounterMetric.WithLabelValues(storage).Inc()
}
