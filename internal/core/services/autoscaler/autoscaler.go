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

package autoscaler

import (
	"context"
	"errors"
	"fmt"

	autoscalerPorts "github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
)

// PolicyMap is a type that corelates a policy type with an autoscaling policy.
type PolicyMap map[autoscaling.PolicyType]autoscalerPorts.Policy

// Autoscaler is a service that holds dependencies to execute autoscaling feature.
type Autoscaler struct {
	policyMap PolicyMap
}

// NewAutoscaler returns a new instance of autoscaler.
func NewAutoscaler(policyMap PolicyMap) *Autoscaler {
	autoscaler := &Autoscaler{
		policyMap: policyMap,
	}

	return autoscaler
}

// CalculateDesiredNumberOfRooms return the number of rooms that a Scheduler should have based on its policy or error if it can calculate.
func (a *Autoscaler) CalculateDesiredNumberOfRooms(ctx context.Context, scheduler *entities.Scheduler) (int, error) {
	if scheduler.Autoscaling == nil {
		return -1, errors.New("scheduler does not have autoscaling struct")
	}

	if _, ok := a.policyMap[scheduler.Autoscaling.Policy.Type]; !ok {
		return -1, fmt.Errorf("error finding policy to scheduler %s", scheduler.Name)
	}

	policy := a.policyMap[scheduler.Autoscaling.Policy.Type]

	currentState, err := policy.CurrentStateBuilder(ctx, scheduler)
	if err != nil {
		return -1, fmt.Errorf("error fetching current state to scheduler %s: %w", scheduler.Name, err)
	}

	desiredNumberOfRooms, err := policy.CalculateDesiredNumberOfRooms(scheduler.Autoscaling.Policy.Parameters, currentState)
	if err != nil {
		return -1, fmt.Errorf("error calculating the desired number of rooms to scheduler %s: %w", scheduler.Name, err)
	}

	desiredNumberOfRooms = ensureDesiredNumberIsBetweenMinAndMax(scheduler.Autoscaling, desiredNumberOfRooms)

	return desiredNumberOfRooms, nil
}

func ensureDesiredNumberIsBetweenMinAndMax(autoscaling *autoscaling.Autoscaling, desiredNumberOfRooms int) int {
	if desiredNumberOfRooms < autoscaling.Min {
		desiredNumberOfRooms = autoscaling.Min
	} else if desiredNumberOfRooms > autoscaling.Max {
		desiredNumberOfRooms = autoscaling.Max
	}

	return desiredNumberOfRooms
}
