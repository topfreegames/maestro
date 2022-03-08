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
	"time"
)

// GameRoomStatus defines which state the room is. This state is composed
// of the Ping status and Runtime status.
type GameRoomStatus int

const (
	// GameStatusPending room is being initialized, and it is not ready yet to
	// receive matches.
	GameStatusPending GameRoomStatus = iota
	// GameStatusUnready room is running, but it is not Ready yet.
	GameStatusUnready
	// GameStatusReady room is running and ready to receive matches.
	GameStatusReady
	// GameStatusOccupied room is running, but not available for allocation.
	GameStatusOccupied
	// GameStatusTerminating room is running, but it is in the termination
	// process.
	GameStatusTerminating
	// GameStatusError room has errors (e.g. CrashLoopBackoff in kubernetes)
	GameStatusError
)

func (status GameRoomStatus) String() string {
	switch status {
	case GameStatusPending:
		return "pending"
	case GameStatusUnready:
		return "unready"
	case GameStatusReady:
		return "ready"
	case GameStatusOccupied:
		return "occupied"
	case GameStatusTerminating:
		return "terminating"
	case GameStatusError:
		return "error"
	default:
		panic(fmt.Sprintf("invalid value for GameRoomStatus: %d", int(status)))
	}
}

// GameRoomPingStatus room status informed through ping.
type GameRoomPingStatus int

const (
	// GameRoomPingStatusUnknown room hasn't sent a ping yet.
	GameRoomPingStatusUnknown GameRoomPingStatus = iota
	// GameRoomPingStatusReady room is empty and ready to receive matches
	GameRoomPingStatusReady
	// GameRoomPingStatusOccupied room has matches running inside of it.
	GameRoomPingStatusOccupied
	// GameRoomPingStatusTerminating room is in the process of terminating itself.
	GameRoomPingStatusTerminating
	// GameRoomPingStatusTerminated room has terminated.
	GameRoomPingStatusTerminated
)

func (status GameRoomPingStatus) String() string {
	switch status {
	case GameRoomPingStatusUnknown:
		return "unknown"
	case GameRoomPingStatusReady:
		return "ready"
	case GameRoomPingStatusOccupied:
		return "occupied"
	case GameRoomPingStatusTerminating:
		return "terminating"
	case GameRoomPingStatusTerminated:
		return "terminated"
	default:
		panic(fmt.Sprintf("invalid value for GameRoomPingStatus: %d", int(status)))
	}
}

func FromStringToGameRoomPingStatus(value string) (GameRoomPingStatus, error) {
	switch value {
	case "ready":
		return GameRoomPingStatusReady, nil
	case "occupied":
		return GameRoomPingStatusOccupied, nil
	case "terminating":
		return GameRoomPingStatusTerminating, nil
	case "terminated":
		return GameRoomPingStatusTerminated, nil
	default:
	}

	return GameRoomPingStatusUnknown, fmt.Errorf("cannot convert string %s to a valid GameRoomStatus representation", value)
}

type GameRoom struct {
	ID               string
	SchedulerID      string
	Version          string
	Status           GameRoomStatus
	PingStatus       GameRoomPingStatus
	Metadata         map[string]interface{}
	LastPingAt       time.Time
	IsValidationRoom bool
}

func (g *GameRoom) SetRoomValidation() {
	g.IsValidationRoom = true
}

// validStatusTransitions this map has all possible status changes for a game
// room.
var validStatusTransitions = map[GameRoomStatus]map[GameRoomStatus]struct{}{
	GameStatusPending: {
		GameStatusReady:       struct{}{},
		GameStatusTerminating: struct{}{},
		GameStatusUnready:     struct{}{},
		GameStatusError:       struct{}{},
	},
	GameStatusReady: {
		GameStatusOccupied:    struct{}{},
		GameStatusTerminating: struct{}{},
		GameStatusUnready:     struct{}{},
		GameStatusError:       struct{}{},
	},
	GameStatusUnready: {
		GameStatusTerminating: struct{}{},
		GameStatusReady:       struct{}{},
		GameStatusError:       struct{}{},
		GameStatusPending:     struct{}{},
	},
	GameStatusOccupied: {
		GameStatusReady:       struct{}{},
		GameStatusTerminating: struct{}{},
		GameStatusUnready:     struct{}{},
		GameStatusError:       struct{}{},
	},
	GameStatusError: {
		GameStatusTerminating: struct{}{},
		GameStatusUnready:     struct{}{},
		GameStatusReady:       struct{}{},
	},
	GameStatusTerminating: {},
}

// roomStatusComposition define what is the "final" game room status based on
// the provided ping and instance status.
var roomStatusComposition = []struct {
	pingStatus         GameRoomPingStatus
	instanceStatusType InstanceStatusType
	status             GameRoomStatus
}{
	// Pending
	{GameRoomPingStatusUnknown, InstanceUnknown, GameStatusPending},
	{GameRoomPingStatusUnknown, InstancePending, GameStatusPending},
	{GameRoomPingStatusReady, InstancePending, GameStatusPending},
	{GameRoomPingStatusReady, InstanceUnknown, GameStatusPending},
	{GameRoomPingStatusOccupied, InstanceUnknown, GameStatusPending},
	{GameRoomPingStatusOccupied, InstancePending, GameStatusPending},

	// Ready
	{GameRoomPingStatusReady, InstanceReady, GameStatusReady},

	// Occupied
	{GameRoomPingStatusOccupied, InstanceReady, GameStatusOccupied},

	// Unready
	{GameRoomPingStatusUnknown, InstanceReady, GameStatusUnready},
	{GameRoomPingStatusUnknown, InstancePending, GameStatusUnready},

	// Terminating
	{GameRoomPingStatusUnknown, InstanceTerminating, GameStatusTerminating},
	{GameRoomPingStatusReady, InstanceTerminating, GameStatusTerminating},
	{GameRoomPingStatusOccupied, InstanceTerminating, GameStatusTerminating},
	{GameRoomPingStatusTerminating, InstancePending, GameStatusTerminating},
	{GameRoomPingStatusTerminating, InstanceReady, GameStatusTerminating},
	{GameRoomPingStatusTerminating, InstanceTerminating, GameStatusTerminating},
	{GameRoomPingStatusTerminating, InstanceUnknown, GameStatusTerminating},
	{GameRoomPingStatusTerminated, InstancePending, GameStatusTerminating},
	{GameRoomPingStatusTerminated, InstanceReady, GameStatusTerminating},
	{GameRoomPingStatusTerminated, InstanceTerminating, GameStatusTerminating},
	{GameRoomPingStatusTerminated, InstanceUnknown, GameStatusTerminating},

	// Error
	{GameRoomPingStatusUnknown, InstanceError, GameStatusError},
	{GameRoomPingStatusReady, InstanceError, GameStatusError},
	{GameRoomPingStatusOccupied, InstanceError, GameStatusError},
	{GameRoomPingStatusTerminating, InstanceError, GameStatusError},
	{GameRoomPingStatusTerminated, InstanceError, GameStatusError},
}

// RoomComposedStatus returns a game room status formed by a game room ping status and an instance status
func (g *GameRoom) RoomComposedStatus(instanceStatusType InstanceStatusType) (GameRoomStatus, error) {
	for _, composition := range roomStatusComposition {
		if composition.pingStatus == g.PingStatus && composition.instanceStatusType == instanceStatusType {
			return composition.status, nil
		}
	}

	return GameStatusPending, fmt.Errorf(
		"ping status \"%s\" and instance status \"%s\" doesn't have a match",
		g.PingStatus.String(), instanceStatusType.String(),
	)
}

// ValidateRoomStatusTransition validates that a transition from currentStatus to newStatus can happen.
func (g *GameRoom) ValidateRoomStatusTransition(newStatus GameRoomStatus) error {
	transitions, ok := validStatusTransitions[g.Status]
	if !ok {
		return fmt.Errorf("game rooms has an invalid status %s", g.Status.String())
	}

	if _, valid := transitions[newStatus]; !valid {
		return fmt.Errorf("cannot change game room status from %s to %s", g.Status.String(), newStatus.String())
	}

	return nil
}
