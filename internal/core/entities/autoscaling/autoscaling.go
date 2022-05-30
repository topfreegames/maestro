// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package autoscaling

import "github.com/topfreegames/maestro/internal/validations"

// PolicyType represents an enum of possible policy types/strategies a scheduler can have.
type PolicyType string

const (
	// RoomOccupancy is an implemented policy in maestro autoscaler,
	// it uses the number of occupied rooms and a ready rooms target percentage to calculate the desired number of rooms in a scheduler.
	RoomOccupancy PolicyType = "roomOccupancy"
)

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

// Validate check if an Autoscaling struct is well formatted and contains valid values.
func (a *Autoscaling) Validate() error {
	return validations.Validate.Struct(a)
}

// NewAutoscaling instantiates a new autoscaling struct based on its parameters.
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

// RoomOccupancyParams represents the parameters accepted by rooms occupancy autoscaling properties.
type RoomOccupancyParams struct {
	// ReadyTarget indicates the target percentage of ready rooms a scheduler should maintain.
	ReadyTarget float64 `validate:"gt=0,lt=1"`
}

func NewRoomOccupancyPolicy(readyTarget float64) Policy {
	newPolicy := Policy{
		Type: RoomOccupancy,
		Parameters: PolicyParameters{
			RoomOccupancy: &RoomOccupancyParams{
				ReadyTarget: readyTarget,
			},
		},
	}
	return newPolicy
}
