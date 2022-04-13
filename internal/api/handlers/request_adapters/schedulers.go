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

package request_adapters

import (
	"time"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager/patch_scheduler"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func FromApiPatchSchedulerRequestToChangeMap(request *api.PatchSchedulerRequest) map[string]interface{} {
	patchMap := make(map[string]interface{})

	if request.Spec != nil {
		patchMap[patch_scheduler.LabelSchedulerSpec] = FromApiOptionalSpecToChangeMap(request.Spec)
	}

	if request.PortRange != nil {
		patchMap[patch_scheduler.LabelSchedulerPortRange] = entities.NewPortRange(
			request.GetPortRange().GetStart(),
			request.GetPortRange().GetEnd(),
		)
	}

	if request.MaxSurge != nil {
		patchMap[patch_scheduler.LabelSchedulerMaxSurge] = request.GetMaxSurge()
	}

	if request.Forwarders != nil {
		patchMap[patch_scheduler.LabelSchedulerForwarders] = FromApiForwarders(request.GetForwarders())
	}

	return patchMap
}

func FromApiOptionalSpecToChangeMap(request *api.OptionalSpec) map[string]interface{} {
	changeMap := make(map[string]interface{})

	if request.TerminationGracePeriod != nil {
		changeMap[patch_scheduler.LabelSpecTerminationGracePeriod] = time.Duration(request.GetTerminationGracePeriod())
	}

	if request.Containers != nil {
		changeMap[patch_scheduler.LabelSpecContainers] = FromApiOptinalContainersToChangeMap(request.GetContainers())
	}

	if request.Toleration != nil {
		changeMap[patch_scheduler.LabelSpecToleration] = request.GetToleration()
	}

	if request.Affinity != nil {
		changeMap[patch_scheduler.LabelSpecAffinity] = request.GetAffinity()
	}

	return changeMap
}

func FromApiOptinalContainersToChangeMap(request []*api.OptionalContainer) []map[string]interface{} {
	returnSlice := make([]map[string]interface{}, 0, len(request))

	for _, container := range request {
		changeMap := make(map[string]interface{})

		if container.Name != nil {
			changeMap[patch_scheduler.LabelContainerName] = container.GetName()
		}

		if container.Image != nil {
			changeMap[patch_scheduler.LabelContainerImage] = container.GetImage()
		}

		if container.ImagePullPolicy != nil {
			changeMap[patch_scheduler.LabelContainerImagePullPolicy] = container.GetImagePullPolicy()
		}

		if container.Command != nil {
			changeMap[patch_scheduler.LabelContainerCommand] = container.GetCommand()
		}

		if container.Environment != nil {
			changeMap[patch_scheduler.LabelContainerEnvironment] = FromApiContainerEnvironments(container.GetEnvironment())
		}

		if container.Requests != nil {
			changeMap[patch_scheduler.LabelContainerRequests] = game_room.ContainerResources{
				CPU:    container.GetRequests().GetCpu(),
				Memory: container.GetRequests().GetMemory(),
			}
		}

		if container.Limits != nil {
			changeMap[patch_scheduler.LabelContainerLimits] = game_room.ContainerResources{
				CPU:    container.GetLimits().GetCpu(),
				Memory: container.GetLimits().GetMemory(),
			}
		}

		if container.Ports != nil {
			changeMap[patch_scheduler.LabelContainerPorts] = FromApiContainerPorts(container.GetPorts())
		}

		returnSlice = append(returnSlice, changeMap)
	}

	return returnSlice
}

func FromApiForwarders(apiForwarders []*api.Forwarder) []*forwarder.Forwarder {
	var forwarders []*forwarder.Forwarder
	for _, apiForwarder := range apiForwarders {
		forwarderStruct := forwarder.Forwarder{
			Name:        apiForwarder.GetName(),
			Enabled:     apiForwarder.GetEnable(),
			ForwardType: forwarder.ForwardType(apiForwarder.GetType()),
			Address:     apiForwarder.GetAddress(),
		}

		if apiForwarder.Options != nil {
			forwarderStruct.Options = &forwarder.ForwardOptions{
				Timeout:  time.Duration(apiForwarder.Options.GetTimeout()),
				Metadata: apiForwarder.Options.Metadata.AsMap(),
			}
		}
		forwarders = append(forwarders, &forwarderStruct)
	}
	return forwarders
}

func FromApiContainerEnvironments(apiEnvironments []*api.ContainerEnvironment) []game_room.ContainerEnvironment {
	var environments []game_room.ContainerEnvironment
	for _, apiEnvironment := range apiEnvironments {
		environment := game_room.ContainerEnvironment{
			Name: apiEnvironment.GetName(),
		}
		switch {
		case apiEnvironment.Value != nil:
			environment.Value = *apiEnvironment.Value
		case apiEnvironment.ValueFrom != nil && apiEnvironment.ValueFrom.SecretKeyRef != nil:
			environment.ValueFrom = &game_room.ValueFrom{
				SecretKeyRef: &game_room.SecretKeyRef{
					Name: apiEnvironment.ValueFrom.SecretKeyRef.Name,
					Key:  apiEnvironment.ValueFrom.SecretKeyRef.Key,
				},
			}
		case apiEnvironment.ValueFrom != nil && apiEnvironment.ValueFrom.FieldRef != nil:
			environment.ValueFrom = &game_room.ValueFrom{
				FieldRef: &game_room.FieldRef{
					FieldPath: apiEnvironment.ValueFrom.FieldRef.FieldPath,
				},
			}
		}
		environments = append(environments, environment)
	}

	return environments
}

func FromApiContainerPorts(apiPorts []*api.ContainerPort) []game_room.ContainerPort {
	var ports []game_room.ContainerPort
	for _, apiPort := range apiPorts {
		port := game_room.ContainerPort{
			Name:     apiPort.GetName(),
			Port:     int(apiPort.GetPort()),
			Protocol: apiPort.GetProtocol(),
			HostPort: int(apiPort.GetHostPort()),
		}
		ports = append(ports, port)
	}

	return ports
}
