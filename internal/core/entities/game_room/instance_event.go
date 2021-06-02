package game_room

type InstanceEventType int

const (
	// InstanceEventTypeAdded will happen when the game instance is
	// added on the runtime. Happen only once per instance.
	InstanceEventTypeAdded InstanceEventType = iota
	// InstanceEventTypeUpdated will happen when the instance has any
	// change on the runtime, for example, if its status change. Can happen
	// multiple times for an instance.
	InstanceEventTypeUpdated
	// InstanceEventTypeDelete will happen when the instance is
	// deleted from the runtime. Can happen only once per instance.
	InstanceEventTypeDeleted
)

// InstanceEvent this struct repesents an event that happened on the
// run time, check InstanceEventType to see which event is avaiable.
type InstanceEvent struct {
	Type     InstanceEventType
	Instance *Instance
}
