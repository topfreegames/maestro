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

package patch

import (
	"fmt"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

const (
	// LabelSchedulerName is the name key in the patch map.
	LabelSchedulerName = "name"
	// LabelSchedulerSpec is the spec key in the patch map.
	LabelSchedulerSpec = "spec"
	// LabelSchedulerPortRange is the port range key in the patch map.
	LabelSchedulerPortRange = "port_range"
	// LabelSchedulerMaxSurge is the max surge key in the patch map.
	LabelSchedulerMaxSurge = "max_surge"
	// LabelSchedulerDownSurge is the down surge key in the patch map.
	LabelSchedulerDownSurge = "down_surge"
	// LabelSchedulerRoomsReplicas is the rooms replicas key in the patch map.
	LabelSchedulerRoomsReplicas = "rooms_replicas"
	// LabelAutoscaling is the key to autoscaling in the patch map.
	LabelAutoscaling = "autoscaling"
	// LabelSchedulerForwarders is the forwarders key in the patch map.
	LabelSchedulerForwarders = "forwarders"

	// LabelSpecTerminationGracePeriod is the termination grace period key in the patch map.
	LabelSpecTerminationGracePeriod = "termination_grace_period"
	// LabelSpecContainers is the containers key in the patch map.
	LabelSpecContainers = "containers"
	// LabelSpecToleration is the toleration key in the patch map.
	LabelSpecToleration = "toleration"
	// LabelSpecAffinity is the affinity key in the patch map.
	LabelSpecAffinity = "affinity"

	// LabelContainerName is the container name key in the patch map.
	LabelContainerName = "name"
	// LabelContainerImage is the image name key in the patch map.
	LabelContainerImage = "image"
	// LabelContainerImagePullPolicy is the image pull policy name key in the patch map.
	LabelContainerImagePullPolicy = "image_pull_policy"
	// LabelContainerCommand is the command key in the patch map.
	LabelContainerCommand = "command"
	// LabelContainerEnvironment is the environment key in the patch map.
	LabelContainerEnvironment = "environment"
	// LabelContainerRequests is the requests key in the patch map.
	LabelContainerRequests = "requests"
	// LabelContainerLimits is the limits key in the patch map.
	LabelContainerLimits = "limits"
	// LabelContainerPorts the ports key in the patch map.
	LabelContainerPorts = "ports"

	// LabelAutoscalingEnabled is the autoscaling enabled key in the patch map.
	LabelAutoscalingEnabled = "autoscalingEnabled"
	// LabelAutoscalingMin is the autoscaling min key in the patch map.
	LabelAutoscalingMin = "min"
	// LabelAutoscalingMax is the autoscaling max key in the patch map.
	LabelAutoscalingMax = "max"
	// LabelAutoscalingCooldown is the autoscaling cooldown key in the patch map.
	LabelAutoscalingCooldown = "cooldown"
	// LabelAutoscalingPolicy is the autoscaling policy key in the patch map.
	LabelAutoscalingPolicy = "policy"
	// LabelAnnotations is the annotations key in the patch map
	LabelAnnotations = "annotations"
	// LabelLabels is the labels key in the patch map
	LabelLabels = "labels"
)

// PatchScheduler function applies the patchMap in the scheduler, returning the patched Scheduler.
func PatchScheduler(scheduler entities.Scheduler, patchMap map[string]interface{}) (*entities.Scheduler, error) {
	if _, ok := patchMap[LabelSchedulerPortRange]; ok {
		if scheduler.PortRange, ok = patchMap[LabelSchedulerPortRange].(*entities.PortRange); !ok {
			return nil, fmt.Errorf("error parsing scheduler: port range malformed")
		}
	}

	if _, ok := patchMap[LabelSchedulerMaxSurge]; ok {
		scheduler.MaxSurge = fmt.Sprint(patchMap[LabelSchedulerMaxSurge])
	}

	if _, ok := patchMap[LabelSchedulerDownSurge]; ok {
		scheduler.DownSurge = fmt.Sprint(patchMap[LabelSchedulerDownSurge])
	}

	if _, ok := patchMap[LabelSchedulerRoomsReplicas]; ok {
		scheduler.RoomsReplicas = (patchMap[LabelSchedulerRoomsReplicas]).(int)
	}

	if _, ok := patchMap[LabelSchedulerForwarders]; ok {
		if scheduler.Forwarders, ok = patchMap[LabelSchedulerForwarders].([]*forwarder.Forwarder); !ok {
			return nil, fmt.Errorf("error parsing scheduler: forwarders malformed")
		}
	}

	if _, ok := patchMap[LabelSchedulerSpec]; ok {
		var patchSpecMap map[string]interface{}
		if patchSpecMap, ok = patchMap[LabelSchedulerSpec].(map[string]interface{}); !ok {
			return nil, fmt.Errorf("error parsing scheduler: spec malformed")
		}
		spec, err := patchSpec(scheduler.Spec, patchSpecMap)
		if err != nil {
			return nil, fmt.Errorf("error parsing scheduler: %w", err)
		}

		scheduler.Spec = *spec
	}

	if _, ok := patchMap[LabelAutoscaling]; ok {
		var patchAutoscalingMap map[string]interface{}
		if patchAutoscalingMap, ok = patchMap[LabelAutoscaling].(map[string]interface{}); !ok {
			return nil, fmt.Errorf("error parsing scheduler: autoscaling malformed")
		}

		err := patchAutoscaling(&scheduler, patchAutoscalingMap)
		if err != nil {
			return nil, fmt.Errorf("error parsing scheduler: %w", err)
		}
	}

	if _, ok := patchMap[LabelAnnotations]; ok {
		if scheduler.Annotations, ok = patchMap[LabelAnnotations].(map[string]string); !ok {
			return nil, fmt.Errorf("error parsing scheduler: annotations malformed")
		}
	}

	if _, ok := patchMap[LabelLabels]; ok {
		if scheduler.Labels, ok = patchMap[LabelLabels].(map[string]string); !ok {
			return nil, fmt.Errorf("error parsing scheduler: labels malformed")
		}
	}

	return &scheduler, nil
}

func patchSpec(spec game_room.Spec, patchMap map[string]interface{}) (*game_room.Spec, error) {
	if _, ok := patchMap[LabelSpecTerminationGracePeriod]; ok {
		if spec.TerminationGracePeriod, ok = patchMap[LabelSpecTerminationGracePeriod].(time.Duration); !ok {
			return nil, fmt.Errorf("error parsing spec: termination grace period malformed")
		}
	}

	if _, ok := patchMap[LabelSpecToleration]; ok {
		spec.Toleration = fmt.Sprint(patchMap[LabelSpecToleration])
	}

	if _, ok := patchMap[LabelSpecAffinity]; ok {
		spec.Affinity = fmt.Sprint(patchMap[LabelSpecAffinity])
	}

	if _, ok := patchMap[LabelSpecContainers]; ok {
		var patchContainersMap []map[string]interface{}
		if patchContainersMap, ok = patchMap[LabelSpecContainers].([]map[string]interface{}); !ok {
			return nil, fmt.Errorf("error parsing spec: containers malformed")
		}
		containers, err := patchContainers(spec.Containers, patchContainersMap)
		if err != nil {
			return nil, fmt.Errorf("error parsing spec: %w", err)
		}
		spec.Containers = containers
	}

	return &spec, nil
}

func patchContainers(containers []game_room.Container, patchSlice []map[string]interface{}) ([]game_room.Container, error) {
	for i, patchMap := range patchSlice {
		if len(containers) <= i {
			containers = append(containers, game_room.Container{})
		}

		if _, ok := patchMap[LabelContainerName]; ok {
			containers[i].Name = fmt.Sprint(patchMap[LabelContainerName])
		}

		if _, ok := patchMap[LabelContainerImage]; ok {
			containers[i].Image = fmt.Sprint(patchMap[LabelContainerImage])
		}

		if _, ok := patchMap[LabelContainerImagePullPolicy]; ok {
			containers[i].ImagePullPolicy = fmt.Sprint(patchMap[LabelContainerImagePullPolicy])
		}

		if _, ok := patchMap[LabelContainerCommand]; ok {
			if containers[i].Command, ok = patchMap[LabelContainerCommand].([]string); !ok {
				return nil, fmt.Errorf("error parsing containers: command malformed")
			}
		}

		if _, ok := patchMap[LabelContainerEnvironment]; ok {
			if containers[i].Environment, ok = patchMap[LabelContainerEnvironment].([]game_room.ContainerEnvironment); !ok {
				return nil, fmt.Errorf("error parsing containers: environment malformed")
			}
		}

		if _, ok := patchMap[LabelContainerRequests]; ok {
			if containers[i].Requests, ok = patchMap[LabelContainerRequests].(game_room.ContainerResources); !ok {
				return nil, fmt.Errorf("error parsing containers: requests malformed")
			}
		}

		if _, ok := patchMap[LabelContainerLimits]; ok {
			if containers[i].Limits, ok = patchMap[LabelContainerLimits].(game_room.ContainerResources); !ok {
				return nil, fmt.Errorf("error parsing containers: limits malformed")
			}
		}

		if _, ok := patchMap[LabelContainerPorts]; ok {
			if containers[i].Ports, ok = patchMap[LabelContainerPorts].([]game_room.ContainerPort); !ok {
				return nil, fmt.Errorf("error parsing containers: ports malformed")
			}
		}

	}

	return containers, nil
}

func patchAutoscaling(scheduler *entities.Scheduler, patchMap map[string]interface{}) error {
	if scheduler.Autoscaling == nil {
		scheduler.Autoscaling = &autoscaling.Autoscaling{}
	}

	if interfaceEnabled, ok := patchMap[LabelAutoscalingEnabled]; ok {
		if scheduler.Autoscaling.Enabled, ok = interfaceEnabled.(bool); !ok {
			return fmt.Errorf("error parsing autoscaling: enabled malformed")
		}
	}

	if interfaceMin, ok := patchMap[LabelAutoscalingMin]; ok {
		min, ok := interfaceMin.(int32)
		if !ok {
			return fmt.Errorf("error parsing autoscaling: min malformed")
		}
		scheduler.Autoscaling.Min = int(min)
	}

	if interfaceMax, ok := patchMap[LabelAutoscalingMax]; ok {
		max, ok := interfaceMax.(int32)
		if !ok {
			return fmt.Errorf("error parsing autoscaling: max malformed")
		}
		scheduler.Autoscaling.Max = int(max)
	}

	if intrefaceCoolDown, ok := patchMap[LabelAutoscalingCooldown]; ok {
		cooldown, ok := intrefaceCoolDown.(int32)
		if !ok {
			return fmt.Errorf("error parsing autoscaling: cooldown malformed")
		}
		scheduler.Autoscaling.Cooldown = int(cooldown)
	}

	if interfacePolicy, ok := patchMap[LabelAutoscalingPolicy]; ok {
		patchPolicy, ok := interfacePolicy.(autoscaling.Policy)
		if !ok {
			return fmt.Errorf("error parsing autoscaling: policy malformed")
		}
		scheduler.Autoscaling.Policy = patchPolicy
	}
	return scheduler.Validate()
}
