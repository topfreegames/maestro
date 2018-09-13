// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package autoscaler

import (
	"k8s.io/client-go/kubernetes"
	metricsClient "k8s.io/metrics/pkg/client/clientset_generated/clientset"

	"github.com/topfreegames/maestro/models"
)

// AutoScaler type is the struct that holds a map of autoscaling policies and usage data sources
// organized by autoscaling policy types. It is used to make autoscaling generic to work with
// multiple autoscaling algorithms.
type AutoScaler struct {
	AutoScalingPoliciesMap map[models.AutoScalingPolicyType]AutoScalingPolicy
}

// NewAutoScaler returns a Autoscaler instance
func NewAutoScaler(schedulerName string, usageDataSource ...interface{}) *AutoScaler {
	return &AutoScaler{
		AutoScalingPoliciesMap: map[models.AutoScalingPolicyType]AutoScalingPolicy{

			// legacyPolicy
			models.LegacyAutoScalingPolicyType: newLegacyUsagePolicy(),

			// roomUsagePolicy
			models.RoomAutoScalingPolicyType: newRoomUsagePolicy(),

			// cpuUsagePolicy
			models.CPUAutoScalingPolicyType: newCPUUsagePolicy(
				usageDataSource[0].(kubernetes.Interface),
				usageDataSource[1].(metricsClient.Interface),
				schedulerName,
			),

			// memUsagePolicy
			models.MemAutoScalingPolicyType: newMemUsagePolicy(
				usageDataSource[0].(kubernetes.Interface),
				usageDataSource[1].(metricsClient.Interface),
				schedulerName,
			),
		},
	}
}

// Delta fetches the correct policy on the AutoScaler policies map and calculate the scaling delta
func (a *AutoScaler) Delta(
	trigger *models.ScalingPolicyMetricsTrigger,
	roomCount *models.RoomsStatusCount,
) int {
	return a.AutoScalingPoliciesMap[trigger.Type].CalculateDelta(trigger, roomCount)
}

// CurrentUtilization fetches the correct policy on the AutoScaler policies map and returns the current usage percentage
func (a *AutoScaler) CurrentUtilization(
	trigger *models.ScalingPolicyMetricsTrigger,
	roomCount *models.RoomsStatusCount,
) float32 {
	return a.AutoScalingPoliciesMap[trigger.Type].GetCurrentUtilization(roomCount)
}
