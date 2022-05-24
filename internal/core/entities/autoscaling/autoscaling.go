package autoscaling

// Autoscaling represents the autoscaling configuration for a scheduler.
type Autoscaling struct {
	// Enabled indicates if autoscaling is enabled.
	Enabled bool
	// Min indicates the minimum number of replicas,
	// it must be greater than 0 and lower than max.
	Min int
	// Max indicates the maximum number of replicas,
	// it must be greater than or equal zero, or -1 to have no limit.
	Max int
	// Policy indicates the autoscaling policy configuration.
	Policy Policy
}

// Policy represents the autoscaling policy configuration.
type Policy struct {
	// Type indicates the autoscaling policy type.
	Type PolicyType
	// Parameters indicates the autoscaling policy parameters.
	Parameters PolicyParameters
}

// PolicyParameters represents the autoscaling policy parameters, its fields validations will
// vary according to the policy type.
type PolicyParameters struct {
	// RoomOccupancy represents the parameters for RoomOccupancy policy type, it must be provided if Policy Type is RoomOccupancy.
	// +optional
	RoomOccupancy *RoomOccupancyParams
}

type RoomOccupancyParams struct {
	// ReadyTarget indicates the target percentage of ready rooms a scheduler should maintain.
	ReadyTarget string
}

// PolicyType represents an enum of possible policy types/strategies a scheduler can have.
type PolicyType string

const (
	RoomOccupancy PolicyType = "roomOccupancy"
)
