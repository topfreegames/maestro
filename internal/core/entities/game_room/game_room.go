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
