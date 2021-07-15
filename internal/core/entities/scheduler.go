package entities

import "github.com/topfreegames/maestro/internal/core/entities/game_room"

const (
	//StateCreating represents a cluster state
	StateCreating = "creating"

	//StateTerminating represents a cluster state
	StateTerminating = "terminating"

	//StateInSync represents a cluster state
	StateInSync = "in-sync"

	//StateTerminating represents a cluster state
	StateOnError = "on-error"
)

type Scheduler struct {
	Name            string
	Game            string
	State           string
	RollbackVersion string
	Spec            game_room.Spec
	PortRange       *PortRange
}
