// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package autoscaler

import (
	"math"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsClient "k8s.io/metrics/pkg/client/clientset_generated/clientset"

	"github.com/topfreegames/maestro/models"
)

// ResourceUsagePolicy comprehend the methods necessary to autoscale according to resource usage (cpu, mem...)
type ResourceUsagePolicy struct {
	SchedulerName                   string
	Clientset                       kubernetes.Interface
	MetricsClientset                metricsClient.Interface
	ResourceGetUsageAndRequestsFunc func(usage, requests v1.ResourceList) (float32, float32)
	currentUtilization              float32
}

func newCPUUsagePolicy(
	clientset kubernetes.Interface,
	metricsClientset metricsClient.Interface,
	schedulerName string,
) *ResourceUsagePolicy {
	return &ResourceUsagePolicy{
		Clientset:                       clientset,
		MetricsClientset:                metricsClientset,
		SchedulerName:                   schedulerName,
		ResourceGetUsageAndRequestsFunc: getCPUUsageAndRequests,
	}
}

func getCPUUsageAndRequests(usage, requests v1.ResourceList) (float32, float32) {
	resourceRequests := float32(requests.Cpu().ScaledValue(-3))
	resourceUsage := float32(usage.Cpu().ScaledValue(-3))
	return resourceUsage, resourceRequests
}

func newMemUsagePolicy(
	clientset kubernetes.Interface,
	metricsClientset metricsClient.Interface,
	schedulerName string,
) *ResourceUsagePolicy {
	return &ResourceUsagePolicy{
		Clientset:                       clientset,
		MetricsClientset:                metricsClientset,
		SchedulerName:                   schedulerName,
		ResourceGetUsageAndRequestsFunc: getMemUsageAndRequests,
	}
}

func getMemUsageAndRequests(usage, requests v1.ResourceList) (float32, float32) {
	resourceRequests := float32(requests.Memory().ScaledValue(0))
	resourceUsage := float32(usage.Memory().ScaledValue(0))
	return resourceUsage, resourceRequests
}

// CalculateDelta returns the room delta to scale up or down in order to maintain the usage percentage
func (sp *ResourceUsagePolicy) CalculateDelta(trigger *models.ScalingPolicyMetricsTrigger, roomCount *models.RoomsStatusCount) int {
	// Delta = ceil( (currentUtilization / targetUtilization) * roomCount.Available() ) - roomCount.Available()
	if sp.currentUtilization == 0 {
		sp.GetCurrentUtilization(roomCount)
	}
	targetUtilization := float64(trigger.Usage) / 100
	usageRatio := float64(sp.currentUtilization) / targetUtilization
	usageRatio = math.Round(usageRatio*100) / 100 // round to nearest with 2 decimal places

	delta := int(math.Ceil(usageRatio*float64(roomCount.Available()))) - roomCount.Available()

	sp.currentUtilization = 0
	return delta
}

// GetCurrentUtilization returns the current usage percentage
func (sp *ResourceUsagePolicy) GetCurrentUtilization(roomCount *models.RoomsStatusCount) (currrentUtilization float32) {
	var resourceUsagePerPodList []float32
	var resourceRequestsPerPodList []float32

	podList, _ := sp.MetricsClientset.Metrics().PodMetricses(sp.SchedulerName).List(metav1.ListOptions{})

	if podList != nil {
		for _, pod := range podList.Items {
			for i, container := range pod.Containers {
				podFromClientset, _ := sp.Clientset.CoreV1().Pods(sp.SchedulerName).Get(pod.Name, metav1.GetOptions{})

				resourceUsage, resourceRequests := sp.ResourceGetUsageAndRequestsFunc(container.Usage, podFromClientset.Spec.Containers[i].Resources.Requests)

				resourceUsagePerPodList = append(resourceUsagePerPodList, resourceUsage)
				resourceRequestsPerPodList = append(resourceRequestsPerPodList, resourceRequests)
			}
		}
	}

	sp.currentUtilization = sum(resourceUsagePerPodList) / sum(resourceRequestsPerPodList)

	return sp.currentUtilization
}
