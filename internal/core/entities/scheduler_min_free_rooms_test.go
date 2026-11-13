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

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/allocation"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/port"
	"github.com/topfreegames/maestro/internal/validations"
)

func minFreeRoomsScheduler(maxMatches, minFreeRooms int) *entities.Scheduler {
	return &entities.Scheduler{
		Name:     "scheduler",
		Game:     "game",
		State:    entities.StateInSync,
		MaxSurge: "10%",
		Autoscaling: &autoscaling.Autoscaling{Enabled: true, Min: 1, Max: 10, Policy: autoscaling.Policy{
			Type: autoscaling.RoomOccupancy,
			Parameters: autoscaling.PolicyParameters{RoomOccupancy: &autoscaling.RoomOccupancyParams{
				ReadyTarget: 0.2, DownThreshold: 0.99, MinFreeRooms: minFreeRooms,
			}},
		}},
		Spec: game_room.Spec{
			Version:                "v1.0.0",
			TerminationGracePeriod: 60,
			Containers: []game_room.Container{{
				Name: "game", Image: "image:tag", ImagePullPolicy: "IfNotPresent",
				Ports:    []game_room.ContainerPort{{Name: "tcp", Protocol: "tcp", Port: 80}},
				Requests: game_room.ContainerResources{CPU: "10m", Memory: "100Mi"},
				Limits:   game_room.ContainerResources{CPU: "10m", Memory: "100Mi"},
			}},
		},
		PortRange:       &port.PortRange{Start: 40000, End: 60000},
		MatchAllocation: &allocation.MatchAllocation{MaxMatches: maxMatches},
	}
}

func TestScheduler_MinFreeRoomsValidation(t *testing.T) {
	require.NoError(t, validations.RegisterValidations())
	require.NoError(t, autoscaling.RegisterPolicyValidation(validations.Validate))

	require.NoError(t, minFreeRoomsScheduler(1, 0).Validate(), "default is always valid")
	require.NoError(t, minFreeRoomsScheduler(1, 300).Validate(), "floor with one match per room")
	require.NoError(t, minFreeRoomsScheduler(13, 0).Validate(), "multi-match without a floor")
	require.NoError(t, minFreeRoomsScheduler(13, 3).Validate(), "multi-match with a floor is supported")

	err := minFreeRoomsScheduler(1, -1).Validate()
	require.Error(t, err, "negative floor must be rejected")
}
