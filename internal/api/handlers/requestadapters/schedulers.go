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

package requestadapters

import (
	"fmt"
	"time"

	_struct "google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager/patch_scheduler"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func FromApiPatchSchedulerRequestToChangeMap(request *api.PatchSchedulerRequest) map[string]interface{} {
	patchMap := make(map[string]interface{})

	if request.Spec != nil {
		patchMap[patch_scheduler.LabelSchedulerSpec] = fromApiOptionalSpecToChangeMap(request.Spec)
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
		patchMap[patch_scheduler.LabelSchedulerForwarders] = fromApiForwarders(request.GetForwarders())
	}

	return patchMap
}

func FromApiCreateSchedulerRequestToEntity(request *api.CreateSchedulerRequest) (*entities.Scheduler, error) {
	roomsReplicas := 0
	return entities.NewScheduler(
		request.GetName(),
		request.GetGame(),
		entities.StateCreating,
		request.GetMaxSurge(),
		*fromApiSpec(request.GetSpec()),
		entities.NewPortRange(
			request.GetPortRange().GetStart(),
			request.GetPortRange().GetEnd(),
		),
		roomsReplicas,
		fromApiForwarders(request.GetForwarders()),
	)
}

func FromEntitySchedulerToListResponse(entity *entities.Scheduler) *api.SchedulerWithoutSpec {
	return &api.SchedulerWithoutSpec{
		Name:      entity.Name,
		Game:      entity.Game,
		State:     entity.State,
		Version:   entity.Spec.Version,
		PortRange: getPortRange(entity.PortRange),
		CreatedAt: timestamppb.New(entity.CreatedAt),
		MaxSurge:  entity.MaxSurge,
	}
}

func FromApiNewSchedulerVersionRequestToEntity(request *api.NewSchedulerVersionRequest) (*entities.Scheduler, error) {
	roomsReplicas := 0
	return entities.NewScheduler(
		request.GetName(),
		request.GetGame(),
		entities.StateCreating,
		request.GetMaxSurge(),
		*fromApiSpec(request.GetSpec()),
		entities.NewPortRange(
			request.GetPortRange().GetStart(),
			request.GetPortRange().GetEnd(),
		),
		roomsReplicas,
		fromApiForwarders(request.GetForwarders()),
	)
}

func FromEntitySchedulerToResponse(entity *entities.Scheduler) (*api.Scheduler, error) {
	forwarders, err := fromEntityForwardersToResponse(entity.Forwarders)
	if err != nil {
		return nil, err
	}
	return &api.Scheduler{
		Name:       entity.Name,
		Game:       entity.Game,
		State:      entity.State,
		PortRange:  getPortRange(entity.PortRange),
		CreatedAt:  timestamppb.New(entity.CreatedAt),
		MaxSurge:   entity.MaxSurge,
		Spec:       getSpec(entity.Spec),
		Forwarders: forwarders,
	}, nil
}

func FromEntitySchedulerVersionListToResponse(entity []*entities.SchedulerVersion) []*api.SchedulerVersion {
	versions := make([]*api.SchedulerVersion, len(entity))
	for i, version := range entity {
		versions[i] = &api.SchedulerVersion{
			Version:   version.Version,
			IsActive:  version.IsActive,
			CreatedAt: timestamppb.New(version.CreatedAt),
		}
	}
	return versions
}

func FromEntitySchedulerInfoToListResponse(entity *entities.SchedulerInfo) *api.SchedulerInfo {
	return &api.SchedulerInfo{
		Name:             entity.Name,
		Game:             entity.Game,
		State:            entity.State,
		RoomsReady:       int32(entity.RoomsReady),
		RoomsOccupied:    int32(entity.RoomsOccupied),
		RoomsPending:     int32(entity.RoomsPending),
		RoomsTerminating: int32(entity.RoomsTerminating),
	}
}

func fromApiOptionalSpecToChangeMap(request *api.OptionalSpec) map[string]interface{} {
	changeMap := make(map[string]interface{})

	if request.TerminationGracePeriod != nil {
		changeMap[patch_scheduler.LabelSpecTerminationGracePeriod] = time.Duration(request.GetTerminationGracePeriod())
	}

	if request.Containers != nil {
		changeMap[patch_scheduler.LabelSpecContainers] = fromApiOptionalContainersToChangeMap(request.GetContainers())
	}

	if request.Toleration != nil {
		changeMap[patch_scheduler.LabelSpecToleration] = request.GetToleration()
	}

	if request.Affinity != nil {
		changeMap[patch_scheduler.LabelSpecAffinity] = request.GetAffinity()
	}

	return changeMap
}

func fromApiOptionalContainersToChangeMap(request []*api.OptionalContainer) []map[string]interface{} {
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
			changeMap[patch_scheduler.LabelContainerEnvironment] = fromApiContainerEnvironments(container.GetEnvironment())
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
			changeMap[patch_scheduler.LabelContainerPorts] = fromApiContainerPorts(container.GetPorts())
		}

		returnSlice = append(returnSlice, changeMap)
	}

	return returnSlice
}

func fromEntityForwardersToResponse(entities []*forwarder.Forwarder) ([]*api.Forwarder, error) {
	forwarders := make([]*api.Forwarder, len(entities))
	for i, entity := range entities {
		opts, err := fromEntityForwardOptions(entity.Options)
		if err != nil {
			return nil, err
		}

		forwarders[i] = &api.Forwarder{
			Name:    entity.Name,
			Enable:  entity.Enabled,
			Type:    fmt.Sprint(entity.ForwardType),
			Address: entity.Address,
			Options: opts,
		}
	}
	return forwarders, nil
}

func fromEntityForwardOptions(entity *forwarder.ForwardOptions) (*api.ForwarderOptions, error) {
	if entity == nil {
		return nil, nil
	}
	protoStruct, err := _struct.NewStruct(entity.Metadata)
	if err != nil {
		return nil, err
	}

	return &api.ForwarderOptions{
		Timeout:  int64(entity.Timeout),
		Metadata: protoStruct,
	}, nil
}

func fromApiSpec(apiSpec *api.Spec) *game_room.Spec {
	return game_room.NewSpec(
		"",
		time.Duration(apiSpec.GetTerminationGracePeriod()),
		fromApiContainers(apiSpec.GetContainers()),
		apiSpec.GetToleration(),
		apiSpec.GetAffinity(),
	)
}

func fromApiContainers(apiContainers []*api.Container) []game_room.Container {
	var containers []game_room.Container
	for _, apiContainer := range apiContainers {
		container := game_room.Container{
			Name:            apiContainer.GetName(),
			Image:           apiContainer.GetImage(),
			ImagePullPolicy: apiContainer.GetImagePullPolicy(),
			Command:         apiContainer.GetCommand(),
			Ports:           fromApiContainerPorts(apiContainer.GetPorts()),
			Environment:     fromApiContainerEnvironments(apiContainer.GetEnvironment()),
			Requests: game_room.ContainerResources{
				CPU:    apiContainer.GetRequests().GetCpu(),
				Memory: apiContainer.GetRequests().GetMemory(),
			},
			Limits: game_room.ContainerResources{
				CPU:    apiContainer.GetLimits().GetCpu(),
				Memory: apiContainer.GetLimits().GetMemory(),
			},
		}
		containers = append(containers, container)
	}

	return containers
}

func fromApiContainerPorts(apiPorts []*api.ContainerPort) []game_room.ContainerPort {
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

func fromApiContainerEnvironments(apiEnvironments []*api.ContainerEnvironment) []game_room.ContainerEnvironment {
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

func fromApiForwarders(apiForwarders []*api.Forwarder) []*forwarder.Forwarder {
	var forwarders []*forwarder.Forwarder
	for _, apiForwarder := range apiForwarders {
		var options *forwarder.ForwardOptions
		if apiForwarder.GetOptions() != nil {
			options = &forwarder.ForwardOptions{
				Timeout:  time.Duration(apiForwarder.Options.GetTimeout()),
				Metadata: apiForwarder.Options.Metadata.AsMap(),
			}
		}

		forwarderStruct := forwarder.Forwarder{
			Name:        apiForwarder.GetName(),
			Enabled:     apiForwarder.GetEnable(),
			ForwardType: forwarder.ForwardType(apiForwarder.GetType()),
			Address:     apiForwarder.GetAddress(),
			Options:     options,
		}
		forwarders = append(forwarders, &forwarderStruct)
	}
	return forwarders
}

func getPortRange(portRange *entities.PortRange) *api.PortRange {
	if portRange != nil {
		return &api.PortRange{
			Start: portRange.Start,
			End:   portRange.End,
		}
	}

	return nil
}

func getSpec(spec game_room.Spec) *api.Spec {
	if spec.Version != "" {
		return &api.Spec{
			Version:                spec.Version,
			Toleration:             spec.Toleration,
			Containers:             fromEntityContainerToApiContainer(spec.Containers),
			TerminationGracePeriod: int64(spec.TerminationGracePeriod),
			Affinity:               spec.Affinity,
		}
	}

	return nil
}

func fromEntityContainerToApiContainer(containers []game_room.Container) []*api.Container {
	var convertedContainers []*api.Container
	for _, container := range containers {
		convertedContainers = append(convertedContainers, &api.Container{
			Name:            container.Name,
			Image:           container.Image,
			ImagePullPolicy: container.ImagePullPolicy,
			Command:         container.Command,
			Environment:     fromEntityContainerEnvironmentToApiContainerEnvironment(container.Environment),
			Requests:        fromEntityContainerResourcesToApiContainerResources(container.Requests),
			Limits:          fromEntityContainerResourcesToApiContainerResources(container.Limits),
			Ports:           fromEntityContainerPortsToApiContainerPorts(container.Ports),
		})
	}
	return convertedContainers
}

func fromEntityContainerEnvironmentToApiContainerEnvironment(environments []game_room.ContainerEnvironment) []*api.ContainerEnvironment {
	var convertedContainerEnvironment []*api.ContainerEnvironment
	for _, environment := range environments {
		apiContainerEnv := &api.ContainerEnvironment{
			Name: environment.Name,
		}
		switch {
		case environment.Value != "":
			envValue := environment.Value
			apiContainerEnv.Value = &envValue
		case environment.ValueFrom != nil && environment.ValueFrom.SecretKeyRef != nil:
			apiContainerEnv.ValueFrom = &api.ContainerEnvironmentValueFrom{
				SecretKeyRef: &api.ContainerEnvironmentValueFromSecretKeyRef{
					Name: environment.ValueFrom.SecretKeyRef.Name,
					Key:  environment.ValueFrom.SecretKeyRef.Key,
				},
			}
		case environment.ValueFrom != nil && environment.ValueFrom.FieldRef != nil:
			apiContainerEnv.ValueFrom = &api.ContainerEnvironmentValueFrom{
				FieldRef: &api.ContainerEnvironmentValueFromFieldRef{
					FieldPath: environment.ValueFrom.FieldRef.FieldPath,
				},
			}
		}
		convertedContainerEnvironment = append(convertedContainerEnvironment, apiContainerEnv)
	}
	return convertedContainerEnvironment
}

func fromEntityContainerResourcesToApiContainerResources(resources game_room.ContainerResources) *api.ContainerResources {
	return &api.ContainerResources{
		Memory: resources.Memory,
		Cpu:    resources.CPU,
	}
}

func fromEntityContainerPortsToApiContainerPorts(ports []game_room.ContainerPort) []*api.ContainerPort {
	var convertedContainerPort []*api.ContainerPort
	for _, port := range ports {
		convertedContainerPort = append(convertedContainerPort, &api.ContainerPort{
			Name:     port.Name,
			Protocol: port.Protocol,
			Port:     int32(port.Port),
			HostPort: int32(port.HostPort),
		})
	}
	return convertedContainerPort
}
