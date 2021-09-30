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
	// GameStatusPingUnknown room hasn't sent a ping yet.
	GameRoomPingStatusUnknown GameRoomPingStatus = iota
	// GameStatusReady room is empty and ready to receive matches
	GameRoomPingStatusReady
	// GameStatusOccupied room has matches running inside of it.
	GameRoomPingStatusOccupied
	// GameStatusTerminating room is in the process of terminating itself.
	GameRoomPingStatusTerminating
	// GameStatusTerminating room has terminated.
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
	ID          string
	SchedulerID string
	Status      GameRoomStatus
	PingStatus  GameRoomPingStatus
	Metadata    map[string]interface{}
	LastPingAt  time.Time
}
