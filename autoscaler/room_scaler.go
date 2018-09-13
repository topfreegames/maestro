// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package autoscaler

import (
	"math"

	"github.com/topfreegames/maestro/models"
)

// RoomUsagePolicy comprehend the methods necessary to autoscale according to room usage
type RoomUsagePolicy struct{}

func newRoomUsagePolicy() *RoomUsagePolicy {
	return &RoomUsagePolicy{}
}

// CalculateDelta returns the room delta to scale up or down in order to maintain the usage percentage
func (sp *RoomUsagePolicy) CalculateDelta(trigger *models.ScalingPolicyMetricsTrigger, roomCount *models.RoomsStatusCount) int {
	// [Occupied / (Total + Delta)] = Usage/100
	occupied := float64(roomCount.Occupied)
	total := float64(roomCount.Available())
	threshold := float64(trigger.Usage) / 100
	delta := occupied - threshold*total
	delta = delta / threshold

	return int(math.Round(float64(delta)))
}

// GetCurrentUtilization returns the current usage percentage
func (sp *RoomUsagePolicy) GetCurrentUtilization(roomCount *models.RoomsStatusCount) float32 {
	return float32(roomCount.Occupied) / float32(roomCount.Available())
}
