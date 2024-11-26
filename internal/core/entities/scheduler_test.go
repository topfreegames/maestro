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

package entities_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/allocation"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/port"
	"github.com/topfreegames/maestro/internal/validations"
)

func TestNewScheduler(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}
	name := "scheduler-name"
	game := "scheduler-game"
	maxSurge := "10"
	roomsReplicas := 3
	containers := []game_room.Container{
		game_room.Container{
			Name:            "default",
			Image:           "some-image",
			ImagePullPolicy: "IfNotPresent",
			Command:         []string{"hello"},
			Ports: []game_room.ContainerPort{
				{Name: "tcp", Protocol: "tcp", Port: 80},
			},
			Requests: game_room.ContainerResources{
				CPU:    "10m",
				Memory: "100Mi",
			},
			Limits: game_room.ContainerResources{
				CPU:    "10m",
				Memory: "100Mi",
			},
		},
	}
	spec := *game_room.NewSpec(
		"v1",
		10,
		containers,
		"10",
		"10",
	)
	portRange := port.NewPortRange(
		1,
		2,
	)

	fwd := &forwarder.Forwarder{
		Name:        "fwd",
		Enabled:     true,
		ForwardType: forwarder.TypeGrpc,
		Address:     "address",
		Options: &forwarder.ForwardOptions{
			Timeout:  time.Second * 5,
			Metadata: nil,
		},
	}
	forwarders := []*forwarder.Forwarder{fwd}
	annotations := map[string]string{"imageregistry": "https://hub.docker.com/"}
	labels := map[string]string{"scheduler": "scheduler-name"}
	matchAllocation := allocation.MatchAllocation{MaxMatches: 1}

	t.Run("with success when create valid scheduler", func(t *testing.T) {
		scheduler, err := entities.NewScheduler(
			name,
			game,
			entities.StateCreating,
			maxSurge,
			spec,
			portRange,
			roomsReplicas,
			nil,
			forwarders,
			annotations,
			labels,
			matchAllocation)

		expectedScheduler := &entities.Scheduler{
			Name:            name,
			Game:            game,
			MaxSurge:        maxSurge,
			State:           entities.StateCreating,
			Spec:            spec,
			PortRange:       portRange,
			RoomsReplicas:   roomsReplicas,
			Autoscaling:     nil,
			Forwarders:      forwarders,
			Annotations:     annotations,
			Labels:          labels,
			MatchAllocation: matchAllocation,
		}

		require.NoError(t, err)
		require.EqualValues(t, expectedScheduler, scheduler)
	})

	t.Run("fails when try to create invalid scheduler", func(t *testing.T) {
		_, err := entities.NewScheduler(
			"",
			"",
			entities.StateCreating,
			maxSurge,
			spec,
			portRange,
			roomsReplicas,
			nil,
			forwarders, annotations, labels, matchAllocation)

		require.Error(t, err)
	})

	t.Run("fails when try to create scheduler with invalid RoomsReplicas", func(t *testing.T) {
		_, err := entities.NewScheduler(
			"",
			"",
			entities.StateCreating,
			"10",
			*game_room.NewSpec(
				"v1",
				10,
				containers,
				"10",
				"10",
			),
			port.NewPortRange(
				1,
				2,
			),
			0,
			nil,
			forwarders, annotations, labels, matchAllocation)

		require.Error(t, err)
	})

	t.Run("fails when try to create scheduler with invalid RoomsReplicas", func(t *testing.T) {
		_, err := entities.NewScheduler(
			"",
			"",
			entities.StateCreating,
			"10",
			*game_room.NewSpec(
				"v1",
				10,
				containers,
				"10",
				"10",
			),
			port.NewPortRange(
				1,
				2,
			),
			-1,
			nil,
			forwarders, annotations, labels, matchAllocation)

		require.Error(t, err)
	})

	t.Run("fails when try to create scheduler with invalid MatchAllocation", func(t *testing.T) {
		_, err := entities.NewScheduler(
			name,
			game,
			entities.StateCreating,
			maxSurge,
			spec,
			portRange,
			roomsReplicas,
			nil,
			forwarders,
			annotations,
			labels,
			allocation.MatchAllocation{})

		require.Error(t, err)
	})
}

func TestIsMajorVersion(t *testing.T) {
	tests := map[string]struct {
		currentScheduler *entities.Scheduler
		newScheduler     *entities.Scheduler
		expected         bool
	}{
		"port range should be a major update": {
			currentScheduler: &entities.Scheduler{PortRange: &port.PortRange{Start: 1000, End: 2000}},
			newScheduler:     &entities.Scheduler{PortRange: &port.PortRange{Start: 1001, End: 2000}},
			expected:         true,
		},
		"container resources should be a major update": {
			currentScheduler: &entities.Scheduler{Spec: game_room.Spec{
				Containers: []game_room.Container{
					{Requests: game_room.ContainerResources{Memory: "100mi"}},
				},
			}},
			newScheduler: &entities.Scheduler{Spec: game_room.Spec{
				Containers: []game_room.Container{
					{Requests: game_room.ContainerResources{Memory: "200mi"}},
				},
			}},
			expected: true,
		},
		"no changes shouldn't be a major": {
			currentScheduler: &entities.Scheduler{PortRange: &port.PortRange{Start: 1000, End: 2000}},
			newScheduler:     &entities.Scheduler{PortRange: &port.PortRange{Start: 1000, End: 2000}},
			expected:         false,
		},
		"max surge shouldn't be a major": {
			currentScheduler: &entities.Scheduler{MaxSurge: "10"},
			newScheduler:     &entities.Scheduler{MaxSurge: "100"},
			expected:         false,
		},
		"spec version shouldn't be a major": {
			currentScheduler: &entities.Scheduler{Spec: game_room.Spec{Version: "v1.1.0"}},
			newScheduler:     &entities.Scheduler{Spec: game_room.Spec{Version: "v1.2.0"}},
			expected:         false,
		},
		"host port shouldn't be a major": {
			currentScheduler: &entities.Scheduler{Spec: game_room.Spec{Containers: []game_room.Container{
				{
					Name: "Name1",
					Ports: []game_room.ContainerPort{
						{
							Name:     "port1",
							Protocol: "http",
							HostPort: 10,
						},
						{
							Name:     "port2",
							Protocol: "http",
							HostPort: 20,
						},
					}},
				{
					Name: "Name2",
					Ports: []game_room.ContainerPort{
						{
							Name:     "port1",
							Protocol: "http",
							HostPort: 30,
						},
						{
							Name:     "port2",
							Protocol: "http",
							HostPort: 40,
						},
					}},
			}}},
			newScheduler: &entities.Scheduler{Spec: game_room.Spec{Containers: []game_room.Container{
				{
					Name: "Name1",
					Ports: []game_room.ContainerPort{
						{
							Name:     "port1",
							Protocol: "http",
							HostPort: 91,
						},
						{
							Name:     "port2",
							Protocol: "http",
							HostPort: 92,
						},
					}},
				{
					Name: "Name2",
					Ports: []game_room.ContainerPort{
						{
							Name:     "port1",
							Protocol: "http",
							HostPort: 93,
						},
						{
							Name:     "port2",
							Protocol: "http",
							HostPort: 94,
						},
					}},
			}}},
			expected: false,
		},
		"roomsReplicas shouldn't be a major": {
			currentScheduler: &entities.Scheduler{RoomsReplicas: 0},
			newScheduler:     &entities.Scheduler{RoomsReplicas: 2},
			expected:         false,
		},
		"Autoscaling shouldn't be a major": {
			currentScheduler: &entities.Scheduler{Autoscaling: &autoscaling.Autoscaling{Enabled: false}},
			newScheduler:     &entities.Scheduler{Autoscaling: &autoscaling.Autoscaling{Enabled: true}},
			expected:         false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			isMajor := test.currentScheduler.IsMajorVersion(test.newScheduler)
			require.Equal(t, test.expected, isMajor)
		})
	}
}

func TestHasValidPortRangeConfiguration(t *testing.T) {
	tests := map[string]struct {
		scheduler *entities.Scheduler
		expected  error
	}{
		"should succeed if only scheduler.portrange is configured": {
			scheduler: &entities.Scheduler{PortRange: &port.PortRange{}},
			expected:  nil,
		},
		"should succeed if only scheduler.spec.container.ports.hostportrange is configured": {
			scheduler: &entities.Scheduler{
				Spec: game_room.Spec{
					Containers: []game_room.Container{
						{
							Ports: []game_room.ContainerPort{
								{
									HostPortRange: &port.PortRange{},
								},
							},
						},
					},
				},
			},
			expected: nil,
		},
		"should fail if neither scheduler.portrange nor container.ports.hostportrange are configured": {
			scheduler: &entities.Scheduler{},
			expected:  entities.ErrNoPortRangeConfigured,
		},
		"should fail if both scheduler.portrange and container.ports.hostportrange are configured": {
			scheduler: &entities.Scheduler{
				PortRange: &port.PortRange{},
				Spec: game_room.Spec{
					Containers: []game_room.Container{
						{
							Ports: []game_room.ContainerPort{
								{
									HostPortRange: &port.PortRange{},
								},
							},
						},
					},
				},
			},
			expected: entities.ErrBothPortRangesConfigured,
		},
		"should fail if not all container.ports have hostportrange configured": {
			scheduler: &entities.Scheduler{
				PortRange: &port.PortRange{},
				Spec: game_room.Spec{
					Containers: []game_room.Container{
						{
							Ports: []game_room.ContainerPort{
								{
									HostPortRange: &port.PortRange{},
								},
								{
									HostPortRange: nil,
								},
							},
						},
					},
				},
			},
			expected: entities.ErrBothPortRangesConfigured,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := test.scheduler.HasValidPortRangeConfiguration()
			require.ErrorIs(t, test.expected, err)
		})
	}
}
