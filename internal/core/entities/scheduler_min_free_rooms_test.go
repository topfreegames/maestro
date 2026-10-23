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
