// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import "k8s.io/api/core/v1"

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

// GetAvailablePolicyTypes returns an array of available policy types
func GetAvailablePolicyTypes() []AutoScalingPolicyType {
	return []AutoScalingPolicyType{
		LegacyAutoScalingPolicyType,
		RoomAutoScalingPolicyType,
		CPUAutoScalingPolicyType,
		MemAutoScalingPolicyType,
	}
}

// ValidPolicyType checks if a given string is a valid autoscaling policy type
func ValidPolicyType(s string) bool {
	for _, p := range GetAvailablePolicyTypes() {
		if s == string(p) {
			return true
		}
	}
	return false
}

// ResourcePolicyType returns a bool indicating if a given policy type has resourcess associated to it or not
func ResourcePolicyType(policyType AutoScalingPolicyType) bool {
	switch policyType {
	case CPUAutoScalingPolicyType:
		return true
	case MemAutoScalingPolicyType:
		return true
	default:
		return false
	}
}

// GetResourceUsage returns the resource usage given a kubernetes resource and a policy type
func GetResourceUsage(usage v1.ResourceList, policyType AutoScalingPolicyType) int64 {
	switch policyType {
	case CPUAutoScalingPolicyType:
		return usage.Cpu().ScaledValue(-3)
	case MemAutoScalingPolicyType:
		return usage.Memory().ScaledValue(0)
	default:
		return 0
	}
}
