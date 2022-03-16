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
			monitoring.LabelScheduler,
		},
	})

	gameRoomPendingGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "gru_pending",
		Help:      "The number of game rooms with status pending",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	})

	gameRoomUnreadyGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "gru_unready",
		Help:      "The number of game rooms with status unready",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	})

	gameRoomTerminatingGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "gru_terminating",
		Help:      "The number of game rooms with status terminating",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	})
	gameRoomErrorGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "gru_error",
		Help:      "The number of game rooms with status error",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	})
	gameRoomOccupiedGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "gru_occupied",
		Help:      "The number of game rooms with status occupied",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	})

	instanceReadyGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "instance_ready",
		Help:      "The number of instances with status ready",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	})

	instancePendingGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "instance_pending",
		Help:      "The number of instances with status pending",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	})

	instanceUnknownGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "instance_unknown",
		Help:      "The number of instances with status unknown",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	})

	instanceTerminatingGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "instance_terminating",
		Help:      "The number of instances with status terminating",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	})
	instanceErrorGaugeMetric = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "instance_error",
		Help:      "The number of instances with status error",
		Labels: []string{
			monitoring.LabelScheduler,
		},
	})
)

func reportGameRoomReadyNumber(schedulerName string, numberOfGameRooms int) {
	gameRoomReadyGaugeMetric.WithLabelValues(schedulerName).Set(float64(numberOfGameRooms))
}
func reportGameRoomPendingNumber(schedulerName string, numberOfGameRooms int) {
	gameRoomPendingGaugeMetric.WithLabelValues(schedulerName).Set(float64(numberOfGameRooms))
}
func reportGameRoomUnreadyNumber(schedulerName string, numberOfGameRooms int) {
	gameRoomUnreadyGaugeMetric.WithLabelValues(schedulerName).Set(float64(numberOfGameRooms))
}
func reportGameRoomTerminatingNumber(schedulerName string, numberOfGameRooms int) {
	gameRoomTerminatingGaugeMetric.WithLabelValues(schedulerName).Set(float64(numberOfGameRooms))
}
func reportGameRoomErrorNumber(schedulerName string, numberOfGameRooms int) {
	gameRoomErrorGaugeMetric.WithLabelValues(schedulerName).Set(float64(numberOfGameRooms))
}
func reportGameRoomOccupiedNumber(schedulerName string, numberOfGameRooms int) {
	gameRoomOccupiedGaugeMetric.WithLabelValues(schedulerName).Set(float64(numberOfGameRooms))
}

func reportInstanceReadyNumber(schedulerName string, numberOfInstances int) {
	instanceReadyGaugeMetric.WithLabelValues(schedulerName).Set(float64(numberOfInstances))
}

func reportInstancePendingNumber(schedulerName string, numberOfInstances int) {
	instancePendingGaugeMetric.WithLabelValues(schedulerName).Set(float64(numberOfInstances))
}

func reportInstanceUnknownNumber(schedulerName string, numberOfInstances int) {
	instanceUnknownGaugeMetric.WithLabelValues(schedulerName).Set(float64(numberOfInstances))
}

func reportInstanceTerminatingNumber(schedulerName string, numberOfInstances int) {
	instanceTerminatingGaugeMetric.WithLabelValues(schedulerName).Set(float64(numberOfInstances))
}
func reportInstanceErrorNumber(schedulerName string, numberOfInstances int) {
	instanceErrorGaugeMetric.WithLabelValues(schedulerName).Set(float64(numberOfInstances))
}
