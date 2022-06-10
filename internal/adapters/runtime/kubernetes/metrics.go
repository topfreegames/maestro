package kubernetes

import "github.com/topfreegames/maestro/internal/core/monitoring"

var (
	watcherInstanceConversionFailCounterMetric = monitoring.CreateCounterMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "workers_sync",
		Help:      "Times of the workers sync processes",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	})
)

func reportInstanceConversionFailed(schedulerName string) {
	watcherInstanceConversionFailCounterMetric.WithLabelValues(schedulerName).Inc()
}
