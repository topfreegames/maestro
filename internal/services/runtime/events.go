package runtime

import (
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

type RuntimeGameInstanceEventType int

const (
	// RuntimeGameInstanceEventTypeAdded will happen when the game instance is
	// added on the runtime. Happen only once per instance.
	RuntimeGameInstanceEventTypeAdded RuntimeGameInstanceEventType = iota
	// RuntimeGameInstanceEventTypeUpdated will happen when the instance has any
	// change on the runtime, for example, if its status change. Can happen
	// multiple times for an instance.
	RuntimeGameInstanceEventTypeUpdated
	// RuntimeGameInstanceEventTypeDelete will happen when the instance is
	// deleted from the runtime. Can happen only once per instance.
	RuntimeGameInstanceEventTypeDeleted
)

// RuntimeGameInstanceEvent this struct repesents an event that happened on the
// run time, check RuntimeGameInstanceEventType to see which event is avaiable.
type RuntimeGameInstanceEvent struct {
	Type     RuntimeGameInstanceEventType
	Instance game_room.Instance
}
