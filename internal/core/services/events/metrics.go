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

package events

import (
	"github.com/topfreegames/maestro/internal/core/monitoring"
)

var (
	successRoomEventForwardingMetric = monitoring.CreateCounterMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemApi,
		Name:      "success_room_event_forwarding",
		Help:      "Current number of room events forwarding with success",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
			monitoring.LabelCode,
		},
	})
)

var (
	successPlayerEventForwardingMetric = monitoring.CreateCounterMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemApi,
		Name:      "success_player_event_forwarding",
		Help:      "Current number of player events forwarding with success",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
			monitoring.LabelCode,
		},
	})
)

var (
	failedRoomEventForwardingMetric = monitoring.CreateCounterMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemApi,
		Name:      "failed_room_event_forwarding",
		Help:      "Current number of failed room events forwarding",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
			monitoring.LabelCode,
		},
	})
)

var (
	failedPlayerEventForwardingMetric = monitoring.CreateCounterMetric(&monitoring.MetricOpts{
		Namespace: monitoring.Namespace,
		Subsystem: monitoring.SubsystemApi,
		Name:      "failed_player_event_forwarding",
		Help:      "Current number of failed player events forwarding",
		Labels: []string{
			monitoring.LabelGame,
			monitoring.LabelScheduler,
			monitoring.LabelCode,
		},
	})
)

func reportRoomEventForwardingSuccess(game, schedulerName, code string) {
	successRoomEventForwardingMetric.WithLabelValues(game, schedulerName, code).Inc()
}

func reportPlayerEventForwardingSuccess(game, schedulerName, code string) {
	successPlayerEventForwardingMetric.WithLabelValues(game, schedulerName, code).Inc()
}

func reportRoomEventForwardingFailed(game, schedulerName, code string) {
	failedRoomEventForwardingMetric.WithLabelValues(game, schedulerName, code).Inc()
}

func reportPlayerEventForwardingFailed(game, schedulerName, code string) {
	failedPlayerEventForwardingMetric.WithLabelValues(game, schedulerName, code).Inc()
}
