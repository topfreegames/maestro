//go:build unit
// +build unit

package adapters_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/api/handlers/adapters"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager/patch_scheduler"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func TestFromApiContainerPort(t *testing.T) {
	type Input struct {
		Ports []*api.ContainerPort
	}

	type Output struct {
		Ports []game_room.ContainerPort
	}

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "should convert api.ContainerPort to game_room.ContainerPort",
			Input: Input{
				Ports: []*api.ContainerPort{
					{
						Name:     "some-port",
						Protocol: "TCP",
						Port:     72,
						HostPort: 1234,
					},
					{
						Name:     "another-port",
						Protocol: "UDP",
						Port:     73,
						HostPort: 12345,
					},
				},
			},
			Output: Output{
				Ports: []game_room.ContainerPort{
					{
						Name:     "some-port",
						Protocol: "TCP",
						Port:     72,
						HostPort: 1234,
					},
					{
						Name:     "another-port",
						Protocol: "UDP",
						Port:     73,
						HostPort: 12345,
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			returnValues := adapters.FromApiContainerPorts(testCase.Input.Ports)
			assert.EqualValues(t, testCase.Output.Ports, returnValues)
		})
	}
}

func TestFromApiContainerEnvironments(t *testing.T) {
	type Input struct {
		Environments []*api.ContainerEnvironment
	}

	type Output struct {
		Environments []game_room.ContainerEnvironment
	}

	value := "some-value"

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "should convert api.ContainerEnvironment to game_room.ContainerEnvironment",
			Input: Input{
				Environments: []*api.ContainerEnvironment{
					{
						Name:  "some-environment",
						Value: &value,
					},
					{
						Name: "another-environment",
						ValueFrom: &api.ContainerEnvironmentValueFrom{
							FieldRef: &api.ContainerEnvironmentValueFromFieldRef{
								FieldPath: "some-field-path",
							},
						},
					},
					{
						Name: "another-environment",
						ValueFrom: &api.ContainerEnvironmentValueFrom{
							SecretKeyRef: &api.ContainerEnvironmentValueFromSecretKeyRef{
								Name: "some-name",
								Key:  "some-key",
							},
						},
					},
				},
			},
			Output: Output{
				Environments: []game_room.ContainerEnvironment{
					{
						Name:  "some-environment",
						Value: "some-value",
					},
					{
						Name: "another-environment",
						ValueFrom: &game_room.ValueFrom{
							FieldRef: &game_room.FieldRef{
								FieldPath: "some-field-path",
							},
						},
					},
					{
						Name: "another-environment",
						ValueFrom: &game_room.ValueFrom{
							SecretKeyRef: &game_room.SecretKeyRef{
								Name: "some-name",
								Key:  "some-key",
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			returnValues := adapters.FromApiContainerEnvironments(testCase.Input.Environments)
			assert.EqualValues(t, testCase.Output.Environments, returnValues)
		})
	}
}

func TestFromApiForwarders(t *testing.T) {
	type Input struct {
		Forwarders []*api.Forwarder
	}

	type Output struct {
		Forwarders []*forwarder.Forwarder
	}

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "should convert api.Forwarder to forwarder.Forwarder",
			Input: Input{
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
			Output: Output{
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
		{
			Title: "should convert api.Forwarder to forwarder.Forwarder when options is nil",
			Input: Input{
				Forwarders: []*api.Forwarder{
					{
						Name:    "some-forwarder",
						Enable:  true,
						Type:    "some-type",
						Address: "localhost:8080",
						Options: nil,
					},
				},
			},
			Output: Output{
				Forwarders: []*forwarder.Forwarder{
					{
						Name:        "some-forwarder",
						Enabled:     true,
						ForwardType: forwarder.ForwardType("some-type"),
						Address:     "localhost:8080",
						Options:     nil,
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			returnValues := adapters.FromApiForwarders(testCase.Input.Forwarders)
			assert.EqualValues(t, testCase.Output.Forwarders, returnValues)
		})
	}
}

func TestFromApiOptinalContainersToChangeMap(t *testing.T) {
	type Input struct {
		Containers []*api.OptionalContainer
	}

	type Output struct {
		Containers []map[string]interface{}
	}

	value := "some-value"

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "should convert api.OptionalContainer to change map",
			Input: Input{
				Containers: []*api.OptionalContainer{
					{
						Image: &value,
					},
					{
						ImagePullPolicy: &value,
					},
					{
						Command: []string{"/bin/sh", "command"},
					},
					{
						Environment: []*api.ContainerEnvironment{
							{
								Name:  "some-environment",
								Value: &value,
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
				},
			},
			Output: Output{
				Containers: []map[string]interface{}{
					map[string]interface{}{
						patch_scheduler.LabelContainerImage: value,
					},
					map[string]interface{}{
						patch_scheduler.LabelContainerImagePullPolicy: value,
					},
					map[string]interface{}{
						patch_scheduler.LabelContainerCommand: []string{"/bin/sh", "command"},
					},
					map[string]interface{}{
						patch_scheduler.LabelContainerEnvironment: []game_room.ContainerEnvironment{
							{
								Name:  "some-environment",
								Value: "some-value",
							},
						},
					},
					map[string]interface{}{
						patch_scheduler.LabelContainerRequests: game_room.ContainerResources{
							CPU:    "100",
							Memory: "1000m",
						},
					},
					map[string]interface{}{
						patch_scheduler.LabelContainerLimits: game_room.ContainerResources{
							CPU:    "200",
							Memory: "2000m",
						},
					},
					map[string]interface{}{
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
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			returnValues := adapters.FromApiOptinalContainersToChangeMap(testCase.Input.Containers)
			assert.EqualValues(t, testCase.Output.Containers, returnValues)
		})
	}
}

func TestFromApiOptionalSpecToChangeMap(t *testing.T) {
	type Input struct {
		Spec *api.OptionalSpec
	}

	type Output struct {
		Spec map[string]interface{}
	}

	terminationValue := int64(62)
	containerValue := "value"
	tolerationValue := "some-toleration"
	affinityValue := "some-affinity"

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "only termination grace period should convert api.OptionalSpec to change map",
			Input: Input{
				Spec: &api.OptionalSpec{
					TerminationGracePeriod: &terminationValue,
				},
			},
			Output: Output{
				Spec: map[string]interface{}{
					patch_scheduler.LabelSpecTerminationGracePeriod: time.Duration(terminationValue),
				},
			},
		},
		{
			Title: "only containers should convert api.OptionalSpec to change map",
			Input: Input{
				Spec: &api.OptionalSpec{
					Containers: []*api.OptionalContainer{
						{
							Image: &containerValue,
						},
					},
				},
			},
			Output: Output{
				Spec: map[string]interface{}{
					patch_scheduler.LabelSpecContainers: []map[string]interface{}{
						{
							patch_scheduler.LabelContainerImage: containerValue,
						},
					},
				},
			},
		},
		{
			Title: "only toleration should convert api.OptionalSpec to change map",
			Input: Input{
				Spec: &api.OptionalSpec{
					Toleration: &tolerationValue,
				},
			},
			Output: Output{
				Spec: map[string]interface{}{
					patch_scheduler.LabelSpecToleration: tolerationValue,
				},
			},
		},
		{
			Title: "only affinity should convert api.OptionalSpec to change map",
			Input: Input{
				Spec: &api.OptionalSpec{
					Affinity: &affinityValue,
				},
			},
			Output: Output{
				Spec: map[string]interface{}{
					patch_scheduler.LabelSpecAffinity: affinityValue,
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			returnValues := adapters.FromApiOptionalSpecToChangeMap(testCase.Input.Spec)
			assert.EqualValues(t, testCase.Output.Spec, returnValues)
		})
	}
}

func TestFromApiPatchSchedulerRequestToChangeMap(t *testing.T) {
	type Input struct {
		PatchScheduler *api.PatchSchedulerRequest
	}

	type Output struct {
		PatchScheduler map[string]interface{}
	}

	terminationValue := int64(62)
	maxSurgeValue := "60%"

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
							Options: &api.ForwarderOptions{
								Timeout: int64(10),
							},
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
			returnValues := adapters.FromApiPatchSchedulerRequestToChangeMap(testCase.Input.PatchScheduler)
			assert.EqualValues(t, testCase.Output.PatchScheduler, returnValues)
		})
	}
}
