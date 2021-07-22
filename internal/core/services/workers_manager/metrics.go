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
		Labels:    []string{},
	}
	currentWorkersGaugeMetric = monitoring.CreateGaugeMetric(&currentWorkersGaugeMetricOpts).WithLabelValues()

	workersSyncCounterMetricOpts = monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "workers_sync",
		Help:      "Times of the workers sync processes",
		Labels:    []string{},
	}
	workersSyncCounterMetric = monitoring.CreateCounterMetric(&workersSyncCounterMetricOpts).WithLabelValues()
)

func ReportWorkerStarted() {
	currentWorkersGaugeMetric.Inc()
}

func ReportWorkerStopped() {
	currentWorkersGaugeMetric.Dec()
}

func ReportWorkersSynced() {
	workersSyncCounterMetric.Inc()
}
