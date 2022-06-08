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

package game_room_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/validations"
)

func TestNewSpec(t *testing.T) {
	err := validations.RegisterValidations()
	require.NoError(t, err)

	t.Run("with success", func(t *testing.T) {
		t.Run("when create a new spec with all fields", func(t *testing.T) {
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

			expectedSpec := &game_room.Spec{
				Version:                "v1",
				TerminationGracePeriod: time.Duration(10),
				Containers:             containers,
				Toleration:             "10",
				Affinity:               "10",
			}

			spec := game_room.NewSpec(
				expectedSpec.Version,
				expectedSpec.TerminationGracePeriod,
				expectedSpec.Containers,
				expectedSpec.Toleration,
				expectedSpec.Affinity,
			)

			assert.EqualValues(t, spec, expectedSpec)
			assert.NoError(t, validations.Validate.Struct(spec))
		})

		t.Run("when create a new spec without ports", func(t *testing.T) {
			containers := []game_room.Container{
				game_room.Container{
					Name:            "default",
					Image:           "some-image",
					ImagePullPolicy: "IfNotPresent",
					Command:         []string{"hello"},
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

			expectedSpec := &game_room.Spec{
				Version:                "v1",
				TerminationGracePeriod: time.Duration(10),
				Containers:             containers,
				Toleration:             "10",
				Affinity:               "10",
			}

			spec := game_room.NewSpec(
				expectedSpec.Version,
				expectedSpec.TerminationGracePeriod,
				expectedSpec.Containers,
				expectedSpec.Toleration,
				expectedSpec.Affinity,
			)

			assert.EqualValues(t, spec, expectedSpec)
			assert.NoError(t, validations.Validate.Struct(spec))
		})
	})

	t.Run("with error", func(t *testing.T) {
		t.Run("when create a new spec with non semantic versioning to Version", func(t *testing.T) {
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

			expectedSpec := &game_room.Spec{
				Version:                "invalid-version",
				TerminationGracePeriod: time.Duration(10),
				Containers:             containers,
				Toleration:             "10",
				Affinity:               "10",
			}

			spec := game_room.NewSpec(
				expectedSpec.Version,
				expectedSpec.TerminationGracePeriod,
				expectedSpec.Containers,
				expectedSpec.Toleration,
				expectedSpec.Affinity,
			)

			assert.EqualValues(t, spec, expectedSpec)
			assert.ErrorContains(t, validations.Validate.Struct(spec), "Error:Field validation for 'Version' failed on the 'semantic_version' tag")
		})

		t.Run("when create a new spec with TerminationGracePeriod lower than 0", func(t *testing.T) {
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

			expectedSpec := &game_room.Spec{
				Version:                "v1",
				TerminationGracePeriod: time.Duration(-10),
				Containers:             containers,
				Toleration:             "10",
				Affinity:               "10",
			}

			spec := game_room.NewSpec(
				expectedSpec.Version,
				expectedSpec.TerminationGracePeriod,
				expectedSpec.Containers,
				expectedSpec.Toleration,
				expectedSpec.Affinity,
			)

			assert.EqualValues(t, spec, expectedSpec)
			assert.ErrorContains(t, validations.Validate.Struct(spec), "Error:Field validation for 'TerminationGracePeriod' failed on the 'gt' tag")
		})
	})
}

func TestSpec_DeepCopy(t *testing.T) {

	tests := []struct {
		name string
		spec *game_room.Spec
		want *game_room.Spec
	}{
		{
			name: "return a deep copy of the spec",
			spec: &game_room.Spec{
				Version:                "v1.1",
				TerminationGracePeriod: 12,
				Containers: []game_room.Container{
					{
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
				},
			},
			want: &game_room.Spec{
				Version:                "v1.1",
				TerminationGracePeriod: 12,
				Containers: []game_room.Container{
					{
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
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spec.DeepCopy()
			require.True(t, &tt.want != &got)
			require.True(t, &tt.want.Containers != &got.Containers)
			require.True(t, &tt.want.Containers[0].Ports[0] != &got.Containers[0].Ports[0])
			require.EqualValues(t, tt.want, got)
		})
	}
}
