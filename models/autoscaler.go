// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

// AutoScalingPolicyType defines all types of autoscaling policies available
type AutoScalingPolicyType string

const (
	// LegacyAutoScalingPolicyType defines legacy usage autoscaling policy type
	LegacyAutoScalingPolicyType AutoScalingPolicyType = "legacy"
	// RoomAutoScalingPolicyType defines room usage autoscaling policy type
	RoomAutoScalingPolicyType AutoScalingPolicyType = "room"
	// CPUAutoScalingPolicyType defines CPU usage autoscaling policy type
	CPUAutoScalingPolicyType AutoScalingPolicyType = "cpu"
	// MemAutoScalingPolicyType defines memory usage autoscaling policy type
	MemAutoScalingPolicyType AutoScalingPolicyType = "mem"
)
