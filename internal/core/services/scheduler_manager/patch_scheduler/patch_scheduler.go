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

package patch_scheduler

import (
	"fmt"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

const (
	LabelSchedulerName          = "name"
	LabelSchedulerSpec          = "spec"
	LabelSchedulerPortRange     = "port_range"
	LabelSchedulerMaxSurge      = "max_surge"
	LabelSchedulerRoomsReplicas = "rooms_replicas"
	LabelSchedulerForwarders    = "forwarders"

	LabelSpecTerminationGracePeriod = "termination_grace_period"
	LabelSpecContainers             = "containers"
	LabelSpecToleration             = "toleration"
	LabelSpecAffinity               = "affinity"

	LabelContainerName            = "name"
	LabelContainerImage           = "image"
	LabelContainerImagePullPolicy = "image_pull_policy"
	LabelContainerCommand         = "command"
	LabelContainerEnvironment     = "environment"
	LabelContainerRequests        = "requests"
	LabelContainerLimits          = "limits"
	LabelContainerPorts           = "ports"
)

func PatchScheduler(scheduler entities.Scheduler, patchMap map[string]interface{}) (*entities.Scheduler, error) {
	if _, ok := patchMap[LabelSchedulerPortRange]; ok {
		if scheduler.PortRange, ok = patchMap[LabelSchedulerPortRange].(*entities.PortRange); !ok {
			return nil, fmt.Errorf("error parsing scheduler: port range malformed")
		}
	}

	if _, ok := patchMap[LabelSchedulerMaxSurge]; ok {
		scheduler.MaxSurge = fmt.Sprint(patchMap[LabelSchedulerMaxSurge])
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
