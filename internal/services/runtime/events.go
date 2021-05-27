package runtime

type RuntimeEventType int

const (
	RuntimeEventAdded RuntimeEventType = iota
	RuntimeEventUpdated
	RuntimeEventDeleted
)

// RuntimeEvent this struct repesents an event that happened on the run time,
// check RuntimeEventType to see which event is avaiable.
type RuntimeEvent struct {
	Type       RuntimeEventType
	GameRoomID string
	Status     RuntimeGameRoomStatus
}
