package entities

import "time"

type GameRoom struct {
	ID         string
	Scheduler  string
	Status     GameRoomStatus
	Metadata   map[string]interface{}
	LastPingAt time.Time
}
