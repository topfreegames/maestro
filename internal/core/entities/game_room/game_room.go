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

type GameRoomStatus int

const (
	// room instance is still not running
	GameStatusPending GameRoomStatus = iota
	// room instance is running but not sending pings
	GameStatusUnready
	// room instance is running and sending ping with ready status
	GameStatusReady
	// room instance is running and sending ping with occupied status
	GameStatusOccupied
	// room instance is terminating
	GameStatusTerminating
	// room instance has errors (e.g. CrashLoopBackoff in kubernetes)
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

type GameRoom struct {
	ID          string
	SchedulerID string
	Status      GameRoomStatus
	Metadata    map[string]interface{}
	LastPingAt  time.Time
}
