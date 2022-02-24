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
		gameRoom := &GameRoom{
			ID:          "GR",
			SchedulerID: "S",
			PingStatus:  pingStatus,
		}
		for _, instanceStatusType := range possibleInstanceStatusType {
			_, err := gameRoom.RoomComposedStatus(instanceStatusType)
			require.NoError(t, err)
		}
	}
}

func TestValidateRoomStatusTransition_SuccessTransitions(t *testing.T) {
	for fromStatus, transitions := range validStatusTransitions {
		for transition := range transitions {
			t.Run(fmt.Sprintf("transition from %s to %s", fromStatus.String(), transition.String()), func(t *testing.T) {
				gameRoom := &GameRoom{
					ID:          "GR",
					SchedulerID: "S",
					Status:      fromStatus,
				}
				err := gameRoom.ValidateRoomStatusTransition(transition)
				require.NoError(t, err)
			})
		}
	}
}

func TestValidateRoomStatusTransition_InvalidTransition(t *testing.T) {
	gameRoom := &GameRoom{
		ID:          "GR",
		SchedulerID: "S",
		Status:      GameStatusTerminating,
	}
	err := gameRoom.ValidateRoomStatusTransition(GameStatusReady)
	require.Error(t, err)
}
