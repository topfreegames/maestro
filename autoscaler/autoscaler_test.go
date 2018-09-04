// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package autoscaler_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/topfreegames/maestro/autoscaler"
	"github.com/topfreegames/maestro/models"
)

var _ = Describe("AutoScaler", func() {

	Describe("NewAutoScaler", func() {
		It("should return configured new autoscaler", func() {
			expectedAutoScaler := &AutoScaler{
				AutoScalingPoliciesMap: map[models.AutoScalingPolicyType]AutoScalingPolicy{
					models.LegacyAutoScalingPolicyType: &LegacyUsagePolicy{}, // LegacyUsagePolicy
					models.RoomAutoScalingPolicyType:   &RoomUsagePolicy{},   // RoomUsagePolicy
				},
			}

			autoScaler := NewAutoScaler()
			Expect(autoScaler).To(Equal(expectedAutoScaler))
		})
	})

	Describe("AutoScaler", func() {
		var autoScaler *AutoScaler
		var trigger *models.ScalingPolicyMetricsTrigger
		var roomCount *models.RoomsStatusCount

		BeforeEach(func() {
			autoScaler = NewAutoScaler()
			trigger = &models.ScalingPolicyMetricsTrigger{
				Time:      100,
				Usage:     70,
				Threshold: 80,
				Delta:     100,
			}
			roomCount = &models.RoomsStatusCount{
				Creating:    0,
				Ready:       2,
				Occupied:    8,
				Terminating: 0,
			}
		})

		Context("legacy type", func() {
			BeforeEach(func() {
				trigger.Type = models.LegacyAutoScalingPolicyType
			})

			It("should return delta", func() {
				delta := autoScaler.Delta(trigger, roomCount)
				Expect(delta).To(Equal(100))
			})

			It("should get current usage percentage", func() {
				usagePercentage := autoScaler.CurrentUsagePercentage(trigger, roomCount)
				Expect(fmt.Sprintf("%.1f", usagePercentage)).To(Equal("0.8"))
			})
		})

		Context("room type", func() {
			BeforeEach(func() {
				trigger.Type = models.RoomAutoScalingPolicyType
			})

			It("should return delta", func() {
				delta := autoScaler.Delta(trigger, roomCount)
				Expect(delta).To(Equal(1))
			})

			It("should get current usage percentage", func() {
				usagePercentage := autoScaler.CurrentUsagePercentage(trigger, roomCount)
				Expect(fmt.Sprintf("%.1f", usagePercentage)).To(Equal("0.8"))
			})
		})
	})
})
