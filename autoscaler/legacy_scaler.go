// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package autoscaler

import (
	"github.com/topfreegames/maestro/models"
)

// LegacyUsagePolicy implements the legacy autoscaler
type LegacyUsagePolicy struct{}

func newLegacyUsagePolicy() *LegacyUsagePolicy {
	return &LegacyUsagePolicy{}
}

// CalculateDelta returns the room delta to scale up or down in order to maintain the usage percentage
func (sp *LegacyUsagePolicy) CalculateDelta(trigger *models.ScalingPolicyMetricsTrigger, roomCount *models.RoomsStatusCount) int {
	return trigger.Delta
}

// GetCurrentUtilization returns the current usage percentage
func (sp *LegacyUsagePolicy) GetCurrentUtilization(roomCount *models.RoomsStatusCount) float32 {
	return float32(roomCount.Occupied) / float32(roomCount.Available())
}
