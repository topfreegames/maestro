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

//go:build unit
// +build unit

package requestadapters_test

import (
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/validations"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/api/handlers/requestadapters"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager/patch_scheduler"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func TestFromApiPatchSchedulerRequestToChangeMap(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	type Input struct {
		PatchScheduler *api.PatchSchedulerRequest
	}

	type Output struct {
		PatchScheduler map[string]interface{}
		Error          bool
	}

	terminationValue := int64(62)
	maxSurgeValue := "60%"
	roomsReplicasValue := int32(2)
	genericString := "some-value"
	genericStringList := []string{"some-value", "another-value"}
	genericFloat32 := float32(0.3)

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "only spec should convert api.PatchSchedulerRequest to change map",
			Input: Input{
				PatchScheduler: &api.PatchSchedulerRequest{
					Spec: &api.OptionalSpec{
						TerminationGracePeriod: &terminationValue,
					},
				},
			},
			Output: Output{
				PatchScheduler: map[string]interface{}{
					patch_scheduler.LabelSchedulerSpec: map[string]interface{}{
						patch_scheduler.LabelSpecTerminationGracePeriod: time.Duration(terminationValue),
					},
				},
			},
		},
		{
			Title: "only port range should convert api.PatchSchedulerRequest to change map",
			Input: Input{
				PatchScheduler: &api.PatchSchedulerRequest{
					PortRange: &api.PortRange{
						Start: 10000,
						End:   60000,
					},
				},
			},
			Output: Output{
				PatchScheduler: map[string]interface{}{
					patch_scheduler.LabelSchedulerPortRange: entities.NewPortRange(10000, 60000),
				},
			},
		},
		{
			Title: "only max surge should convert api.PatchSchedulerRequest to change map",
			Input: Input{
				PatchScheduler: &api.PatchSchedulerRequest{
					MaxSurge: &maxSurgeValue,
				},
			},
			Output: Output{
				PatchScheduler: map[string]interface{}{
					patch_scheduler.LabelSchedulerMaxSurge: maxSurgeValue,
				},
			},
		},
		{
			Title: "only rooms replicas should convert api.PatchSchedulerRequest to change map",
			Input: Input{
				PatchScheduler: &api.PatchSchedulerRequest{
					RoomsReplicas: &roomsReplicasValue,
				},
			},
			Output: Output{
				PatchScheduler: map[string]interface{}{
					patch_scheduler.LabelSchedulerRoomsReplicas: int(roomsReplicasValue),
				},
			},
		},
		{
			Title: "only forwarders should convert api.PatchSchedulerRequest to change map",
			Input: Input{
				PatchScheduler: &api.PatchSchedulerRequest{
					Forwarders: []*api.Forwarder{
						{
							Name:    "some-forwarder",
							Enable:  true,
							Type:    "some-type",
							Address: "localhost:8080",
							Options: &api.ForwarderOptions{
								Timeout: int64(10),
							},
						},
						{
							Name:    "another-forwarder",
							Enable:  false,
							Type:    "another-type",
							Address: "localhost:8888",
							Options: nil,
						},
					},
				},
			},
			Output: Output{
				PatchScheduler: map[string]interface{}{
					patch_scheduler.LabelSchedulerForwarders: []*forwarder.Forwarder{
						{
							Name:        "some-forwarder",
							Enabled:     true,
							ForwardType: forwarder.ForwardType("some-type"),
							Address:     "localhost:8080",
							Options: &forwarder.ForwardOptions{
								Timeout:  time.Duration(10),
								Metadata: map[string]interface{}{},
							},
						},
						{
							Name:        "another-forwarder",
							Enabled:     false,
							ForwardType: forwarder.ForwardType("another-type"),
							Address:     "localhost:8888",
							Options:     nil,
						},
					},
				},
			},
		},
		{
			Title: "only empty forwarders should convert api.PatchSchedulerRequest to change map",
			Input: Input{
				PatchScheduler: &api.PatchSchedulerRequest{
					Forwarders: []*api.Forwarder{},
				},
			},
			Output: Output{
				PatchScheduler: map[string]interface{}{
					patch_scheduler.LabelSchedulerForwarders: []*forwarder.Forwarder{},
				},
			},
		},
		{
			Title: "only containers should convert api.PatchSchedulerRequest to change map",
			Input: Input{
				PatchScheduler: &api.PatchSchedulerRequest{
					Spec: &api.OptionalSpec{Containers: []*api.OptionalContainer{
						{
							Name: &genericString,
						},
						{
							Image: &genericString,
						},
						{
							ImagePullPolicy: &genericString,
						},
						{
							Command: genericStringList,
						},
						{
							Environment: []*api.ContainerEnvironment{
								{
									Name:  genericString,
									Value: &genericString,
								},
							},
						},
						{
							Requests: &api.ContainerResources{
								Memory: "1000m",
								Cpu:    "100",
							},
						},
						{
							Limits: &api.ContainerResources{
								Memory: "2000m",
								Cpu:    "200",
							},
						},
						{
							Ports: []*api.ContainerPort{
								{
									Name:     "some-port",
									Protocol: "TCP",
									Port:     123,
									HostPort: 1234,
								},
							},
						},
					}},
				},
			},
			Output: Output{
				PatchScheduler: map[string]interface{}{
					patch_scheduler.LabelSchedulerSpec: map[string]interface{}{
						patch_scheduler.LabelSpecContainers: []map[string]interface{}{
							{
								patch_scheduler.LabelContainerName: genericString,
							},
							{
								patch_scheduler.LabelContainerImage: genericString,
							},
							{
								patch_scheduler.LabelContainerImagePullPolicy: genericString,
							},
							{
								patch_scheduler.LabelContainerCommand: genericStringList,
							},
							{
								patch_scheduler.LabelContainerEnvironment: []game_room.ContainerEnvironment{
									{
										Name:  genericString,
										Value: genericString,
									},
								},
							},
							{
								patch_scheduler.LabelContainerRequests: game_room.ContainerResources{
									CPU:    "100",
									Memory: "1000m",
								},
							},
							{
								patch_scheduler.LabelContainerLimits: game_room.ContainerResources{
									CPU:    "200",
									Memory: "2000m",
								},
							},
							{
								patch_scheduler.LabelContainerPorts: []game_room.ContainerPort{
									{
										Name:     "some-port",
										Protocol: "TCP",
										Port:     int(123),
										HostPort: int(1234),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Title: "only toleration should convert api.PatchSchedulerRequest to change map",
			Input: Input{
				PatchScheduler: &api.PatchSchedulerRequest{
					Spec: &api.OptionalSpec{Toleration: &genericString},
				},
			},
			Output: Output{
				PatchScheduler: map[string]interface{}{
					patch_scheduler.LabelSchedulerSpec: map[string]interface{}{
						patch_scheduler.LabelSpecToleration: genericString,
					},
				},
			},
		},
		{
			Title: "only affinity should convert api.PatchSchedulerRequest to change map",
			Input: Input{
				PatchScheduler: &api.PatchSchedulerRequest{
					Spec: &api.OptionalSpec{Affinity: &genericString},
				},
			},
			Output: Output{
				PatchScheduler: map[string]interface{}{
					patch_scheduler.LabelSchedulerSpec: map[string]interface{}{
						patch_scheduler.LabelSpecAffinity: genericString,
					},
				},
			},
		},
		{
			Title: "only autoscaling should convert api.PatchSchedulerRequest to change map",
			Input: Input{
				PatchScheduler: &api.PatchSchedulerRequest{
					Autoscaling: &api.Autoscaling{
						Enabled: true,
						Min:     int32(1),
						Max:     int32(5),
						Policy: &api.AutoscalingPolicy{
							Type: "roomOccupancy",
							Parameters: &api.PolicyParameters{
								RoomOccupancy: &api.RoomOccupancy{
									ReadyTarget: genericFloat32,
								},
							},
						},
					},
				},
			},
			Output: Output{
				PatchScheduler: map[string]interface{}{
					patch_scheduler.LabelAutoscaling: &autoscaling.Autoscaling{
						Enabled: true,
						Min:     1,
						Max:     5,
						Policy: autoscaling.Policy{
							Type: autoscaling.RoomOccupancy,
							Parameters: autoscaling.PolicyParameters{
								RoomOccupancy: &autoscaling.RoomOccupancyParams{
									ReadyTarget: float64(genericFloat32),
								},
							},
						},
					},
				},
			},
		},
		{
			Title: "unknown autoscaling - return error",
			Input: Input{
				PatchScheduler: &api.PatchSchedulerRequest{
					Autoscaling: &api.Autoscaling{
						Enabled: true,
						Min:     int32(1),
						Max:     int32(5),
						Policy: &api.AutoscalingPolicy{
							Type: "UNKNOWN",
							Parameters: &api.PolicyParameters{
								RoomOccupancy: &api.RoomOccupancy{
									ReadyTarget: genericFloat32,
								},
							},
						},
					},
				},
			},
			Output: Output{
				PatchScheduler: map[string]interface{}{},
				Error:          true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			returnValues, err := requestadapters.FromApiPatchSchedulerRequestToChangeMap(testCase.Input.PatchScheduler)
			if testCase.Output.Error {
				assert.Error(t, err)
			}
			assert.EqualValues(t, testCase.Output.PatchScheduler, returnValues)
		})
	}
}

func TestFromApiCreateSchedulerRequestToEntity(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	type Input struct {
		CreateScheduler *api.CreateSchedulerRequest
	}

	type Output struct {
		Scheduler *entities.Scheduler
	}

	genericInt32 := int32(62)
	genericInt := 62
	maxSurgeValue := "60%"
	roomsReplicasValue := int32(2)
	genericString := "some-value"
	genericValidVersion := "v1.0.0"
	genericStringList := []string{"some-value", "another-value"}
	genericFloat32 := float32(0.3)

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "convert create api scheduler to entity scheduler",
			Input: Input{
				CreateScheduler: &api.CreateSchedulerRequest{
					Name: genericString,
					Game: genericString,
					Spec: &api.Spec{
						Version:                genericValidVersion,
						TerminationGracePeriod: 10,
						Toleration:             genericString,
						Affinity:               genericString,
						Containers: []*api.Container{
							{
								Name:            genericString,
								Image:           genericString,
								ImagePullPolicy: genericString,
								Command:         genericStringList,
								Environment: []*api.ContainerEnvironment{
									{
										Name:  genericString,
										Value: &genericString,
									},
									{
										Name: genericString,
										ValueFrom: &api.ContainerEnvironmentValueFrom{
											FieldRef: &api.ContainerEnvironmentValueFromFieldRef{FieldPath: genericString},
										},
									},
									{
										Name: genericString,
										ValueFrom: &api.ContainerEnvironmentValueFrom{
											SecretKeyRef: &api.ContainerEnvironmentValueFromSecretKeyRef{
												Name: genericString,
												Key:  genericString,
											},
										},
									},
								},
								Requests: &api.ContainerResources{
									Memory: "1000m",
									Cpu:    "100",
								},
								Limits: &api.ContainerResources{
									Memory: "1000m",
									Cpu:    "100",
								},
								Ports: []*api.ContainerPort{
									{
										Name:     genericString,
										Port:     genericInt32,
										Protocol: "TCP",
									},
								},
							},
						},
					},
					PortRange: &api.PortRange{
						Start: 10000,
						End:   60000,
					},
					MaxSurge:      maxSurgeValue,
					RoomsReplicas: roomsReplicasValue,
					Autoscaling: &api.Autoscaling{
						Enabled: true,
						Min:     int32(1),
						Max:     int32(5),
						Policy: &api.AutoscalingPolicy{
							Type: "roomOccupancy",
							Parameters: &api.PolicyParameters{
								RoomOccupancy: &api.RoomOccupancy{
									ReadyTarget: genericFloat32,
								},
							},
						},
					},
					Forwarders: []*api.Forwarder{
						{
							Name:    "some-forwarder",
							Enable:  true,
							Type:    "some-type",
							Address: "localhost:8080",
							Options: &api.ForwarderOptions{
								Timeout: int64(10),
							},
						},
						{
							Name:    "another-forwarder",
							Enable:  false,
							Type:    "another-type",
							Address: "localhost:8888",
							Options: &api.ForwarderOptions{
								Timeout: int64(10),
							},
						},
					},
				},
			},
			Output: Output{
				Scheduler: &entities.Scheduler{
					Name:  genericString,
					Game:  genericString,
					State: entities.StateCreating,
					Spec: game_room.Spec{
						Version:                genericValidVersion,
						TerminationGracePeriod: time.Duration(10),
						Containers: []game_room.Container{
							{
								Name:            genericString,
								Image:           genericString,
								ImagePullPolicy: genericString,
								Command:         genericStringList,
								Environment: []game_room.ContainerEnvironment{
									{
										Name:  genericString,
										Value: genericString,
									},
									{
										Name: genericString,
										ValueFrom: &game_room.ValueFrom{
											FieldRef: &game_room.FieldRef{FieldPath: genericString},
										},
									},
									{
										Name: genericString,
										ValueFrom: &game_room.ValueFrom{
											SecretKeyRef: &game_room.SecretKeyRef{
												Name: genericString,
												Key:  genericString,
											},
										},
									},
								},
								Requests: game_room.ContainerResources{
									Memory: "1000m",
									CPU:    "100",
								},
								Limits: game_room.ContainerResources{
									Memory: "1000m",
									CPU:    "100",
								},
								Ports: []game_room.ContainerPort{
									{
										Name:     genericString,
										Port:     genericInt,
										Protocol: "TCP",
									},
								},
							},
						},
						Toleration: genericString,
						Affinity:   genericString,
					},
					PortRange: &entities.PortRange{
						Start: 10000,
						End:   60000,
					},
					MaxSurge:      maxSurgeValue,
					RoomsReplicas: int(roomsReplicasValue),
					Autoscaling: &autoscaling.Autoscaling{
						Enabled: true,
						Min:     1,
						Max:     5,
						Policy: autoscaling.Policy{
							Type: autoscaling.RoomOccupancy,
							Parameters: autoscaling.PolicyParameters{
								RoomOccupancy: &autoscaling.RoomOccupancyParams{
									ReadyTarget: float64(genericFloat32),
								},
							},
						},
					},
					Forwarders: []*forwarder.Forwarder{
						{
							Name:        "some-forwarder",
							Enabled:     true,
							ForwardType: forwarder.ForwardType("some-type"),
							Address:     "localhost:8080",
							Options: &forwarder.ForwardOptions{
								Timeout:  time.Duration(10),
								Metadata: map[string]interface{}{},
							},
						},
						{
							Name:        "another-forwarder",
							Enabled:     false,
							ForwardType: forwarder.ForwardType("another-type"),
							Address:     "localhost:8888",
							Options: &forwarder.ForwardOptions{
								Timeout:  time.Duration(10),
								Metadata: map[string]interface{}{},
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			_scheduler, _ := requestadapters.FromApiCreateSchedulerRequestToEntity(testCase.Input.CreateScheduler)
			assert.EqualValues(t, testCase.Output.Scheduler, _scheduler)
		})
	}
}

func TestFromEntitySchedulerToListResponse(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	type Input struct {
		Scheduler *entities.Scheduler
	}

	type Output struct {
		SchedulerWithoutSpec *api.SchedulerWithoutSpec
	}

	genericInt := 62
	maxSurgeValue := "60%"
	roomsReplicasValue := int32(6)
	genericString := "some-value"
	genericValidVersion := "v1.0.0"
	genericStringList := []string{"some-value", "another-value"}
	genericTime := time.Now()

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "receives entity scheduler without portRange, return api scheduler without spec",
			Input: Input{
				Scheduler: &entities.Scheduler{
					Name:      genericString,
					Game:      genericString,
					State:     entities.StateCreating,
					CreatedAt: genericTime,
					Spec: game_room.Spec{
						Version:                genericValidVersion,
						TerminationGracePeriod: time.Duration(10),
						Containers: []game_room.Container{
							{
								Name:            genericString,
								Image:           genericString,
								ImagePullPolicy: genericString,
								Command:         genericStringList,
								Environment: []game_room.ContainerEnvironment{
									{
										Name:  genericString,
										Value: genericString,
									},
									{
										Name: genericString,
										ValueFrom: &game_room.ValueFrom{
											FieldRef: &game_room.FieldRef{FieldPath: genericString},
										},
									},
									{
										Name: genericString,
										ValueFrom: &game_room.ValueFrom{
											SecretKeyRef: &game_room.SecretKeyRef{
												Name: genericString,
												Key:  genericString,
											},
										},
									},
								},
								Requests: game_room.ContainerResources{
									Memory: "1000m",
									CPU:    "100",
								},
								Limits: game_room.ContainerResources{
									Memory: "1000m",
									CPU:    "100",
								},
								Ports: []game_room.ContainerPort{
									{
										Name:     genericString,
										Port:     genericInt,
										Protocol: "TCP",
									},
								},
							},
						},
						Toleration: genericString,
						Affinity:   genericString,
					},
					PortRange:     nil,
					MaxSurge:      maxSurgeValue,
					RoomsReplicas: int(roomsReplicasValue),
					Forwarders: []*forwarder.Forwarder{
						{
							Name:        "some-forwarder",
							Enabled:     true,
							ForwardType: forwarder.ForwardType("some-type"),
							Address:     "localhost:8080",
							Options: &forwarder.ForwardOptions{
								Timeout:  time.Duration(10),
								Metadata: map[string]interface{}{},
							},
						},
						{
							Name:        "another-forwarder",
							Enabled:     false,
							ForwardType: forwarder.ForwardType("another-type"),
							Address:     "localhost:8888",
							Options: &forwarder.ForwardOptions{
								Timeout:  time.Duration(10),
								Metadata: map[string]interface{}{},
							},
						},
					},
				},
			},
			Output: Output{
				SchedulerWithoutSpec: &api.SchedulerWithoutSpec{
					Name:          genericString,
					Game:          genericString,
					State:         "creating",
					Version:       genericValidVersion,
					PortRange:     nil,
					CreatedAt:     timestamppb.New(genericTime),
					MaxSurge:      maxSurgeValue,
					RoomsReplicas: roomsReplicasValue,
				},
			},
		},
		{
			Title: "receives entity scheduler with portRange, return api scheduler without spec",
			Input: Input{
				Scheduler: &entities.Scheduler{
					Name:      genericString,
					Game:      genericString,
					State:     entities.StateCreating,
					CreatedAt: genericTime,
					Spec: game_room.Spec{
						Version:                genericValidVersion,
						TerminationGracePeriod: time.Duration(10),
						Containers: []game_room.Container{
							{
								Name:            genericString,
								Image:           genericString,
								ImagePullPolicy: genericString,
								Command:         genericStringList,
								Environment: []game_room.ContainerEnvironment{
									{
										Name:  genericString,
										Value: genericString,
									},
									{
										Name: genericString,
										ValueFrom: &game_room.ValueFrom{
											FieldRef: &game_room.FieldRef{FieldPath: genericString},
										},
									},
									{
										Name: genericString,
										ValueFrom: &game_room.ValueFrom{
											SecretKeyRef: &game_room.SecretKeyRef{
												Name: genericString,
												Key:  genericString,
											},
										},
									},
								},
								Requests: game_room.ContainerResources{
									Memory: "1000m",
									CPU:    "100",
								},
								Limits: game_room.ContainerResources{
									Memory: "1000m",
									CPU:    "100",
								},
								Ports: []game_room.ContainerPort{
									{
										Name:     genericString,
										Port:     genericInt,
										Protocol: "TCP",
									},
								},
							},
						},
						Toleration: genericString,
						Affinity:   genericString,
					},
					PortRange: &entities.PortRange{
						Start: 10000,
						End:   60000,
					},
					MaxSurge:      maxSurgeValue,
					RoomsReplicas: int(roomsReplicasValue),
					Forwarders: []*forwarder.Forwarder{
						{
							Name:        "some-forwarder",
							Enabled:     true,
							ForwardType: forwarder.ForwardType("some-type"),
							Address:     "localhost:8080",
							Options: &forwarder.ForwardOptions{
								Timeout:  time.Duration(10),
								Metadata: map[string]interface{}{},
							},
						},
						{
							Name:        "another-forwarder",
							Enabled:     false,
							ForwardType: forwarder.ForwardType("another-type"),
							Address:     "localhost:8888",
							Options: &forwarder.ForwardOptions{
								Timeout:  time.Duration(10),
								Metadata: map[string]interface{}{},
							},
						},
					},
				},
			},
			Output: Output{
				SchedulerWithoutSpec: &api.SchedulerWithoutSpec{
					Name:    genericString,
					Game:    genericString,
					State:   "creating",
					Version: genericValidVersion,
					PortRange: &api.PortRange{
						Start: 10000,
						End:   60000,
					},
					CreatedAt:     timestamppb.New(genericTime),
					MaxSurge:      maxSurgeValue,
					RoomsReplicas: roomsReplicasValue,
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			returnValues := requestadapters.FromEntitySchedulerToListResponse(testCase.Input.Scheduler)

			assert.EqualValues(t, testCase.Output.SchedulerWithoutSpec, returnValues)
		})
	}
}

func TestFromApiNewSchedulerVersionRequestToEntity(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	type Input struct {
		NewSchedulerVersion *api.NewSchedulerVersionRequest
	}

	type Output struct {
		Scheduler *entities.Scheduler
	}

	genericInt32 := int32(62)
	genericInt := 62
	maxSurgeValue := "60%"
	roomsReplicasValue := int32(6)
	genericString := "some-value"
	genericValidVersion := "v1.0.0"
	genericStringList := []string{"some-value", "another-value"}
	genericFloat32 := float32(0.3)

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "convert newSchedulerVersion api scheduler to entity scheduler",
			Input: Input{
				NewSchedulerVersion: &api.NewSchedulerVersionRequest{
					Name: genericString,
					Game: genericString,
					Spec: &api.Spec{
						Version:                genericValidVersion,
						TerminationGracePeriod: 10,
						Toleration:             genericString,
						Affinity:               genericString,
						Containers: []*api.Container{
							{
								Name:            genericString,
								Image:           genericString,
								ImagePullPolicy: genericString,
								Command:         genericStringList,
								Environment: []*api.ContainerEnvironment{
									{
										Name:  genericString,
										Value: &genericString,
									},
									{
										Name: genericString,
										ValueFrom: &api.ContainerEnvironmentValueFrom{
											FieldRef: &api.ContainerEnvironmentValueFromFieldRef{FieldPath: genericString},
										},
									},
									{
										Name: genericString,
										ValueFrom: &api.ContainerEnvironmentValueFrom{
											SecretKeyRef: &api.ContainerEnvironmentValueFromSecretKeyRef{
												Name: genericString,
												Key:  genericString,
											},
										},
									},
								},
								Requests: &api.ContainerResources{
									Memory: "1000m",
									Cpu:    "100",
								},
								Limits: &api.ContainerResources{
									Memory: "1000m",
									Cpu:    "100",
								},
								Ports: []*api.ContainerPort{
									{
										Name:     genericString,
										Port:     genericInt32,
										Protocol: "TCP",
									},
								},
							},
						},
					},
					PortRange: &api.PortRange{
						Start: 10000,
						End:   60000,
					},
					MaxSurge:      maxSurgeValue,
					RoomsReplicas: roomsReplicasValue,
					Autoscaling: &api.Autoscaling{
						Enabled: true,
						Min:     int32(1),
						Max:     int32(5),
						Policy: &api.AutoscalingPolicy{
							Type: "roomOccupancy",
							Parameters: &api.PolicyParameters{
								RoomOccupancy: &api.RoomOccupancy{
									ReadyTarget: genericFloat32,
								},
							},
						},
					},
					Forwarders: []*api.Forwarder{
						{
							Name:    "some-forwarder",
							Enable:  true,
							Type:    "some-type",
							Address: "localhost:8080",
							Options: &api.ForwarderOptions{
								Timeout: int64(10),
							},
						},
						{
							Name:    "another-forwarder",
							Enable:  false,
							Type:    "another-type",
							Address: "localhost:8888",
							Options: &api.ForwarderOptions{
								Timeout: int64(10),
							},
						},
					},
				},
			},
			Output: Output{
				Scheduler: &entities.Scheduler{
					Name:  genericString,
					Game:  genericString,
					State: entities.StateCreating,
					Spec: game_room.Spec{
						Version:                genericValidVersion,
						TerminationGracePeriod: time.Duration(10),
						Containers: []game_room.Container{
							{
								Name:            genericString,
								Image:           genericString,
								ImagePullPolicy: genericString,
								Command:         genericStringList,
								Environment: []game_room.ContainerEnvironment{
									{
										Name:  genericString,
										Value: genericString,
									},
									{
										Name: genericString,
										ValueFrom: &game_room.ValueFrom{
											FieldRef: &game_room.FieldRef{FieldPath: genericString},
										},
									},
									{
										Name: genericString,
										ValueFrom: &game_room.ValueFrom{
											SecretKeyRef: &game_room.SecretKeyRef{
												Name: genericString,
												Key:  genericString,
											},
										},
									},
								},
								Requests: game_room.ContainerResources{
									Memory: "1000m",
									CPU:    "100",
								},
								Limits: game_room.ContainerResources{
									Memory: "1000m",
									CPU:    "100",
								},
								Ports: []game_room.ContainerPort{
									{
										Name:     genericString,
										Port:     genericInt,
										Protocol: "TCP",
									},
								},
							},
						},
						Toleration: genericString,
						Affinity:   genericString,
					},
					PortRange: &entities.PortRange{
						Start: 10000,
						End:   60000,
					},
					MaxSurge:      maxSurgeValue,
					RoomsReplicas: int(roomsReplicasValue),
					Autoscaling: &autoscaling.Autoscaling{
						Enabled: true,
						Min:     1,
						Max:     5,
						Policy: autoscaling.Policy{
							Type: autoscaling.RoomOccupancy,
							Parameters: autoscaling.PolicyParameters{
								RoomOccupancy: &autoscaling.RoomOccupancyParams{
									ReadyTarget: float64(genericFloat32),
								},
							},
						},
					},
					Forwarders: []*forwarder.Forwarder{
						{
							Name:        "some-forwarder",
							Enabled:     true,
							ForwardType: forwarder.ForwardType("some-type"),
							Address:     "localhost:8080",
							Options: &forwarder.ForwardOptions{
								Timeout:  time.Duration(10),
								Metadata: map[string]interface{}{},
							},
						},
						{
							Name:        "another-forwarder",
							Enabled:     false,
							ForwardType: forwarder.ForwardType("another-type"),
							Address:     "localhost:8888",
							Options: &forwarder.ForwardOptions{
								Timeout:  time.Duration(10),
								Metadata: map[string]interface{}{},
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			_scheduler, _ := requestadapters.FromApiNewSchedulerVersionRequestToEntity(testCase.Input.NewSchedulerVersion)
			assert.EqualValues(t, testCase.Output.Scheduler, _scheduler)
		})
	}
}

func TestFromEntitySchedulerToResponse(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	type Input struct {
		Scheduler *entities.Scheduler
	}

	type Output struct {
		ApiScheduler *api.Scheduler
	}

	genericInt32 := int32(62)
	genericInt := 62
	maxSurgeValue := "60%"
	roomsReplicasValue := int32(6)
	genericString := "some-value"
	genericValidVersion := "v1.0.0"
	genericStringList := []string{"some-value", "another-value"}
	genericTime := time.Now()

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "receives entity scheduler return api scheduler",
			Input: Input{
				Scheduler: &entities.Scheduler{
					Name:      genericString,
					Game:      genericString,
					State:     entities.StateCreating,
					CreatedAt: genericTime,
					Spec: game_room.Spec{
						Version:                genericValidVersion,
						TerminationGracePeriod: time.Duration(10),
						Containers: []game_room.Container{
							{
								Name:            genericString,
								Image:           genericString,
								ImagePullPolicy: genericString,
								Command:         genericStringList,
								Environment: []game_room.ContainerEnvironment{
									{
										Name:  genericString,
										Value: genericString,
									},
									{
										Name: genericString,
										ValueFrom: &game_room.ValueFrom{
											FieldRef: &game_room.FieldRef{FieldPath: genericString},
										},
									},
									{
										Name: genericString,
										ValueFrom: &game_room.ValueFrom{
											SecretKeyRef: &game_room.SecretKeyRef{
												Name: genericString,
												Key:  genericString,
											},
										},
									},
								},
								Requests: game_room.ContainerResources{
									Memory: "1000m",
									CPU:    "100",
								},
								Limits: game_room.ContainerResources{
									Memory: "1000m",
									CPU:    "100",
								},
								Ports: []game_room.ContainerPort{
									{
										Name:     genericString,
										Port:     genericInt,
										Protocol: "TCP",
									},
								},
							},
						},
						Toleration: genericString,
						Affinity:   genericString,
					},
					PortRange: &entities.PortRange{
						Start: 10000,
						End:   60000,
					},
					MaxSurge:      maxSurgeValue,
					RoomsReplicas: int(roomsReplicasValue),
					Autoscaling: &autoscaling.Autoscaling{
						Enabled: true,
						Min:     1,
						Max:     5,
						Policy: autoscaling.Policy{
							Type: autoscaling.RoomOccupancy,
							Parameters: autoscaling.PolicyParameters{
								RoomOccupancy: &autoscaling.RoomOccupancyParams{
									ReadyTarget: 0.3,
								},
							},
						},
					},
					Forwarders: []*forwarder.Forwarder{
						{
							Name:        "some-forwarder",
							Enabled:     true,
							ForwardType: forwarder.ForwardType("some-type"),
							Address:     "localhost:8080",
							Options: &forwarder.ForwardOptions{
								Timeout:  time.Duration(10),
								Metadata: map[string]interface{}{"some-value": "another-value"},
							},
						},
					},
				},
			},
			Output: Output{
				ApiScheduler: &api.Scheduler{
					Name:      genericString,
					Game:      genericString,
					State:     "creating",
					CreatedAt: timestamppb.New(genericTime),
					Spec: &api.Spec{
						Version:                genericValidVersion,
						TerminationGracePeriod: 10,
						Containers: []*api.Container{
							{
								Name:            genericString,
								Image:           genericString,
								ImagePullPolicy: genericString,
								Command:         genericStringList,
								Environment: []*api.ContainerEnvironment{
									{
										Name:  genericString,
										Value: &genericString,
									},
									{
										Name: genericString,
										ValueFrom: &api.ContainerEnvironmentValueFrom{
											FieldRef: &api.ContainerEnvironmentValueFromFieldRef{FieldPath: genericString},
										},
									},
									{
										Name: genericString,
										ValueFrom: &api.ContainerEnvironmentValueFrom{
											SecretKeyRef: &api.ContainerEnvironmentValueFromSecretKeyRef{
												Name: genericString,
												Key:  genericString,
											},
										},
									},
								},
								Requests: &api.ContainerResources{
									Memory: "1000m",
									Cpu:    "100",
								},
								Limits: &api.ContainerResources{
									Memory: "1000m",
									Cpu:    "100",
								},
								Ports: []*api.ContainerPort{
									{
										Name:     genericString,
										Port:     genericInt32,
										Protocol: "TCP",
									},
								},
							},
						},
						Toleration: genericString,
						Affinity:   genericString,
					},
					PortRange: &api.PortRange{
						Start: 10000,
						End:   60000,
					},
					MaxSurge:      maxSurgeValue,
					RoomsReplicas: roomsReplicasValue,
					Autoscaling: &api.Autoscaling{
						Enabled: true,
						Min:     int32(1),
						Max:     int32(5),
						Policy: &api.AutoscalingPolicy{
							Type: "roomOccupancy",
							Parameters: &api.PolicyParameters{
								RoomOccupancy: &api.RoomOccupancy{
									ReadyTarget: float32(0.3),
								},
							},
						},
					},
					Forwarders: []*api.Forwarder{
						{
							Name:    "some-forwarder",
							Enable:  true,
							Type:    "some-type",
							Address: "localhost:8080",
							Options: &api.ForwarderOptions{
								Timeout: int64(10),
								Metadata: &structpb.Struct{
									Fields: map[string]*structpb.Value{
										"some-value": {
											Kind: &structpb.Value_StringValue{
												StringValue: "another-value",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			returnValues, _ := requestadapters.FromEntitySchedulerToResponse(testCase.Input.Scheduler)

			assert.EqualValues(t, testCase.Output.ApiScheduler, returnValues)
		})
	}
}

func TestFromEntitySchedulerVersionListToResponse(t *testing.T) {
	type Input struct {
		SchedulerVersionList []*entities.SchedulerVersion
	}

	type Output struct {
		ApiSchedulerVersionList []*api.SchedulerVersion
	}

	genericTime := time.Now()

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "List have no value returns empty list successfully",
			Input: Input{
				SchedulerVersionList: []*entities.SchedulerVersion{},
			},
			Output: Output{
				ApiSchedulerVersionList: []*api.SchedulerVersion{},
			},
		},
		{
			Title: "List have 1 value returns converted list successfully",
			Input: Input{
				SchedulerVersionList: []*entities.SchedulerVersion{
					{
						Version:   "v1.0.0",
						IsActive:  true,
						CreatedAt: genericTime,
					},
				},
			},
			Output: Output{
				ApiSchedulerVersionList: []*api.SchedulerVersion{
					{
						Version:   "v1.0.0",
						IsActive:  true,
						CreatedAt: timestamppb.New(genericTime),
					},
				},
			},
		},
		{
			Title: "List have more than 1 value returns converted list successfully",
			Input: Input{
				SchedulerVersionList: []*entities.SchedulerVersion{
					{
						Version:   "v1.1.0",
						IsActive:  true,
						CreatedAt: genericTime,
					},
					{
						Version:   "v1.0.0",
						IsActive:  false,
						CreatedAt: genericTime.Add(-time.Hour * 24),
					},
				},
			},
			Output: Output{
				ApiSchedulerVersionList: []*api.SchedulerVersion{
					{
						Version:   "v1.1.0",
						IsActive:  true,
						CreatedAt: timestamppb.New(genericTime),
					},
					{
						Version:   "v1.0.0",
						IsActive:  false,
						CreatedAt: timestamppb.New(genericTime.Add(-time.Hour * 24)),
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			returnValues := requestadapters.FromEntitySchedulerVersionListToResponse(testCase.Input.SchedulerVersionList)
			assert.EqualValues(t, testCase.Output.ApiSchedulerVersionList, returnValues)
		})
	}
}

func TestFromEntitySchedulerInfoToListResponse(t *testing.T) {
	type Input struct {
		SchedulerInfo *entities.SchedulerInfo
	}

	type Output struct {
		ApiSchedulerInfo *api.SchedulerInfo
	}

	genericString := "some-value"
	genericInt := 62

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "only spec should convert api.PatchSchedulerRequest to change map",
			Input: Input{
				SchedulerInfo: &entities.SchedulerInfo{
					Name:             genericString,
					Game:             genericString,
					State:            entities.StateCreating,
					RoomsReplicas:    genericInt,
					RoomsReady:       genericInt,
					RoomsOccupied:    genericInt,
					RoomsPending:     genericInt,
					RoomsTerminating: genericInt,
				},
			},
			Output: Output{
				ApiSchedulerInfo: &api.SchedulerInfo{
					Name:             genericString,
					Game:             genericString,
					State:            "creating",
					RoomsReplicas:    int32(genericInt),
					RoomsReady:       int32(genericInt),
					RoomsOccupied:    int32(genericInt),
					RoomsPending:     int32(genericInt),
					RoomsTerminating: int32(genericInt),
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			returnValues := requestadapters.FromEntitySchedulerInfoToListResponse(testCase.Input.SchedulerInfo)
			assert.EqualValues(t, testCase.Output.ApiSchedulerInfo, returnValues)
		})
	}
}
