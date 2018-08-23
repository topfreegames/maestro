// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package watcher

import (
	"fmt"

	"github.com/topfreegames/maestro/models"
)

func dynamicDelta(
	metricTrigger *models.ScalingPolicyMetricsTrigger,
	roomCount *models.RoomsStatusCount,
) (int, error) {
	switch metricTrigger.Metric {
	case "room":
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
	usageThreshold := float32(metricTrigger.Usage) / 100
	return int(float32(float32(roomCount.Occupied)-float32(usageThreshold*float32(roomCount.Total()))) / usageThreshold)
}
