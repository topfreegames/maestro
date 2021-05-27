package entities

// GameRoomInstanceStatusType represents the status that a game room instance has.
type GameRoomInstanceStatusType int

const (
	// GameRoomInstance has an undefined status.
	GameRoomInstanceUnknown GameRoomInstanceStatusType = iota
	// GameRoomInstance is not ready yet. It will be on this state while creating.
	GameRoomInstancePending
	// GameRoomInstance is running fine and all probes are being successful.
	GameRoomInstanceReady
	// GameRoomInstance has received a termination signal and it is on process of
	// shutdown.
	GameRoomInstanceTerminating
	// GameRoomInstance has some sort of error.
	GameRoomInstanceError
)

type GameRoomInstanceStatus struct {
	Type GameRoomInstanceStatusType
	// Description has more information about the status. For example, if we have a
	// status Error, it will tell us which error it is.
	Description string
}

type GameRoomPort struct {
	Name     string
	Port     int32
	Protocol string
}

type GameRoomAddress struct {
	Host  string
	Ports []GameRoomPort
}

type GameRoomInstance struct {
	ID          string
	SchedulerID string
	Version     string
	Status      GameRoomInstanceStatus
	Address     *GameRoomAddress
}
