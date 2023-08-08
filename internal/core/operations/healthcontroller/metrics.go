package healthcontroller

import "github.com/topfreegames/maestro/internal/core/monitoring"

var (
	desiredNumberOfRoomsMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "desired_rooms",
		Help:      "Desired number of rooms for a scheduler, based on the autoscaling policy",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	currentNumberOfRoomsMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "current_rooms",
		Help:      "Current number of rooms for a scheduler",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})
)

func reportDesiredNumberOfRooms(game, scheduler string, desired int) {
	desiredNumberOfRoomsMetric.WithLabelValues(game, scheduler).Set(float64(desired))
}

func reportCurrentNumberOfRooms(game, scheduler string, current int) {
	currentNumberOfRoomsMetric.WithLabelValues(game, scheduler).Set(float64(current))
}
