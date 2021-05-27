package runtime

// RuntimeGameRoomStatus represents the status that a room has on the Runtime.
type RuntimeGameRoomStatusType int

const (
	// Runtime wasn't able to define which status it is.
	RuntimeGameRoomStatusTypeUnknown RuntimeGameRoomStatusType = iota
	// Runtime room is not ready yet. It will be on this state while creating.
	RuntimeGameRoomStatusTypePending
	// Runtime room is running fine and all probes are being successful.
	RuntimeGameRoomStatusTypeReady
	// Runtime room has received a termination signal and it is on process of
	// shutdown.
	RuntimeGameRoomStatusTypeTerminating
	// RuntimeRoom has some sort of error.
	RuntimeGameRoomStatusTypeError
)

type RuntimeGameRoomStatus struct {
	Type RuntimeGameRoomStatusType
	// Reason has more information about the status. For example, if we have a
	// status Error, it will tell us which error it is.
	Reason string
}
