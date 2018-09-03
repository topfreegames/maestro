// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package watcher

import (
	"fmt"
	"math"

	"github.com/topfreegames/maestro/models"
)

func dynamicDelta(
	metricTrigger *models.ScalingPolicyMetricsTrigger,
	roomCount *models.RoomsStatusCount,
) (int, error) {
	switch metricTrigger.Metric {
	case models.MetricTypeRoom:
		return roomDynamicDelta(metricTrigger, roomCount), nil
	default:
		// TODO: implement default to deal with kubernetes metrics
		return 0, fmt.Errorf("Undefined metric: %s", metricTrigger.Metric)
	}
}

func roomDynamicDelta(
	metricTrigger *models.ScalingPolicyMetricsTrigger,
	roomCount *models.RoomsStatusCount,
) int { // delta
	// [Occupied / (Total + Delta)] = Usage/100
	occupied := float64(roomCount.Occupied)
	total := float64(roomCount.Total())
	threshold := float64(metricTrigger.Usage) / 100
	delta := occupied - threshold*total
	delta = delta / threshold

	return int(math.Round(float64(delta)))
}

// Get current usage
func getCurrentUsageFromMetric(
	metricTrigger *models.ScalingPolicyMetricsTrigger,
	roomCount *models.RoomsStatusCount,
) float32 {
	switch metricTrigger.Metric {
	case models.MetricTypeRoom:
		if roomCount.Total() > 0 {
			return float32(roomCount.Occupied) / float32(roomCount.Total())
		}
	}
	return 0
}
