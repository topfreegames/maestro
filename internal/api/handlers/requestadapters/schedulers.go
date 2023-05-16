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

	"github.com/topfreegames/maestro/internal/core/services/schedulers/patch"

	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"

	"google.golang.org/protobuf/types/known/durationpb"
	_struct "google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func FromApiPatchSchedulerRequestToChangeMap(request *api.PatchSchedulerRequest) map[string]interface{} {
	patchMap := make(map[string]interface{})

	if request.Spec != nil {
		patchMap[patch.LabelSchedulerSpec] = fromApiOptionalSpecToChangeMap(request.Spec)
	}

	if request.PortRange != nil {
		patchMap[patch.LabelSchedulerPortRange] = entities.NewPortRange(
			request.GetPortRange().GetStart(),
			request.GetPortRange().GetEnd(),
		)
	}

	if request.MaxSurge != nil {
		patchMap[patch.LabelSchedulerMaxSurge] = request.GetMaxSurge()
	}

	if request.RoomsReplicas != nil {
		patchMap[patch.LabelSchedulerRoomsReplicas] = int(request.GetRoomsReplicas())
	}

	if request.Autoscaling != nil {
		newAutoscaling := fromApiOptionalAutoscalingToChangeMap(request.GetAutoscaling())
		patchMap[patch.LabelAutoscaling] = newAutoscaling
	}

	if request.Forwarders != nil {
		patchMap[patch.LabelSchedulerForwarders] = fromApiForwarders(request.GetForwarders())
	}

	return patchMap
}

func fromApiOptionalAutoscalingToChangeMap(apiAutoscaling *api.OptionalAutoscaling) map[string]interface{} {
	changeMap := make(map[string]interface{})

	if apiAutoscaling.Enabled != nil {
		changeMap[patch.LabelAutoscalingEnabled] = apiAutoscaling.GetEnabled()
	}

	if apiAutoscaling.Min != nil {
		changeMap[patch.LabelAutoscalingMin] = apiAutoscaling.GetMin()
	}

	if apiAutoscaling.Max != nil {
		changeMap[patch.LabelAutoscalingMax] = apiAutoscaling.GetMax()
	}

	if apiAutoscaling.Policy != nil {
		autoscalingPolicy := fromApiAutoscalingPolicy(apiAutoscaling.GetPolicy())
		changeMap[patch.LabelAutoscalingPolicy] = autoscalingPolicy
	}

	return changeMap
}

func FromApiCreateSchedulerRequestToEntity(request *api.CreateSchedulerRequest) (*entities.Scheduler, error) {
	schedulerAutoscaling, err := fromApiAutoscaling(request.GetAutoscaling())
	if err != nil {
		return nil, err
	}

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
		int(request.GetRoomsReplicas()),
		schedulerAutoscaling,
		fromApiForwarders(request.GetForwarders()),
		fromApiToAnnotation(request.GetAnnotations()),
	)
}

func FromEntitySchedulerToListResponse(entity *entities.Scheduler) *api.SchedulerWithoutSpec {
	return &api.SchedulerWithoutSpec{
		Name:          entity.Name,
		Game:          entity.Game,
		State:         entity.State,
		Version:       entity.Spec.Version,
		PortRange:     getPortRange(entity.PortRange),
		CreatedAt:     timestamppb.New(entity.CreatedAt),
		MaxSurge:      entity.MaxSurge,
		RoomsReplicas: int32(entity.RoomsReplicas),
	}
}

func FromApiNewSchedulerVersionRequestToEntity(request *api.NewSchedulerVersionRequest) (*entities.Scheduler, error) {
	schedulerAutoscaling, err := fromApiAutoscaling(request.GetAutoscaling())
	if err != nil {
		return nil, err
	}

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
		int(request.GetRoomsReplicas()),
		schedulerAutoscaling,
		fromApiForwarders(request.GetForwarders()),
		fromApiToAnnotation(request.GetAnnotations()),
	)
}

func FromEntitySchedulerToResponse(entity *entities.Scheduler) (*api.Scheduler, error) {
	forwarders, err := fromEntityForwardersToResponse(entity.Forwarders)
	annotations, err := fromEntityAnnotationsToResponse(entity.Annotations)
	if err != nil {
		return nil, err
	}
	return &api.Scheduler{
		Name:          entity.Name,
		Game:          entity.Game,
		State:         entity.State,
		PortRange:     getPortRange(entity.PortRange),
		CreatedAt:     timestamppb.New(entity.CreatedAt),
		MaxSurge:      entity.MaxSurge,
		RoomsReplicas: int32(entity.RoomsReplicas),
		Spec:          getSpec(entity.Spec),
		Autoscaling:   getAutoscaling(entity.Autoscaling),
		Forwarders:    forwarders,
		Annotations:   annotations,
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
		RoomsReplicas:    int32(entity.RoomsReplicas),
		RoomsReady:       int32(entity.RoomsReady),
		RoomsOccupied:    int32(entity.RoomsOccupied),
		RoomsPending:     int32(entity.RoomsPending),
		RoomsTerminating: int32(entity.RoomsTerminating),
		Autoscaling:      fromEntityAutoscalingInfoToApiResponse(entity.Autoscaling),
	}
}

func fromEntityAutoscalingInfoToApiResponse(entity *entities.AutoscalingInfo) *api.AutoscalingInfo {
	if entity == nil {
		return nil
	}
	return &api.AutoscalingInfo{
		Enabled: entity.Enabled,
		Min:     int32(entity.Min),
		Max:     int32(entity.Max),
	}
}

func fromApiOptionalSpecToChangeMap(request *api.OptionalSpec) map[string]interface{} {
	changeMap := make(map[string]interface{})

	if request.TerminationGracePeriod != nil {
		changeMap[patch.LabelSpecTerminationGracePeriod] = time.Duration(request.GetTerminationGracePeriod().AsDuration())
	}

	if request.Containers != nil {
		changeMap[patch.LabelSpecContainers] = fromApiOptionalContainersToChangeMap(request.GetContainers())
	}

	if request.Toleration != nil {
		changeMap[patch.LabelSpecToleration] = request.GetToleration()
	}

	if request.Affinity != nil {
		changeMap[patch.LabelSpecAffinity] = request.GetAffinity()
	}

	return changeMap
}

func fromApiOptionalContainersToChangeMap(request []*api.OptionalContainer) []map[string]interface{} {
	returnSlice := make([]map[string]interface{}, 0, len(request))

	for _, container := range request {
		changeMap := make(map[string]interface{})

		if container.Name != nil {
			changeMap[patch.LabelContainerName] = container.GetName()
		}

		if container.Image != nil {
			changeMap[patch.LabelContainerImage] = container.GetImage()
		}

		if container.ImagePullPolicy != nil {
			changeMap[patch.LabelContainerImagePullPolicy] = container.GetImagePullPolicy()
		}

		if container.Command != nil {
			changeMap[patch.LabelContainerCommand] = container.GetCommand()
		}

		if container.Environment != nil {
			changeMap[patch.LabelContainerEnvironment] = fromApiContainerEnvironments(container.GetEnvironment())
		}

		if container.Requests != nil {
			changeMap[patch.LabelContainerRequests] = game_room.ContainerResources{
				CPU:    container.GetRequests().GetCpu(),
				Memory: container.GetRequests().GetMemory(),
			}
		}

		if container.Limits != nil {
			changeMap[patch.LabelContainerLimits] = game_room.ContainerResources{
				CPU:    container.GetLimits().GetCpu(),
				Memory: container.GetLimits().GetMemory(),
			}
		}

		if container.Ports != nil {
			changeMap[patch.LabelContainerPorts] = fromApiContainerPorts(container.GetPorts())
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
func fromEntityAnnotationsToResponse(entities []*entities.Annotations) ([]*api.Annotation, error) {
	annotations := make([]*api.Annotation, len(entities))

	for i, entity := range entities {

		annotations[i] = &api.Annotation{
			Name:  entity.Name,
			Value: entity.Opt,
		}

	}
	return annotations, nil
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
		time.Duration(apiSpec.GetTerminationGracePeriod().AsDuration()),
		fromApiContainers(apiSpec.GetContainers()),
		apiSpec.GetToleration(),
		apiSpec.GetAffinity(),
	)
}

func fromApiAutoscalingPolicy(apiAutoscalingPolicy *api.AutoscalingPolicy) autoscaling.Policy {
	var policy autoscaling.Policy
	if policyType := apiAutoscalingPolicy.GetType(); policyType != "" {
		policy.Type = autoscaling.PolicyType(policyType)
	}
	if apiPolicyParameters := apiAutoscalingPolicy.GetParameters(); apiPolicyParameters != nil {
		policy.Parameters = fromApiAutoscalingPolicyParameters(apiPolicyParameters)
	}
	return policy
}

func fromApiAutoscalingPolicyParameters(apiPolicyParameters *api.PolicyParameters) autoscaling.PolicyParameters {
	var policyParameters autoscaling.PolicyParameters
	if roomOccupancy := apiPolicyParameters.GetRoomOccupancy(); roomOccupancy != nil {
		policyParameters.RoomOccupancy = fromApiRoomOccupancyPolicyToEntity(roomOccupancy)
	}
	return policyParameters
}

func fromApiRoomOccupancyPolicyToEntity(roomOccupancy *api.RoomOccupancy) *autoscaling.RoomOccupancyParams {
	roomOccupancyParam := &autoscaling.RoomOccupancyParams{}

	readyTarget := roomOccupancy.GetReadyTarget()
	roomOccupancyParam.ReadyTarget = float64(readyTarget)

	return roomOccupancyParam
}

func fromApiAutoscaling(apiAutoscaling *api.Autoscaling) (*autoscaling.Autoscaling, error) {
	if apiAutoscaling != nil {
		autoscalingPolicy := fromApiAutoscalingPolicy(apiAutoscaling.GetPolicy())
		return autoscaling.NewAutoscaling(
			apiAutoscaling.GetEnabled(),
			int(apiAutoscaling.GetMin()),
			int(apiAutoscaling.GetMax()),
			autoscalingPolicy,
		)
	}
	return nil, nil
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
func fromApiToAnnotation(apiAnnotations []*api.Annotation) []*entities.Annotations {
	annotations := make([]*entities.Annotations, len(apiAnnotations))
	for i, apiAnnotation := range apiAnnotations {
		annotation := entities.NewAnnotations(apiAnnotation.Name, apiAnnotation.Value)
		annotations[i] = &entities.Annotations{
			Name: annotation.Name,
			Opt:  annotation.Opt,
		}
	}
	return annotations
}

func fromApiForwarders(apiForwarders []*api.Forwarder) []*forwarder.Forwarder {
	forwarders := make([]*forwarder.Forwarder, 0)
	for _, apiForwarder := range apiForwarders {
		var options *forwarder.ForwardOptions
		if apiForwarder.GetOptions() != nil {
			options = &forwarder.ForwardOptions{
				Timeout:  time.Duration(apiForwarder.Options.GetTimeout()),
				Metadata: apiForwarder.Options.Metadata.AsMap(),
			}
		}

		forwarderStruct := forwarder.New(
			apiForwarder.GetName(),
			apiForwarder.GetEnable(),
			forwarder.ForwardType(apiForwarder.GetType()),
			apiForwarder.GetAddress(),
			options,
		)
		forwarders = append(forwarders, forwarderStruct)
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
			TerminationGracePeriod: durationpb.New(spec.TerminationGracePeriod),
			Affinity:               spec.Affinity,
		}
	}

	return nil
}

func getAutoscaling(autoscaling *autoscaling.Autoscaling) *api.Autoscaling {
	if autoscaling != nil {
		return &api.Autoscaling{
			Enabled: autoscaling.Enabled,
			Min:     int32(autoscaling.Min),
			Max:     int32(autoscaling.Max),
			Policy:  getAutoscalingPolicy(autoscaling.Policy),
		}
	}

	return nil
}

func getAutoscalingPolicy(autoscalingPolicy autoscaling.Policy) *api.AutoscalingPolicy {
	return &api.AutoscalingPolicy{
		Type:       string(autoscalingPolicy.Type),
		Parameters: getPolicyParameters(autoscalingPolicy.Parameters),
	}
}

func getPolicyParameters(parameters autoscaling.PolicyParameters) *api.PolicyParameters {
	return &api.PolicyParameters{
		RoomOccupancy: getRoomOccupancy(parameters.RoomOccupancy),
	}
}

func getRoomOccupancy(roomOccupancyParameters *autoscaling.RoomOccupancyParams) *api.RoomOccupancy {
	if roomOccupancyParameters == nil {
		return nil
	}
	return &api.RoomOccupancy{
		ReadyTarget: float32(roomOccupancyParameters.ReadyTarget),
	}
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
