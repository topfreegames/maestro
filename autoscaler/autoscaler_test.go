// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package autoscaler

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	uuid "github.com/satori/go.uuid"

	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/testing"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("AutoScaler", func() {

	Describe("NewAutoScaler", func() {
		It("should return configured new autoscaler", func() {
			expectedAutoScaler := &AutoScaler{
				AutoScalingPoliciesMap: map[models.AutoScalingPolicyType]AutoScalingPolicy{
					// LegacyUsagePolicy
					models.LegacyAutoScalingPolicyType: &LegacyUsagePolicy{},
					// RoomUsagePolicy
					models.RoomAutoScalingPolicyType: &RoomUsagePolicy{},
					// CPUUsagePolicy
					models.CPUAutoScalingPolicyType: &ResourceUsagePolicy{
						Clientset:                       clientset,
						MetricsClientset:                metricsClientset,
						SchedulerName:                   schedulerName,
						ResourceGetUsageAndRequestsFunc: getCPUUsageAndRequests,
					},
					// MemUsagePolicy
					models.MemAutoScalingPolicyType: &ResourceUsagePolicy{
						Clientset:                       clientset,
						MetricsClientset:                metricsClientset,
						SchedulerName:                   schedulerName,
						ResourceGetUsageAndRequestsFunc: getMemUsageAndRequests,
					},
				},
			}

			autoScaler := NewAutoScaler(schedulerName, clientset, metricsClientset)

			Expect(len(autoScaler.AutoScalingPoliciesMap)).
				To(Equal(4))

			Expect(autoScaler.AutoScalingPoliciesMap[models.LegacyAutoScalingPolicyType]).
				To(Equal(expectedAutoScaler.AutoScalingPoliciesMap[models.LegacyAutoScalingPolicyType]))

			Expect(autoScaler.AutoScalingPoliciesMap[models.RoomAutoScalingPolicyType]).
				To(Equal(expectedAutoScaler.AutoScalingPoliciesMap[models.RoomAutoScalingPolicyType]))

			Expect(autoScaler.AutoScalingPoliciesMap[models.CPUAutoScalingPolicyType]).
				To(BeAssignableToTypeOf(expectedAutoScaler.AutoScalingPoliciesMap[models.CPUAutoScalingPolicyType]))

			Expect(autoScaler.AutoScalingPoliciesMap[models.MemAutoScalingPolicyType]).
				To(BeAssignableToTypeOf(expectedAutoScaler.AutoScalingPoliciesMap[models.MemAutoScalingPolicyType]))

			Expect(autoScaler.AutoScalingPoliciesMap[models.CPUAutoScalingPolicyType].(*ResourceUsagePolicy).Clientset).
				To(BeAssignableToTypeOf(clientset))

			Expect(autoScaler.AutoScalingPoliciesMap[models.CPUAutoScalingPolicyType].(*ResourceUsagePolicy).MetricsClientset).
				To(BeAssignableToTypeOf(metricsClientset))
		})
	})

	Describe("AutoScaler", func() {
		var containerName string
		var autoScaler *AutoScaler
		var trigger *models.ScalingPolicyMetricsTrigger
		var roomCount *models.RoomsStatusCount

		Context("legacy type", func() {
			BeforeEach(func() {
				autoScaler = NewAutoScaler(schedulerName, clientset, metricsClientset)
				trigger = &models.ScalingPolicyMetricsTrigger{
					Time:      100,
					Usage:     70,
					Threshold: 80,
					Delta:     100,
					Type:      models.LegacyAutoScalingPolicyType,
				}
				roomCount = &models.RoomsStatusCount{
					Creating:    0,
					Ready:       2,
					Occupied:    8,
					Terminating: 0,
				}
			})

			It("should return delta", func() {
				delta := autoScaler.Delta(trigger, roomCount)
				Expect(delta).To(Equal(100))
			})

			It("should get current usage percentage", func() {
				usagePercentage := autoScaler.CurrentUtilization(trigger, roomCount)
				Expect(fmt.Sprintf("%.1f", usagePercentage)).To(Equal("0.8"))
			})
		})

		Context("room type", func() {
			BeforeEach(func() {
				autoScaler = NewAutoScaler(schedulerName, clientset, metricsClientset)
				trigger = &models.ScalingPolicyMetricsTrigger{
					Time:      100,
					Usage:     70,
					Threshold: 80,
					Delta:     100,
					Type:      models.RoomAutoScalingPolicyType,
				}
				roomCount = &models.RoomsStatusCount{
					Creating:    0,
					Ready:       2,
					Occupied:    8,
					Terminating: 0,
				}
			})

			It("should return delta", func() {
				delta := autoScaler.Delta(trigger, roomCount)
				Expect(delta).To(Equal(1))
			})

			It("should get current usage percentage", func() {
				usagePercentage := autoScaler.CurrentUtilization(trigger, roomCount)
				Expect(fmt.Sprintf("%.1f", usagePercentage)).To(Equal("0.8"))
			})
		})

		Context("cpu type", func() {
			BeforeEach(func() {
				containerName = fmt.Sprintf("%s-%s", schedulerName, uuid.NewV4().String())
				trigger = &models.ScalingPolicyMetricsTrigger{
					Time:      100,
					Usage:     70,
					Threshold: 80,
					Delta:     100,
					Type:      models.CPUAutoScalingPolicyType,
				}
				roomCount = &models.RoomsStatusCount{
					Creating:    0,
					Ready:       2,
					Occupied:    8,
					Terminating: 0,
				}
				testing.CreatePod(clientset, "1.0", "0", schedulerName, containerName)
			})

			It("should return delta", func() {
				// scale down
				containerMetrics := testing.BuildContainerMetricsArray(
					[]testing.ContainerMetricsDefinition{
						testing.ContainerMetricsDefinition{
							Name: containerName,
							Usage: map[models.AutoScalingPolicyType]int{
								models.CPUAutoScalingPolicyType: 500,
								models.MemAutoScalingPolicyType: 0,
							},
							MemScale: 0,
						},
					},
				)
				fakeMetricsClient := testing.CreatePodsMetricsList(containerMetrics, 1, schedulerName)
				autoScaler := NewAutoScaler(schedulerName, clientset, fakeMetricsClient)

				delta := autoScaler.Delta(trigger, roomCount)
				Expect(delta).To(Equal(-2))

				// scale up
				containerMetrics = testing.BuildContainerMetricsArray(
					[]testing.ContainerMetricsDefinition{
						testing.ContainerMetricsDefinition{
							Name: containerName,
							Usage: map[models.AutoScalingPolicyType]int{
								models.CPUAutoScalingPolicyType: 900,
								models.MemAutoScalingPolicyType: 0,
							},
							MemScale: 0,
						},
					},
				)
				fakeMetricsClient = testing.CreatePodsMetricsList(containerMetrics, 1, schedulerName)
				autoScaler = NewAutoScaler(schedulerName, clientset, fakeMetricsClient)

				delta = autoScaler.Delta(trigger, roomCount)
				Expect(delta).To(Equal(3))
			})

			It("should get current usage percentage", func() {
				containerMetrics := testing.BuildContainerMetricsArray(
					[]testing.ContainerMetricsDefinition{
						testing.ContainerMetricsDefinition{
							Name: containerName,
							Usage: map[models.AutoScalingPolicyType]int{
								models.CPUAutoScalingPolicyType: 500,
								models.MemAutoScalingPolicyType: 0,
							},
							MemScale: 0,
						},
					},
				)
				fakeMetricsClient := testing.CreatePodsMetricsList(containerMetrics, 1, schedulerName)
				autoScaler := NewAutoScaler(schedulerName, clientset, fakeMetricsClient)

				usagePercentage := autoScaler.CurrentUtilization(trigger, roomCount)
				Expect(fmt.Sprintf("%.1f", usagePercentage)).To(Equal("0.5"))
			})
		})

		Context("mem type", func() {
			BeforeEach(func() {
				containerName = fmt.Sprintf("%s-%s", schedulerName, uuid.NewV4().String())
				trigger = &models.ScalingPolicyMetricsTrigger{
					Time:      100,
					Usage:     70,
					Threshold: 80,
					Delta:     100,
					Type:      models.MemAutoScalingPolicyType,
				}
				roomCount = &models.RoomsStatusCount{
					Creating:    0,
					Ready:       2,
					Occupied:    8,
					Terminating: 0,
				}

				testing.CreatePod(clientset, "0", "1Gi", schedulerName, containerName)
			})

			It("should return delta", func() {
				// scale down
				containerMetrics := testing.BuildContainerMetricsArray(
					[]testing.ContainerMetricsDefinition{
						testing.ContainerMetricsDefinition{
							Name: containerName,
							Usage: map[models.AutoScalingPolicyType]int{
								models.CPUAutoScalingPolicyType: 0,
								models.MemAutoScalingPolicyType: 600,
							},
							MemScale: resource.Mega,
						},
					},
				)
				fakeMetricsClient := testing.CreatePodsMetricsList(containerMetrics, 1, schedulerName)
				autoScaler := NewAutoScaler(schedulerName, clientset, fakeMetricsClient)

				delta := autoScaler.Delta(trigger, roomCount)
				Expect(delta).To(Equal(-2))

				// scale up
				containerMetrics = testing.BuildContainerMetricsArray(
					[]testing.ContainerMetricsDefinition{
						testing.ContainerMetricsDefinition{
							Name: containerName,
							Usage: map[models.AutoScalingPolicyType]int{
								models.CPUAutoScalingPolicyType: 0,
								models.MemAutoScalingPolicyType: 950,
							},
							MemScale: resource.Mega,
						},
					},
				)
				fakeMetricsClient = testing.CreatePodsMetricsList(containerMetrics, 1, schedulerName)
				autoScaler = NewAutoScaler(schedulerName, clientset, fakeMetricsClient)

				delta = autoScaler.Delta(trigger, roomCount)
				Expect(delta).To(Equal(3))
			})

			It("should get current usage percentage", func() {
				containerMetrics := testing.BuildContainerMetricsArray(
					[]testing.ContainerMetricsDefinition{
						testing.ContainerMetricsDefinition{
							Name: containerName,
							Usage: map[models.AutoScalingPolicyType]int{
								models.CPUAutoScalingPolicyType: 0,
								models.MemAutoScalingPolicyType: 500,
							},
							MemScale: resource.Mega,
						},
					},
				)
				fakeMetricsClient := testing.CreatePodsMetricsList(containerMetrics, 1, schedulerName)
				autoScaler := NewAutoScaler(schedulerName, clientset, fakeMetricsClient)

				usagePercentage := autoScaler.CurrentUtilization(trigger, roomCount)
				Expect(fmt.Sprintf("%.1f", usagePercentage)).To(Equal("0.5"))
			})
		})
	})
})
