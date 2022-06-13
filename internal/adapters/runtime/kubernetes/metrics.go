package kubernetes

import "github.com/topfreegames/maestro/internal/core/monitoring"

var (
	watcherInstanceConversionFailCounterMetric = monitoring.CreateCounterMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWatcher,
		Name:      "failed_instance_conversion",
		Help:      "Amount of instances conversions failed",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	})
)

func reportInstanceConversionFailed(schedulerName string) {
	watcherInstanceConversionFailCounterMetric.WithLabelValues(schedulerName).Inc()
}
