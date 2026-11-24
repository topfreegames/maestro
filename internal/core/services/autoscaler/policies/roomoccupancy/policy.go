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
	// RunningMatchesKey is the key to running matches in the CurrentState map.
	RunningMatchesKey = "RoomsOccupancyRunningMatches"
	// MaxMatchesKey is the key to max matches in the CurrentState map.
	MaxMatchesKey = "RoomsOccupancyMaxMatches"
	// CurrentAvailableSlotsKey is the key to Current available slots in the CurrentState map.
	CurrentFreeSlotsKey = "RoomsOccupancyCurrentFreeSlots"
	// RoomsHostingMatchesKey is the key to rooms with at least one running match (active, occupied, allocated).
	RoomsHostingMatchesKey = "RoomsOccupancyRoomsHostingMatches"
	// ReadyRoomsKey is the key to rooms with zero running matches.
	ReadyRoomsKey = "RoomsOccupancyReadyRooms"
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
	readyCount, err := p.roomStorage.GetRoomCountByStatus(ctx, scheduler.Name, game_room.GameStatusReady)
	if err != nil {
		return nil, fmt.Errorf("error fetching ready game rooms amount: %w", err)
	}

	activeCount, err := p.roomStorage.GetRoomCountByStatus(ctx, scheduler.Name, game_room.GameStatusActive)
	if err != nil {
		return nil, fmt.Errorf("error fetching active game rooms amount: %w", err)
	}

	occupiedCount, err := p.roomStorage.GetRoomCountByStatus(ctx, scheduler.Name, game_room.GameStatusOccupied)
	if err != nil {
		return nil, fmt.Errorf("error fetching occupied game rooms amount: %w", err)
	}

	allocatedCount, err := p.roomStorage.GetRoomCountByStatus(ctx, scheduler.Name, game_room.GameStatusAllocated)
	if err != nil {
		return nil, fmt.Errorf("error fetching allocated game rooms amount: %w", err)
	}

	totalFreeSlots := (readyCount + activeCount + occupiedCount + allocatedCount) * scheduler.MatchAllocation.MaxMatches

	runningMatchesAmount, err := p.roomStorage.GetRunningMatchesCount(ctx, scheduler.Name)
	if err != nil {
		return nil, fmt.Errorf("error fetching running matches amount: %w", err)
	}

	// NOTE: Adding allocatedCount to runningMatches assumes 1 match per allocated room.
	// This approach does not work accurately for schedulers with multiple matches per room (maxMatches > 1).
	// The allocate feature currently does not support specifying match count, so we use 1 as a conservative estimate.
	currentState := policies.CurrentState{
		CurrentFreeSlotsKey:    totalFreeSlots - runningMatchesAmount,
		RunningMatchesKey:      runningMatchesAmount + allocatedCount,
		MaxMatchesKey:          scheduler.MatchAllocation.MaxMatches,
		RoomsHostingMatchesKey: activeCount + occupiedCount + allocatedCount,
		ReadyRoomsKey:          readyCount,
	}

	return currentState, nil
}

// CalculateDesiredNumberOfRooms executes to knows how many rooms should a scheduler have based on your current state.
func (p *Policy) CalculateDesiredNumberOfRooms(policyParameters autoscaling.PolicyParameters, currentState policies.CurrentState) (int, error) {
	if err := validateParams(policyParameters.RoomOccupancy); err != nil {
		return -1, err
	}

	desiredNumberOfRooms, _, err := desiredRooms(policyParameters.RoomOccupancy, currentState)
	return desiredNumberOfRooms, err
}

// desiredRooms returns the desired number of rooms and whether the MinFreeRooms floor, rather than
// the ReadyTarget percentage, decided it.
func desiredRooms(params *autoscaling.RoomOccupancyParams, currentState policies.CurrentState) (int, bool, error) {
	runningMatches, ok := currentState[RunningMatchesKey].(int)
	if !ok {
		return -1, false, errors.New("there are no runningMatches in the currentState")
	}

	maxMatchesPerRoom, ok := currentState[MaxMatchesKey].(int)
	if !ok {
		return -1, false, errors.New("could not get maxMatchesPerRoom from the currentState")
	}

	desiredNumberOfMatches := math.Ceil(float64(runningMatches) / (float64(1) - params.ReadyTarget))
	desiredNumberOfRooms := int(math.Ceil(desiredNumberOfMatches / float64(maxMatchesPerRoom)))

	// MinFreeRooms is a floor on rooms with zero running matches: never plan fewer rooms than
	// the ones hosting matches plus the floor. The percentage still wins whenever it asks for
	// more, so the buffer keeps growing with load. Statuses are derived by Maestro from the
	// running match count, so this holds for any maxMatches. Zero disables the floor.
	floorDecided := false
	if params.MinFreeRooms > 0 {
		roomsHostingMatches, ok := currentState[RoomsHostingMatchesKey].(int)
		if !ok {
			return -1, false, errors.New("could not get roomsHostingMatches from the currentState")
		}
		if floorRooms := roomsHostingMatches + params.MinFreeRooms; floorRooms > desiredNumberOfRooms {
			desiredNumberOfRooms = floorRooms
			floorDecided = true
		}
	}

	return desiredNumberOfRooms, floorDecided, nil
}

// CanDownscale determines if downscaling is safe based on current occupancy and policy parameters.
func (p *Policy) CanDownscale(policyParameters autoscaling.PolicyParameters, currentState policies.CurrentState) (bool, error) {
	if err := validateParams(policyParameters.RoomOccupancy); err != nil {
		return false, err
	}

	desiredNumberOfRooms, floorDecided, err := desiredRooms(policyParameters.RoomOccupancy, currentState)
	if err != nil {
		return false, fmt.Errorf("error calculating the desired number of rooms: %w", err)
	}

	maxMatchesPerRoom, ok := currentState[MaxMatchesKey].(int)
	if !ok {
		return false, errors.New("could not get maxMatchesPerRoom from the currentState")
	}

	freeSlots, ok := currentState[CurrentFreeSlotsKey].(int)
	if !ok {
		return false, errors.New("could not get freeSlots from the currentState")
	}

	// Never downscale while the rooms with zero matches are at or below the MinFreeRooms
	// floor. This does not guarantee the floor: the health controller counts unready rooms
	// in the total but the remove operation never selects them, so each one costs an extra
	// ready room, and the remove operation does not recount rooms before removing.
	if minFreeRooms := policyParameters.RoomOccupancy.MinFreeRooms; minFreeRooms > 0 {
		readyRooms, ok := currentState[ReadyRoomsKey].(int)
		if !ok {
			return false, errors.New("could not get readyRooms from the currentState")
		}
		if readyRooms <= minFreeRooms {
			return false, nil
		}
	}

	desiredNumberOfMatches := desiredNumberOfRooms * maxMatchesPerRoom
	desiredFreeSlots := math.Ceil(float64(desiredNumberOfMatches) * policyParameters.RoomOccupancy.ReadyTarget)

	// When the percentage decides, desired * readyTarget is the buffer the fleet has at the
	// desired count, because the desired count was derived from readyTarget. When the floor
	// decides, the desired count is hosting + MinFreeRooms and readyTarget describes nothing
	// about it: the percentage figure understates the real buffer, the ratio below collapses,
	// and DownThreshold is bypassed. Use the buffer the fleet will actually have instead.
	if floorDecided {
		runningMatches, ok := currentState[RunningMatchesKey].(int)
		if !ok {
			return false, errors.New("there are no runningMatches in the currentState")
		}
		desiredFreeSlots = float64(desiredNumberOfMatches - runningMatches)
	}

	return (desiredFreeSlots / float64(freeSlots)) <= policyParameters.RoomOccupancy.DownThreshold, nil
}

func validateParams(params *autoscaling.RoomOccupancyParams) error {
	if params == nil {
		return errors.New("roomOccupancy parameters is empty")
	}

	if params.ReadyTarget >= float64(1) || params.ReadyTarget <= 0 {
		return errors.New("ready target must be greater than 0 and less than 1")
	}

	if params.DownThreshold >= float64(1) || params.DownThreshold <= 0 {
		return errors.New("downscale threshold must be greater than 0 and less than 1")
	}

	if params.MinFreeRooms < 0 {
		return errors.New("min free rooms must be greater than or equal to 0")
	}

	return nil
}
