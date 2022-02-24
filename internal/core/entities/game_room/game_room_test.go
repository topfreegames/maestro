package game_room

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComposeRoomStatus(t *testing.T) {
	// validate that all the combinations have a final state
	possiblePingStatus := []GameRoomPingStatus{
		GameRoomPingStatusUnknown,
		GameRoomPingStatusReady,
		GameRoomPingStatusOccupied,
		GameRoomPingStatusTerminating,
		GameRoomPingStatusTerminated,
	}

	possibleInstanceStatusType := []InstanceStatusType{
		InstanceUnknown,
		InstancePending,
		InstanceReady,
		InstanceTerminating,
		InstanceError,
	}

	// roomComposedStatus
	for _, pingStatus := range possiblePingStatus {
		for _, instanceStatusType := range possibleInstanceStatusType {
			_, err := RoomComposedStatus(pingStatus, instanceStatusType)
			require.NoError(t, err)
		}
	}
}

func TestValidateRoomStatusTransition_SuccessTransitions(t *testing.T) {
	for fromStatus, transitions := range validStatusTransitions {
		for transition := range transitions {
			t.Run(fmt.Sprintf("transition from %s to %s", fromStatus.String(), transition.String()), func(t *testing.T) {
				err := ValidateRoomStatusTransition(fromStatus, transition)
				require.NoError(t, err)
			})
		}
	}
}

func TestValidateRoomStatusTransition_InvalidTransition(t *testing.T) {
	err := ValidateRoomStatusTransition(GameStatusTerminating, GameStatusReady)
	require.Error(t, err)
}
