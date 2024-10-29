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

package rooms

import (
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"go.uber.org/zap"
)

type StatusCalculator interface {
	CalculateRoomStatus(room game_room.GameRoom, instance game_room.Instance) (game_room.GameRoomStatus, error)
}

func NewStatusCalculator(scheduler entities.Scheduler, logger *zap.Logger) StatusCalculator {
	if scheduler.MatchAllocation.MaxMatches > 1 {
		return &MultipleMatchStatusCalculator{
			scheduler: scheduler,
			logger:    logger,
		}
	}

	return &SingleMatchStatusCalculator{
		scheduler: scheduler,
		logger:    logger,
	}
}

type SingleMatchStatusCalculator struct {
	scheduler entities.Scheduler
	logger    *zap.Logger
}

func (sm *SingleMatchStatusCalculator) CalculateRoomStatus(room game_room.GameRoom, instance game_room.Instance) (game_room.GameRoomStatus, error) {
	currentStatus, err := room.RoomComposedStatus(instance.Status.Type)
	if err != nil {
		return currentStatus, fmt.Errorf("error calculating room status: %w", err)
	}

	if currentStatus == game_room.GameStatusReady && room.RunningMatches > 0 {
		sm.logger.Warn("room is ready but has running matches", zap.String("room", room.ID))
	}

	if currentStatus == game_room.GameStatusOccupied && room.RunningMatches == 0 {
		sm.logger.Warn("room is occupied but has no running matches", zap.String("room", room.ID))
	}

	return currentStatus, nil
}

type MultipleMatchStatusCalculator struct {
	scheduler entities.Scheduler
	logger    *zap.Logger
}

func (mm *MultipleMatchStatusCalculator) CalculateRoomStatus(room game_room.GameRoom, instance game_room.Instance) (game_room.GameRoomStatus, error) {
	composedStatus, err := room.RoomComposedStatus(instance.Status.Type)
	if err != nil {
		return composedStatus, fmt.Errorf("error calculating room status: %w", err)
	}

	// For now, only when the room is ready or occupied the calculation is different
	if composedStatus != game_room.GameStatusReady && composedStatus != game_room.GameStatusOccupied {
		return composedStatus, nil
	}

	if room.RunningMatches == 0 {
		return game_room.GameStatusReady, nil
	}

	if room.RunningMatches == mm.scheduler.MatchAllocation.MaxMatches {
		return game_room.GameStatusOccupied, nil
	}

	if room.Status == game_room.GameStatusOccupied && room.RunningMatches < mm.scheduler.MatchAllocation.MaxMatches {
		return game_room.GameStatusCooling, nil
	}

	if room.Status == game_room.GameStatusCooling && (mm.scheduler.MatchAllocation.MaxMatches-room.RunningMatches) <= mm.scheduler.MatchAllocation.MinFreeSlots {
		return game_room.GameStatusCooling, nil
	}

	return game_room.GameStatusActive, nil
}
