package operation_execution_worker

import (
	"fmt"
	"time"

	"github.com/topfreegames/maestro/internal/core/monitoring"
)

const (
	LabelNoOperationExecutorFound = "no_operation_executor_found"
	LabelShouldNotExecute = "should_not_execute"
	LabelNextOperationFailed = "next_operation_failed"
	LabelStartOperationFailed = "start_operation_failed"
)

var (
	operationExecutionMetricOpts = monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "operation_execution",
		Help:      "An scheduler operation was executed",
		Buckets:   monitoring.DefBucketsMs,
		Labels: []string{
			monitoring.LabelScheduler,
			monitoring.LabelOperation,
			monitoring.LabelSuccess,
		},
	}
	operationExecutionLatencyMetric = monitoring.CreateLatencyMetric(&operationExecutionMetricOpts)

	operationOnErrorMetricOpts = monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "operation_on_error",
		Help:      "An scheduler operation on error fallback was executed",
		Buckets:   monitoring.DefBucketsMs,
		Labels: []string{
			monitoring.LabelScheduler,
			monitoring.LabelOperation,
			monitoring.LabelSuccess,
		},
	}
	operationOnErrorLatencyMetric = monitoring.CreateLatencyMetric(&operationOnErrorMetricOpts)

	operationEvictedCountMetricOpts = monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "operation_evicted",
		Help:      "An scheduler operation was evicted",
		Buckets:   monitoring.DefBucketsMs,
		Labels: []string{
			monitoring.LabelScheduler,
			monitoring.LabelOperation,
			monitoring.LabelReason,
		},
	}
	operationEvictedCountMetric = monitoring.CreateCounterMetric(&operationEvictedCountMetricOpts)

	operationExecutionWorkerFailedCountMetricOpts = monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "operation_execution_worker_failed",
		Help:      "An scheduler operation execution worker failed and is no longer running",
		Buckets:   monitoring.DefBucketsMs,
		Labels: []string{
			monitoring.LabelScheduler,
			monitoring.LabelReason,
		},
	}
	operationExecutionWorkerFailedCountMetric = monitoring.CreateCounterMetric(&operationExecutionWorkerFailedCountMetricOpts)
)

func ReportOperationExecutionLatency(start time.Time, schedulerName, operationName string, success bool) {
	successLabelValue := fmt.Sprint(success)
	monitoring.ReportLatencyMetric(
		operationExecutionLatencyMetric, start, schedulerName, operationName, successLabelValue,
	)
}

func ReportOperationOnErrorLatency(start time.Time, schedulerName, operationName string, success bool) {
	successLabelValue := fmt.Sprint(success)
	monitoring.ReportLatencyMetric(
		operationOnErrorLatencyMetric, start, schedulerName, operationName, successLabelValue,
	)
}

func ReportOperationEvicted(schedulerName, operationName, reason string) {
	operationEvictedCountMetric.WithLabelValues(schedulerName, operationName, reason).Inc()
}

func ReportOperationExecutionWorkerFailed(schedulerName, reason string) {
	operationExecutionWorkerFailedCountMetric.WithLabelValues(schedulerName, reason).Inc()
}
