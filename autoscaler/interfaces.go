// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package autoscaler

import "github.com/topfreegames/maestro/models"

//AutoScalingPolicy is a contract for multiple autoscaling algorithms
type AutoScalingPolicy interface {
	CalculateDelta(trigger *models.ScalingPolicyMetricsTrigger, roomCount *models.RoomsStatusCount) int
	GetCurrentUtilization(roomCount *models.RoomsStatusCount) float32
}
