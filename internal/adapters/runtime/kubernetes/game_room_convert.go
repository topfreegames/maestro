// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package kubernetes

import (
	"fmt"
	"strings"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Toleration constants
	tolerationKey      = "dedicated"
	tolerationOperator = v1.TolerationOpEqual
	tolerationEffect   = v1.TaintEffectNoSchedule

	// Affinity constants
	affinityOperator = v1.NodeSelectorOpIn
	affinityValue    = "true"

	// Pod labels
	maestroLabelKey   = "heritage"
	maestroLabelValue = "maestro"
	schedulerLabelKey = "maestro-scheduler"
	versionLabelKey   = "version"
)

// invalidPodWaitingStates are all the states that are not accepted in a waiting pod.
var invalidPodWaitingStates = []string{
	"ErrImageNeverPull",
	"ErrImagePullBackOff",
	"ImagePullBackOff",
	"ErrInvalidImageName",
	"ErrImagePull",
	"CrashLoopBackOff",
	"RunContainerError",
}

func convertGameRoomSpec(schedulerID string, gameRoomSpec game_room.Spec) (*v1.Pod, error) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", schedulerID),
			Namespace:    schedulerID,
			Labels: map[string]string{
				maestroLabelKey:   maestroLabelValue,
				schedulerLabelKey: schedulerID,
				versionLabelKey:   gameRoomSpec.Version,
			},
		},
		Spec: v1.PodSpec{
			TerminationGracePeriodSeconds: convertTerminationGracePeriod(gameRoomSpec),
			Containers:                    []v1.Container{},
			Tolerations:                   convertSpecTolerations(gameRoomSpec),
			Affinity:                      convertSpecAffinity(gameRoomSpec),
		},
	}

	for _, container := range gameRoomSpec.Containers {
		podContainer, err := convertContainer(container)
		if err != nil {
			return nil, fmt.Errorf("error with container \"%s\": %w", container.Name, err)
		}

		pod.Spec.Containers = append(pod.Spec.Containers, podContainer)
	}

	return pod, nil
}

func convertContainer(container game_room.Container) (v1.Container, error) {
	podContainer := v1.Container{
		Name:    container.Name,
		Image:   container.Image,
		Command: container.Command,
		Ports:   []v1.ContainerPort{},
		Env:     []v1.EnvVar{},
	}

	for _, port := range container.Ports {
		containerPort, err := convertContainerPort(port)
		if err != nil {
			return v1.Container{}, fmt.Errorf("error with port \"%s\": %w", port.Name, err)
		}

		podContainer.Ports = append(podContainer.Ports, containerPort)
	}

	for _, env := range container.Environment {
		podContainer.Env = append(podContainer.Env, convertContainerEnvironment(env))
	}

	requestsResources, err := convertContainerResources(container.Requests)
	if err != nil {
		return v1.Container{}, fmt.Errorf("error with requests: %w", err)
	}

	limitsResources, err := convertContainerResources(container.Limits)
	if err != nil {
		return v1.Container{}, fmt.Errorf("error with limits: %w", err)
	}

	podContainer.Resources = v1.ResourceRequirements{
		Requests: requestsResources,
		Limits:   limitsResources,
	}

	return podContainer, nil
}

func convertContainerPort(port game_room.ContainerPort) (v1.ContainerPort, error) {
	var kubePortProtocol v1.Protocol
	switch protocol := strings.ToLower(port.Protocol); protocol {
	case "tcp":
		kubePortProtocol = v1.ProtocolTCP
	case "udp":
		kubePortProtocol = v1.ProtocolUDP
	default:
		return v1.ContainerPort{}, fmt.Errorf("invalid port protocol \"%s\"", protocol)
	}

	return v1.ContainerPort{
		Name:          port.Name,
		Protocol:      kubePortProtocol,
		ContainerPort: int32(port.Port),
		HostPort:      int32(port.HostPort),
	}, nil
}

func convertContainerResources(resources game_room.ContainerResources) (v1.ResourceList, error) {
	resourceList := v1.ResourceList{}

	if resources.CPU != "" {
		cpuQuantity, err := resource.ParseQuantity(resources.CPU)
		if err != nil {
			return v1.ResourceList{}, fmt.Errorf("failed to parse resource \"cpu\" = \"%s\"", resources.CPU)
		}

		resourceList[v1.ResourceCPU] = cpuQuantity
	}

	if resources.Memory != "" {
		cpuQuantity, err := resource.ParseQuantity(resources.Memory)
		if err != nil {
			return v1.ResourceList{}, fmt.Errorf("failed to parse resource \"memory\" = \"%s\"", resources.Memory)
		}

		resourceList[v1.ResourceMemory] = cpuQuantity
	}

	return resourceList, nil
}

func convertContainerEnvironment(env game_room.ContainerEnvironment) v1.EnvVar {
	return v1.EnvVar{
		Name:  env.Name,
		Value: env.Value,
	}
}

func convertSpecTolerations(spec game_room.Spec) []v1.Toleration {
	if spec.Toleration == "" {
		return []v1.Toleration{}
	}

	return []v1.Toleration{
		{Key: tolerationKey, Operator: tolerationOperator, Effect: tolerationEffect, Value: spec.Toleration},
	}
}

func convertSpecAffinity(spec game_room.Spec) *v1.Affinity {
	if spec.Affinity == "" {
		return nil
	}

	return &v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      spec.Affinity,
								Operator: affinityOperator,
								Values:   []string{affinityValue},
							},
						},
					},
				},
			},
		},
	}
}

func convertTerminationGracePeriod(spec game_room.Spec) *int64 {
	seconds := int64(spec.TerminationGracePeriod.Seconds())
	if seconds == int64(0) {
		return nil
	}

	return &seconds
}

func convertPodStatus(pod *v1.Pod) game_room.InstanceStatus {
	if pod.ObjectMeta.DeletionTimestamp != nil {
		return game_room.InstanceStatus{Type: game_room.InstanceTerminating}
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		state := containerStatus.State
		if state.Waiting != nil {
			for _, invalidState := range invalidPodWaitingStates {
				if state.Waiting.Reason == invalidState {
					return game_room.InstanceStatus{
						Type:        game_room.InstanceError,
						Description: fmt.Sprintf("%s: %s", state.Waiting.Reason, state.Waiting.Message),
					}
				}
			}
		}
	}

	var podReady v1.ConditionStatus
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady {
			podReady = condition.Status
		}

		if condition.Status == v1.ConditionFalse {
			return game_room.InstanceStatus{
				Type:        game_room.InstancePending,
				Description: fmt.Sprintf("%s: %s", condition.Reason, condition.Message),
			}
		}
	}

	if podReady == v1.ConditionTrue {
		return game_room.InstanceStatus{Type: game_room.InstanceReady}
	}

	if pod.Status.Phase == v1.PodPending {
		return game_room.InstanceStatus{Type: game_room.InstancePending}
	}

	return game_room.InstanceStatus{Type: game_room.InstanceUnknown}
}

func convertPod(pod *v1.Pod) *game_room.Instance {
	return &game_room.Instance{
		ID:          pod.ObjectMeta.Name,
		SchedulerID: pod.ObjectMeta.Namespace,
		Version:     pod.ObjectMeta.Labels[versionLabelKey],
		Status:      convertPodStatus(pod),
	}
}
