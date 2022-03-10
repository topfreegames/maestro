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

package entities_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/validations"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

func TestNewScheduler(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}
	containers := []game_room.Container{
		game_room.Container{
			Name:            "default",
			Image:           "some-image",
			ImagePullPolicy: "Always",
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
		}}

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

	t.Run("with success when create valid scheduler", func(t *testing.T) {
		scheduler, err := entities.NewScheduler(
			"scheduler-name",
			"scheduler-game",
			entities.StateCreating,
			"10",
			*game_room.NewSpec(
				"v1",
				10,
				containers,
				"10",
				"10",
			),
			entities.NewPortRange(
				1,
				2,
			),
			forwarders)

		require.NoError(t, err)
		require.NotNil(t, scheduler)
	})

	t.Run("fails when try to create invalid scheduler", func(t *testing.T) {
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
			entities.NewPortRange(
				1,
				2,
			),
			forwarders)

		require.NotNil(t, err)
	})
}

func TestIsMajorVersion(t *testing.T) {
	tests := map[string]struct {
		currentScheduler *entities.Scheduler
		newScheduler     *entities.Scheduler
		expected         bool
	}{
		"port range should be a major update": {
			currentScheduler: &entities.Scheduler{PortRange: &entities.PortRange{Start: 1000, End: 2000}},
			newScheduler:     &entities.Scheduler{PortRange: &entities.PortRange{Start: 1001, End: 2000}},
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
			currentScheduler: &entities.Scheduler{PortRange: &entities.PortRange{Start: 1000, End: 2000}},
			newScheduler:     &entities.Scheduler{PortRange: &entities.PortRange{Start: 1000, End: 2000}},
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
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			isMajor := test.currentScheduler.IsMajorVersion(test.newScheduler)
			require.Equal(t, test.expected, isMajor)
		})
	}
}
