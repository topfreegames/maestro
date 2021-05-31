package game_room

// InstanceStatusType represents the status that a game room instance has.
type InstanceStatusType int

const (
	// Instance has an undefined status.
	InstanceUnknown InstanceStatusType = iota
	// Instance is not ready yet. It will be on this state while creating.
	InstancePending
	// Instance is running fine and all probes are being successful.
	InstanceReady
	// Instance has received a termination signal and it is on process of
	// shutdown.
	InstanceTerminating
	// Instance has some sort of error.
	InstanceError
)

type InstanceStatus struct {
	Type InstanceStatusType `json:"type"`
	// Description has more information about the status. For example, if we have a
	// status Error, it will tell us which error it is.
	Description string `json:"description"`
}

type Port struct {
	Name     string `json:"name"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
}

type Address struct {
	Host  string `json:"host"`
	Ports []Port `json:"ports"`
}

type Instance struct {
	ID          string         `json:"id"`
	SchedulerID string         `json:"schedulerId"`
	Version     string         `json:"version"`
	Status      InstanceStatus `json:"status"`
	Address     *Address       `json:"address"`
}
