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

// AutoScaler type is the struct that holds a map of autoscaling policies and usage data sources
// organized by autoscaling policy types. It is used to make autoscaling generic to work with
// multiple autoscaling algorithms.
type AutoScaler struct {
	AutoScalingPoliciesMap map[models.AutoScalingPolicyType]AutoScalingPolicy
}

// NewAutoScaler returns a Autoscaler instance
func NewAutoScaler(usageDataSource ...interface{}) *AutoScaler {
	a := &AutoScaler{
		AutoScalingPoliciesMap: map[models.AutoScalingPolicyType]AutoScalingPolicy{
			models.LegacyAutoScalingPolicyType: &LegacyUsagePolicy{}, // legacyPolicy
			models.RoomAutoScalingPolicyType:   &RoomUsagePolicy{},   // roomUsagePolicy
		},
	}
	return a
}

// Delta fetches the correct policy on the AutoScaler policies map and calculate the scaling delta
func (a *AutoScaler) Delta(
	trigger *models.ScalingPolicyMetricsTrigger,
	roomCount *models.RoomsStatusCount,
) int {
	return a.AutoScalingPoliciesMap[trigger.Type].CalculateDelta(trigger, roomCount)
}

// CurrentUsagePercentage fetches the correct policy on the AutoScaler policies map and returns the current usage percentage
func (a *AutoScaler) CurrentUsagePercentage(
	trigger *models.ScalingPolicyMetricsTrigger,
	roomCount *models.RoomsStatusCount,
) float32 {
	return a.AutoScalingPoliciesMap[trigger.Type].GetUsagePercentage(roomCount)
}

// AutoScalingPolicies implementations
// ------------------------------------------------------------

// LegacyUsagePolicy implements the legacy autoscaler
type LegacyUsagePolicy struct{}

// CalculateDelta returns the room delta to scale up or down in order to maintain the usage percentage
func (sp *LegacyUsagePolicy) CalculateDelta(trigger *models.ScalingPolicyMetricsTrigger, roomCount *models.RoomsStatusCount) int {
	return trigger.Delta
}

// GetUsagePercentage returns the current usage percentage
func (sp *LegacyUsagePolicy) GetUsagePercentage(roomCount *models.RoomsStatusCount) float32 {
	return float32(roomCount.Occupied) / float32(roomCount.Total())
}

// RoomUsagePolicy comprehend the methods necessary to autoscale according to room usage
type RoomUsagePolicy struct{}

// CalculateDelta returns the room delta to scale up or down in order to maintain the usage percentage
func (sp *RoomUsagePolicy) CalculateDelta(trigger *models.ScalingPolicyMetricsTrigger, roomCount *models.RoomsStatusCount) int {
	// [Occupied / (Total + Delta)] = Usage/100
	occupied := float64(roomCount.Occupied)
	total := float64(roomCount.Total())
	threshold := float64(trigger.Usage) / 100
	delta := occupied - threshold*total
	delta = delta / threshold

	return int(math.Round(float64(delta)))
}

// GetUsagePercentage returns the current usage percentage
func (sp *RoomUsagePolicy) GetUsagePercentage(roomCount *models.RoomsStatusCount) float32 {
	return float32(roomCount.Occupied) / float32(roomCount.Total())
}
