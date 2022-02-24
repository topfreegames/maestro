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

package room_manager

import (
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

// validStatusTransitions this map has all possible status changes for a game
// room.
var validStatusTransitions = map[game_room.GameRoomStatus]map[game_room.GameRoomStatus]struct{}{
	game_room.GameStatusPending: {
		game_room.GameStatusReady:       struct{}{},
		game_room.GameStatusTerminating: struct{}{},
		game_room.GameStatusUnready:     struct{}{},
		game_room.GameStatusError:       struct{}{},
	},
	game_room.GameStatusReady: {
		game_room.GameStatusOccupied:    struct{}{},
		game_room.GameStatusTerminating: struct{}{},
		game_room.GameStatusUnready:     struct{}{},
		game_room.GameStatusError:       struct{}{},
	},
	game_room.GameStatusUnready: {
		game_room.GameStatusTerminating: struct{}{},
		game_room.GameStatusReady:       struct{}{},
		game_room.GameStatusError:       struct{}{},
	},
	game_room.GameStatusOccupied: {
		game_room.GameStatusReady:       struct{}{},
		game_room.GameStatusTerminating: struct{}{},
		game_room.GameStatusUnready:     struct{}{},
		game_room.GameStatusError:       struct{}{},
	},
	game_room.GameStatusError: {
		game_room.GameStatusTerminating: struct{}{},
		game_room.GameStatusUnready:     struct{}{},
		game_room.GameStatusReady:       struct{}{},
	},
	game_room.GameStatusTerminating: {},
}

// roomStatusComposition define what is the "final" game room status based on
// the provided ping and instance status.
var roomStatusComposition = []struct {
	pingStatus         game_room.GameRoomPingStatus
	instanceStatusType game_room.InstanceStatusType
	status             game_room.GameRoomStatus
}{
	// Pending
	{game_room.GameRoomPingStatusUnknown, game_room.InstanceUnknown, game_room.GameStatusPending},
	{game_room.GameRoomPingStatusUnknown, game_room.InstancePending, game_room.GameStatusPending},
	{game_room.GameRoomPingStatusReady, game_room.InstancePending, game_room.GameStatusPending},
	{game_room.GameRoomPingStatusReady, game_room.InstanceUnknown, game_room.GameStatusPending},
	{game_room.GameRoomPingStatusOccupied, game_room.InstanceUnknown, game_room.GameStatusPending},
	{game_room.GameRoomPingStatusOccupied, game_room.InstancePending, game_room.GameStatusPending},

	// Ready
	{game_room.GameRoomPingStatusReady, game_room.InstanceReady, game_room.GameStatusReady},

	// Occupied
	{game_room.GameRoomPingStatusOccupied, game_room.InstanceReady, game_room.GameStatusOccupied},

	// Unready
	{game_room.GameRoomPingStatusUnknown, game_room.InstanceReady, game_room.GameStatusUnready},

	// Terminating
	{game_room.GameRoomPingStatusUnknown, game_room.InstanceTerminating, game_room.GameStatusTerminating},
	{game_room.GameRoomPingStatusReady, game_room.InstanceTerminating, game_room.GameStatusTerminating},
	{game_room.GameRoomPingStatusOccupied, game_room.InstanceTerminating, game_room.GameStatusTerminating},
	{game_room.GameRoomPingStatusTerminating, game_room.InstancePending, game_room.GameStatusTerminating},
	{game_room.GameRoomPingStatusTerminating, game_room.InstanceReady, game_room.GameStatusTerminating},
	{game_room.GameRoomPingStatusTerminating, game_room.InstanceTerminating, game_room.GameStatusTerminating},
	{game_room.GameRoomPingStatusTerminating, game_room.InstanceUnknown, game_room.GameStatusTerminating},
	{game_room.GameRoomPingStatusTerminated, game_room.InstancePending, game_room.GameStatusTerminating},
	{game_room.GameRoomPingStatusTerminated, game_room.InstanceReady, game_room.GameStatusTerminating},
	{game_room.GameRoomPingStatusTerminated, game_room.InstanceTerminating, game_room.GameStatusTerminating},
	{game_room.GameRoomPingStatusTerminated, game_room.InstanceUnknown, game_room.GameStatusTerminating},

	// Error
	{game_room.GameRoomPingStatusUnknown, game_room.InstanceError, game_room.GameStatusError},
	{game_room.GameRoomPingStatusReady, game_room.InstanceError, game_room.GameStatusError},
	{game_room.GameRoomPingStatusOccupied, game_room.InstanceError, game_room.GameStatusError},
	{game_room.GameRoomPingStatusTerminating, game_room.InstanceError, game_room.GameStatusError},
	{game_room.GameRoomPingStatusTerminated, game_room.InstanceError, game_room.GameStatusError},
}

func roomComposedStatus(pingStatus game_room.GameRoomPingStatus, instanceStatusType game_room.InstanceStatusType) (game_room.GameRoomStatus, error) {
	for _, composition := range roomStatusComposition {
		if composition.pingStatus == pingStatus && composition.instanceStatusType == instanceStatusType {
			return composition.status, nil
		}
	}

	return game_room.GameStatusPending, fmt.Errorf(
		"ping status \"%s\" and instance status \"%s\" doesn't have a match",
		pingStatus.String(), instanceStatusType.String(),
	)
}

// validateRoomStatusTransition validates that a transition from currentStatus to newStatus can happen.
func validateRoomStatusTransition(currentStatus game_room.GameRoomStatus, newStatus game_room.GameRoomStatus) error {
	transitions, ok := validStatusTransitions[currentStatus]
	if !ok {
		return fmt.Errorf("game rooms has an invalid status %s", currentStatus.String())
	}

	if _, valid := transitions[newStatus]; !valid {
		return fmt.Errorf("cannot change game room status from %s to %s", currentStatus.String(), newStatus.String())
	}

	return nil
}
