package autoscaling

import "github.com/topfreegames/maestro/internal/validations"

// Autoscaling represents the autoscaling configuration for a scheduler.
type Autoscaling struct {
	// Enabled indicates if autoscaling is enabled.
	Enabled bool
	// Min indicates the minimum number of replicas,
	// it must be greater than 1 and lower than max.
	Min int `validate:"min=1,custom_lower_than=Max"`
	// Max indicates the maximum number of replicas,
	// it must be greater than or equal zero, or -1 to have no limit.
	Max int `validate:"min=-1"`
	// Policy indicates the autoscaling policy configuration.
	Policy Policy
}

func (a *Autoscaling) Validate() error {
	return validations.Validate.Struct(a)
}

func NewAutoscaling(enabled bool, min, max int, policy Policy) (*Autoscaling, error) {
	autoscaling := &Autoscaling{
		Enabled: enabled,
		Min:     min,
		Max:     max,
		Policy:  policy,
	}
	return autoscaling, autoscaling.Validate()
}

// Policy represents the autoscaling policy configuration.
type Policy struct {
	// Type indicates the autoscaling policy type.
	Type PolicyType `validate:"oneof=roomOccupancy"`
	// Parameters indicates the autoscaling policy parameters.
	Parameters PolicyParameters
}

// PolicyParameters represents the autoscaling policy parameters, its fields validations will
// vary according to the policy type.
type PolicyParameters struct {
	// RoomOccupancy represents the parameters for RoomOccupancy policy type, it must be provided if Policy Type is RoomOccupancy.
	// +optional
	RoomOccupancy *RoomOccupancyParams `validate:"required_for_room_occupancy=Type"`
}

type RoomOccupancyParams struct {
	// ReadyTarget indicates the target percentage of ready rooms a scheduler should maintain.
	ReadyTarget float64 `validate:"gt=0,lt=1"`
}

// PolicyType represents an enum of possible policy types/strategies a scheduler can have.
type PolicyType string

const (
	RoomOccupancy PolicyType = "roomOccupancy"
)
