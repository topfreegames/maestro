package kubernetes

import (
	"fmt"
	"strings"

	"github.com/topfreegames/maestro/internal/entities"

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

func convertGameRoomSpec(scheduler entities.Scheduler, gameRoomSpec entities.GameRoomSpec) (*v1.Pod, error) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", scheduler.ID),
			Namespace:    scheduler.ID,
			Labels: map[string]string{
				maestroLabelKey:   maestroLabelValue,
				schedulerLabelKey: scheduler.ID,
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

func convertContainer(container entities.GameRoomContainer) (v1.Container, error) {
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

func convertContainerPort(port entities.GameRoomContainerPort) (v1.ContainerPort, error) {
	var kubePortProtocol v1.Protocol
	switch protocol := strings.ToLower(port.Protocol); protocol {
	case "tcp":
		kubePortProtocol = v1.ProtocolTCP
		break
	case "udp":
		kubePortProtocol = v1.ProtocolUDP
		break
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

func convertContainerResources(resources entities.GameRoomContainerResources) (v1.ResourceList, error) {
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

func convertContainerEnvironment(env entities.GameRoomContainerEnvironment) v1.EnvVar {
	return v1.EnvVar{
		Name:  env.Name,
		Value: env.Value,
	}
}

func convertSpecTolerations(spec entities.GameRoomSpec) []v1.Toleration {
	if spec.Toleration == "" {
		return []v1.Toleration{}
	}

	return []v1.Toleration{
		{Key: tolerationKey, Operator: tolerationOperator, Effect: tolerationEffect, Value: spec.Toleration},
	}
}

func convertSpecAffinity(spec entities.GameRoomSpec) *v1.Affinity {
	if spec.Affinity == "" {
		return nil
	}

	return &v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					v1.NodeSelectorTerm{
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

func convertTerminationGracePeriod(spec entities.GameRoomSpec) *int64 {
	seconds := int64(spec.TerminationGracePeriod.Seconds())
	if seconds == int64(0) {
		return nil
	}

	return &seconds
}
