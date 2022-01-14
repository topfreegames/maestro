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

package events_forwarder

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
			monitoring.LabelScheduler,
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
			monitoring.LabelScheduler,
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
			monitoring.LabelScheduler,
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
			monitoring.LabelScheduler,
		},
	})
)

func reportRoomEventForwardingSuccess(schedulerName string) {
	successRoomEventForwardingMetric.WithLabelValues(schedulerName).Inc()
}

func reportPlayerEventForwardingSuccess(schedulerName string) {
	successPlayerEventForwardingMetric.WithLabelValues(schedulerName).Inc()
}

func reportRoomEventForwardingFailed(schedulerName string) {
	failedRoomEventForwardingMetric.WithLabelValues(schedulerName).Inc()
}

func reportPlayerEventForwardingFailed(schedulerName string) {
	failedPlayerEventForwardingMetric.WithLabelValues(schedulerName).Inc()
}
