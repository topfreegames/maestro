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

package metricsreporter

import (
	"github.com/topfreegames/maestro/internal/core/monitoring"
)

var (
	gameRoomReadyGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "gru_ready",
		Help:      "The number of game rooms with status ready",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	gameRoomPendingGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "gru_pending",
		Help:      "The number of game rooms with status pending",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	gameRoomUnreadyGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "gru_unready",
		Help:      "The number of game rooms with status unready",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	gameRoomTerminatingGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "gru_terminating",
		Help:      "The number of game rooms with status terminating",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	gameRoomErrorGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "gru_error",
		Help:      "The number of game rooms with status error",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	gameRoomOccupiedGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "gru_occupied",
		Help:      "The number of game rooms with status occupied",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})
	gameRoomActiveGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "gru_active",
		Help:      "The number of game rooms with status active, with running matches but not fully occupied",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	gameRoomAllocatedGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "gru_allocated",
		Help:      "The number of game rooms with status allocated",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	instanceReadyGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "instance_ready",
		Help:      "The number of instances with status ready",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	instancePendingGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "instance_pending",
		Help:      "The number of instances with status pending",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	instanceUnknownGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "instance_unknown",
		Help:      "The number of instances with status unknown",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	instanceTerminatingGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "instance_terminating",
		Help:      "The number of instances with status terminating",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	instanceErrorGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "instance_error",
		Help:      "The number of instances with status error",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	runningMatchesGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "running_matches",
		Help:      "The total number of running matches",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	schedulerMaxMatchesGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "max_matches",
		Help:      "The max number of matches that each Game Room can handle",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	schedulerAutoscalePolicyReadyTargetGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "ready_target",
		Help:      "Ready target configured in autoscale policy",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	schedulerAutoscalePolicyFixedBufferGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "fixed_buffer",
		Help:      "Fixed buffer amount configured in autoscale policy",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})
)

func reportGameRoomReadyNumber(game, schedulerName string, numberOfGameRooms int) {
	gameRoomReadyGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(numberOfGameRooms))
}

func reportGameRoomPendingNumber(game, schedulerName string, numberOfGameRooms int) {
	gameRoomPendingGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(numberOfGameRooms))
}

func reportGameRoomUnreadyNumber(game, schedulerName string, numberOfGameRooms int) {
	gameRoomUnreadyGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(numberOfGameRooms))
}

func reportGameRoomTerminatingNumber(game, schedulerName string, numberOfGameRooms int) {
	gameRoomTerminatingGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(numberOfGameRooms))
}

func reportGameRoomErrorNumber(game, schedulerName string, numberOfGameRooms int) {
	gameRoomErrorGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(numberOfGameRooms))
}

func reportGameRoomOccupiedNumber(game, schedulerName string, numberOfGameRooms int) {
	gameRoomOccupiedGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(numberOfGameRooms))
}
func reportGameRoomActiveNumber(game, schedulerName string, numberOfGameRooms int) {
	gameRoomActiveGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(numberOfGameRooms))
}

func reportGameRoomAllocatedNumber(game, schedulerName string, numberOfGameRooms int) {
	gameRoomAllocatedGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(numberOfGameRooms))
}

func reportInstanceReadyNumber(game, schedulerName string, numberOfInstances int) {
	instanceReadyGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(numberOfInstances))
}

func reportInstancePendingNumber(game, schedulerName string, numberOfInstances int) {
	instancePendingGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(numberOfInstances))
}

func reportInstanceUnknownNumber(game, schedulerName string, numberOfInstances int) {
	instanceUnknownGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(numberOfInstances))
}

func reportInstanceTerminatingNumber(game, schedulerName string, numberOfInstances int) {
	instanceTerminatingGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(numberOfInstances))
}

func reportInstanceErrorNumber(game, schedulerName string, numberOfInstances int) {
	instanceErrorGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(numberOfInstances))
}

func reportTotalRunningMatches(game, schedulerName string, runningMatches int) {
	runningMatchesGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(runningMatches))
}

func reportSchedulerMaxMatches(game, schedulerName string, availableSlots int) {
	schedulerMaxMatchesGaugeMetric.WithLabelValues(game, schedulerName).Set(float64(availableSlots))
}

func reportSchedulerPolicyReadyTarget(game, schedulerName string, readyTarget float64) {
	schedulerAutoscalePolicyReadyTargetGaugeMetric.WithLabelValues(game, schedulerName).Set(readyTarget)
}

func reportSchedulerPolicyFixedBuffer(game, schedulerName string, amount float64) {
	schedulerAutoscalePolicyFixedBufferGaugeMetric.WithLabelValues(game, schedulerName).Set(amount)
}
