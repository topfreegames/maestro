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

package ports

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies"
)

// Primary ports (input, driving ports)

// Autoscaler is the interface to the primary port of the autoscaler service.
type Autoscaler interface {
	// CalculateDesiredNumberOfRooms return the number of rooms that a Scheduler should have based on its policy or error if it can calculate.
	CalculateDesiredNumberOfRooms(ctx context.Context, scheduler *entities.Scheduler) (int, error)
	// CanDownscale returns true if the scheduler can downscale, false otherwise.
	CanDownscale(ctx context.Context, scheduler *entities.Scheduler) (bool, error)
	CanDownscaleToNextState(ctx context.Context, scheduler *entities.Scheduler, occupiedToBeDeleted, readyToBeDeleted int) (bool, error)
}

// Secondary ports (output, driven ports)

// Policy is an interface to the port that builds the current scheduler state and calculates the desired number of rooms
// based on it.
type Policy interface {
	// CurrentStateBuilder builds and return the current state of the scheduler required by the policy to calculate the
	// desired number of rooms.
	CurrentStateBuilder(ctx context.Context, scheduler *entities.Scheduler) (policies.CurrentState, error)
	// CalculateDesiredNumberOfRooms calculates the desired number of rooms based on the current state of the scheduler.
	CalculateDesiredNumberOfRooms(policyParameters autoscaling.PolicyParameters, currentState policies.CurrentState) (desiredNumberOfRooms int, err error)
	// CanDownscale returns true if the scheduler can downscale, false otherwise.
	CanDownscale(policyParameters autoscaling.PolicyParameters, currentState policies.CurrentState) (bool, error)
	// NextStateBuilder builds and return the next state of the scheduler considering rooms to be deleted.
	NextStateBuilder(ctx context.Context, scheduler *entities.Scheduler, occupiedToBeDeleted, readyToBeDeleted int) (policies.CurrentState, error)
}
