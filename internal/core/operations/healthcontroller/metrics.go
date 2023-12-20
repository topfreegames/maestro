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

	roomsWithTerminationTimeout = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "rooms_with_termination_timeout",
		Help:      "Number of rooms that extrapolate the graceful termination period",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
		},
	})

	roomsProperlyTerminated = monitoring.CreateGaugeMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemWorker,
		Name:      "rooms_properly_terminated",
		Help:      "Number of rooms that were properly terminated",
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

func reportRoomsWithTerminationTimeout(game, scheduler string, rooms int) {
	roomsWithTerminationTimeout.WithLabelValues(game, scheduler).Set(float64(rooms))
}

func reportRoomsProperlyTerminated(game, scheduler string, rooms int) {
	roomsProperlyTerminated.WithLabelValues(game, scheduler).Set(float64(rooms))
}
