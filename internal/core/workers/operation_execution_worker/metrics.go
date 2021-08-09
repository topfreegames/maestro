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

package operation_execution_worker

import (
	"fmt"
	"time"

	"github.com/topfreegames/maestro/internal/core/monitoring"
)

const (
	LabelNoOperationExecutorFound = "no_operation_executor_found"
	LabelShouldNotExecute         = "should_not_execute"
	LabelNextOperationFailed      = "next_operation_failed"
	LabelStartOperationFailed     = "start_operation_failed"
)

var (
	operationExecutionLatencyMetric = monitoring.CreateLatencyMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "operation_execution",
		Help:      "An scheduler operation was executed",
		Labels: []string{
			monitoring.LabelScheduler,
			monitoring.LabelOperation,
			monitoring.LabelSuccess,
		},
	})

	operationOnErrorLatencyMetric = monitoring.CreateLatencyMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "operation_on_error",
		Help:      "An scheduler operation on error fallback was executed",
		Labels: []string{
			monitoring.LabelScheduler,
			monitoring.LabelOperation,
			monitoring.LabelSuccess,
		},
	})

	operationEvictedCountMetric = monitoring.CreateCounterMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "operation_evicted",
		Help:      "An scheduler operation was evicted",
		Labels: []string{
			monitoring.LabelScheduler,
			monitoring.LabelOperation,
			monitoring.LabelReason,
		},
	})

	operationExecutionWorkerFailedCountMetric = monitoring.CreateCounterMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "operation_execution_worker_failed",
		Help:      "An scheduler operation execution worker failed and is no longer running",
		Labels: []string{
			monitoring.LabelScheduler,
			monitoring.LabelReason,
		},
	})
)

func reportOperationExecutionLatency(start time.Time, schedulerName, operationName string, success bool) {
	successLabelValue := fmt.Sprint(success)
	monitoring.ReportLatencyMetricInMillis(
		operationExecutionLatencyMetric, start, schedulerName, operationName, successLabelValue,
	)
}

func reportOperationOnErrorLatency(start time.Time, schedulerName, operationName string, success bool) {
	successLabelValue := fmt.Sprint(success)
	monitoring.ReportLatencyMetricInMillis(
		operationOnErrorLatencyMetric, start, schedulerName, operationName, successLabelValue,
	)
}

func reportOperationEvicted(schedulerName, operationName, reason string) {
	operationEvictedCountMetric.WithLabelValues(schedulerName, operationName, reason).Inc()
}

func reportOperationExecutionWorkerFailed(schedulerName, reason string) {
	operationExecutionWorkerFailedCountMetric.WithLabelValues(schedulerName, reason).Inc()
}
