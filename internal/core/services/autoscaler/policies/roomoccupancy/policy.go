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

package roomoccupancy

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies"
)

const (
	// OccupiedRoomsKey is the key to occupied rooms in the CurrentState map.
	OccupiedRoomsKey = "RoomsOccupancyOccupiedRooms"
	// ReadyRoomsKey is the key to total rooms in the CurrentState map.
	ReadyRoomsKey = "RoomsOccupancyTotalRooms"
)

// Policy holds the requirements to build the current state of
// the scheduler that should be considered to calculate the desired number of rooms in the room occupancy policy.
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
	occupiedRoomsAmount, err := p.roomStorage.GetRoomCountByStatus(ctx, scheduler.Name, game_room.GameStatusOccupied)
	if err != nil {
		return nil, fmt.Errorf("error fetching occupied game rooms amount: %w", err)
	}

	readyRoomsAmount, err := p.roomStorage.GetRoomCountByStatus(ctx, scheduler.Name, game_room.GameStatusReady)
	if err != nil {
		return nil, fmt.Errorf("error fetching ready game rooms amount: %w", err)
	}

	currentState := policies.CurrentState{
		OccupiedRoomsKey: occupiedRoomsAmount,
		ReadyRoomsKey:    readyRoomsAmount,
	}

	return currentState, nil
}

// CalculateDesiredNumberOfRooms executes to knows how many rooms should a scheduler have based on your current state.
func (p *Policy) CalculateDesiredNumberOfRooms(policyParameters autoscaling.PolicyParameters, currentState policies.CurrentState) (int, error) {
	if policyParameters.RoomOccupancy == nil {
		return -1, errors.New("RoomOccupancy parameters is empty")
	}

	readyTarget := policyParameters.RoomOccupancy.ReadyTarget
	if readyTarget >= float64(1) || readyTarget <= 0 {
		return -1, errors.New("Ready target must be between 0 and 1")
	}

	if _, ok := currentState[OccupiedRoomsKey].(int); !ok {
		return -1, errors.New("There are no occupiedRooms in the currentState")
	}

	occupiedRooms := currentState[OccupiedRoomsKey].(int)
	desiredNumberOfRoom := int(math.Ceil(float64(occupiedRooms) / (float64(1) - readyTarget)))

	return desiredNumberOfRoom, nil
}

func (p *Policy) CanDownscale(policyParameters autoscaling.PolicyParameters, currentState policies.CurrentState) (bool, error) {
	if policyParameters.RoomOccupancy == nil {
		return false, errors.New("RoomOccupancy parameters is empty")
	}

	downThreshold := policyParameters.RoomOccupancy.DownThreshold
	if downThreshold >= float64(1) || downThreshold <= 0 {
		return false, errors.New("Downscale threshold must be between 0 and 1")
	}

	readyTarget := policyParameters.RoomOccupancy.ReadyTarget
	if readyTarget >= float64(1) || readyTarget <= 0 {
		return false, errors.New("Ready target must be between 0 and 1")
	}

	readyRooms, ok := currentState[ReadyRoomsKey].(int)
	if !ok {
		return false, errors.New("There are no readyRooms in the currentState")
	}

	desiredNumberOfRooms, err := p.CalculateDesiredNumberOfRooms(policyParameters, currentState)
	if err != nil {
		return false, fmt.Errorf("Error calculating the desired number of rooms: %w", err)
	}

	desiredNumberOfReadyRooms := int(math.Ceil(float64(desiredNumberOfRooms) * readyTarget))

	return (float64(desiredNumberOfReadyRooms) / float64(readyRooms)) <= downThreshold, nil
}
