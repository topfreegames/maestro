package entities

import "github.com/topfreegames/maestro/internal/core/entities/game_room"

type Scheduler struct {
	ID        string
	Spec      game_room.Spec
	PortRange *PortRange
}
