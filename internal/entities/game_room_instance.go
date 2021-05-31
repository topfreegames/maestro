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
	Type GameRoomInstanceStatusType `json:"type"`
	// Description has more information about the status. For example, if we have a
	// status Error, it will tell us which error it is.
	Description string `json:"description"`
}

type GameRoomPort struct {
	Name     string `json:"name"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
}

type GameRoomAddress struct {
	Host  string         `json:"host"`
	Ports []GameRoomPort `json:"ports"`
}

type GameRoomInstance struct {
	ID          string                 `json:"id"`
	SchedulerID string                 `json:"schedulerId"`
	Version     string                 `json:"version"`
	Status      GameRoomInstanceStatus `json:"status"`
	Address     *GameRoomAddress       `json:"address"`
}
