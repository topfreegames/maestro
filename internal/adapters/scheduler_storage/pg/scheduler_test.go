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

package pg

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

func TestScheduler_ToScheduler(t *testing.T) {
	t.Run("valid schedulers", func(t *testing.T) {
		schedulers := []*entities.Scheduler{
			{
				Name:            "scheduler-1",
				Game:            "game",
				State:           entities.StateInSync,
				RollbackVersion: "v1",
				Spec: game_room.Spec{
					Version:                "v2",
					TerminationGracePeriod: 60,
					Toleration:             "toleration",
					Affinity:               "affinity",
				},
			},
			{
				Name:            "scheduler-2",
				Game:            "game",
				State:           entities.StateInSync,
				RollbackVersion: "v1",
				Spec: game_room.Spec{
					Version:                "v2",
					TerminationGracePeriod: 60,
					Toleration:             "toleration",
					Affinity:               "affinity",
				},
				PortRange: &entities.PortRange{
					Start: 40000,
					End:   60000,
				},
			},
			{
				Name:            "scheduler-3",
				Game:            "game",
				State:           entities.StateInSync,
				RollbackVersion: "v1",
				Spec: game_room.Spec{
					Version: "v2",
					Containers: []game_room.Container{
						{
							Name:            "game",
							Image:           "image",
							ImagePullPolicy: "always",
							Command:         []string{"ls", "/"},
							Environment: []game_room.ContainerEnvironment{
								{Name: "ENV_1", Value: "1"},
								{Name: "ENV_2", Value: "2"},
							},
							Requests: game_room.ContainerResources{
								Memory: "100",
								CPU:    "200",
							},
							Limits: game_room.ContainerResources{
								Memory: "200",
								CPU:    "400",
							},
							Ports: []game_room.ContainerPort{
								{
									Name:     "udp",
									Protocol: "UDP",
									Port:     1000,
								},
								{
									Name:     "udp",
									Protocol: "TCP",
									Port:     2000,
								},
							},
						},
					},
					Toleration:             "toleration",
					Affinity:               "affinity",
					TerminationGracePeriod: 60,
				},
			},
			{
				Name:            "scheduler-3",
				Game:            "game",
				State:           entities.StateInSync,
				RollbackVersion: "v1",
				Spec: game_room.Spec{
					Version: "v2",
					Containers: []game_room.Container{
						{
							Name:            "game",
							Image:           "image",
							ImagePullPolicy: "always",
							Command:         []string{"ls", "/"},
							Environment: []game_room.ContainerEnvironment{
								{Name: "ENV_1", Value: "1"},
								{Name: "ENV_2", Value: "2"},
							},
							Requests: game_room.ContainerResources{
								Memory: "100",
								CPU:    "200",
							},
							Limits: game_room.ContainerResources{
								Memory: "200",
								CPU:    "400",
							},
							Ports: []game_room.ContainerPort{
								{
									Name:     "udp",
									Protocol: "UDP",
									Port:     1000,
								},
								{
									Name:     "udp",
									Protocol: "TCP",
									Port:     2000,
								},
							},
						},
					},
					Toleration:             "toleration",
					Affinity:               "affinity",
					TerminationGracePeriod: 60,
				},
				PortRange: &entities.PortRange{
					Start: 40000,
					End:   60000,
				},
			},
		}

		for _, expectedScheduler := range schedulers {
			dbScheduler := NewDBScheduler(expectedScheduler)
			actualScheduler, err := dbScheduler.ToScheduler()
			require.NoError(t, err)
			require.Equal(t, expectedScheduler, actualScheduler)
		}
	})

	t.Run("invalid scheduler yaml", func(t *testing.T) {
		dbScheduler := Scheduler{
			ID:              "id",
			Name:            "scheduler",
			Game:            "game",
			Yaml:            "a b c d",
			State:           "creating",
			Version:         "v2",
			RollbackVersion: "v1",
		}
		_, err := dbScheduler.ToScheduler()
		require.Error(t, err)
	})
}
