package workers_manager

import (
	"github.com/topfreegames/maestro/internal/core/monitoring"
)

var (
	currentWorkersGaugeMetricOpts = monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "current_workers",
		Help:      "Current number of alive workers",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	}
	currentWorkersGaugeMetric = monitoring.CreateGaugeMetric(&currentWorkersGaugeMetricOpts)

	restartedWorkersCounterMetricOpts = monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "restarted_workers",
		Help:      "Number of restarted workers",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	}
	restartedWorkersCounterMetric = monitoring.CreateCounterMetric(&restartedWorkersCounterMetricOpts)

	workersSyncCounterMetricOpts = monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "workers_sync",
		Help:      "Times of the workers sync processes",
		Labels:    []string{},
	}
	workersSyncCounterMetric = monitoring.CreateCounterMetric(&workersSyncCounterMetricOpts).WithLabelValues()
)

func ReportWorkerStart(schedulerName string) {
	currentWorkersGaugeMetric.WithLabelValues(schedulerName).Inc()
}

func ReportWorkerStop(schedulerName string) {
	currentWorkersGaugeMetric.WithLabelValues(schedulerName).Dec()
}

func ReportWorkerRestart(schedulerName string) {
	restartedWorkersCounterMetric.WithLabelValues(schedulerName).Inc()
}

func ReportWorkersSynced() {
	workersSyncCounterMetric.Inc()
}
