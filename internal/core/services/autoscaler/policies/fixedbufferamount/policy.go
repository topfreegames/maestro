package fixedbufferamount

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

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies"
)

const (
	// TotalRoomsKey is the key to total rooms in the CurrentState map.
	TotalRoomsKey = "TotalRooms"
	// OccupiedRoomsKey is the key to occupied rooms in the CurrentState map.
	OccupiedRoomsKey = "OccupiedRooms"
)

type Policy struct {
	roomStorage ports.RoomStorage
}

var _ ports.Policy = new(Policy)

// NewPolicy create a new room occupancy autoscaling policy.
func NewPolicy(roomStorage ports.RoomStorage) *Policy {
	return &Policy{
		roomStorage: roomStorage,
	}
}

// CurrentStateBuilder fill the fields that should be considered during the autoscaling policy.
func (p *Policy) CurrentStateBuilder(ctx context.Context, scheduler *entities.Scheduler) (policies.CurrentState, error) {
	occupiedCount, err := p.roomStorage.GetRoomCountByStatus(ctx, scheduler.Name, game_room.GameStatusOccupied)
	if err != nil {
		return nil, fmt.Errorf("error fetching occupied game rooms amount: %w", err)
	}

	terminatingCount, err := p.roomStorage.GetRoomCountByStatus(ctx, scheduler.Name, game_room.GameStatusTerminating)
	if err != nil {
		return nil, fmt.Errorf("error fetching terminating game rooms amount: %w", err)
	}

	errorCount, err := p.roomStorage.GetRoomCountByStatus(ctx, scheduler.Name, game_room.GameStatusError)
	if err != nil {
		return nil, fmt.Errorf("error fetching error game rooms amount: %w", err)
	}

	totalCount, err := p.roomStorage.GetRoomCount(ctx, scheduler.Name)
	if err != nil {
		return nil, fmt.Errorf("error fetching total game rooms amount: %w", err)
	}

	totalCount = totalCount - terminatingCount - errorCount

	return policies.CurrentState{
		OccupiedRoomsKey: occupiedCount,
		TotalRoomsKey:    totalCount,
	}, nil
}

// CalculateDesiredNumberOfRooms returns the desired number of rooms: occupied rooms + fixed amount (Amount).
func (p *Policy) CalculateDesiredNumberOfRooms(policyParameters autoscaling.PolicyParameters, currentState policies.CurrentState) (int, error) {
	currentOccupiedRooms, ok := currentState[OccupiedRoomsKey].(int)
	if !ok {
		return -1, fmt.Errorf("could not get occupied rooms from the currentState: %v", currentState)
	}

	fixedAmount := policyParameters.FixedBuffer.Amount
	// Desired = occupied + fixed amount
	desiredNumberOfRooms := currentOccupiedRooms + int(fixedAmount)

	return desiredNumberOfRooms, nil
}

// CanDownscale returns true if the current number of rooms is above the desired number (occupied + fixed amount).
func (p *Policy) CanDownscale(policyParameters autoscaling.PolicyParameters, currentState policies.CurrentState) (bool, error) {
	desiredNumberOfRooms, err := p.CalculateDesiredNumberOfRooms(policyParameters, currentState)
	if err != nil {
		return false, fmt.Errorf("error calculating the desired number of rooms: %w", err)
	}

	currentTotalRooms, ok := currentState[TotalRoomsKey].(int)
	if !ok {
		return false, fmt.Errorf("could not get total rooms from the currentState: %v", currentState)
	}

	canDownscale := currentTotalRooms > desiredNumberOfRooms

	return canDownscale, nil
}
