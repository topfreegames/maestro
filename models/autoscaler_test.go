// maestro
//go:build unit
// +build unit

// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/topfreegames/maestro/models"
)

var _ = Describe("Autoscaler", func() {
	Describe("ValidPolicyType", func() {
		It("should return true for legacy policy type", func() {
			Expect(models.ValidPolicyType("legacy")).To(BeTrue())
		})

		It("should return true for room policy type", func() {
			Expect(models.ValidPolicyType("room")).To(BeTrue())
		})

		It("should return true for cpu policy type", func() {
			Expect(models.ValidPolicyType("cpu")).To(BeTrue())
		})

		It("should return true for mem policy type", func() {
			Expect(models.ValidPolicyType("mem")).To(BeTrue())
		})

		It("should return false if invalid policy type", func() {
			Expect(models.ValidPolicyType("asjbkdhjs")).To(BeFalse())
		})
	})

	Describe("ResourcePolicyType", func() {
		It("should return false for legacy policy type", func() {
			Expect(models.ResourcePolicyType(models.LegacyAutoScalingPolicyType)).To(BeFalse())
		})

		It("should return false for room policy type", func() {
			Expect(models.ResourcePolicyType(models.RoomAutoScalingPolicyType)).To(BeFalse())
		})

		It("should return true for cpu policy type", func() {
			Expect(models.ResourcePolicyType(models.CPUAutoScalingPolicyType)).To(BeTrue())
		})

		It("should return true for mem policy type", func() {
			Expect(models.ResourcePolicyType(models.MemAutoScalingPolicyType)).To(BeTrue())
		})
	})

	Describe("GetResourceUsage", func() {
		resourceList := v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse("500m"),
			v1.ResourceMemory: resource.MustParse("100M"),
		}

		It("should return 0 for legacy policy type", func() {
			Expect(models.GetResourceUsage(resourceList, models.LegacyAutoScalingPolicyType)).To(Equal(int64(0)))
		})

		It("should return 0 for room policy type", func() {
			Expect(models.GetResourceUsage(resourceList, models.RoomAutoScalingPolicyType)).To(Equal(int64(0)))
		})

		It("should return true for cpu policy type", func() {
			Expect(models.GetResourceUsage(resourceList, models.CPUAutoScalingPolicyType)).To(Equal(int64(500)))
		})

		It("should return true for mem policy type", func() {
			Expect(models.GetResourceUsage(resourceList, models.MemAutoScalingPolicyType)).To(Equal(int64(100000000)))
		})
	})
})
