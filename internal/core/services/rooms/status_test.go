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

//go:build unit
// +build unit

package rooms

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/allocation"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"go.uber.org/zap"
)

func TestMultipleMatchStatusCalculator_CalculateRoomStatus(t *testing.T) {
	logger := zap.NewNop()
	scheduler := entities.Scheduler{
		MatchAllocation: allocation.MatchAllocation{
			MaxMatches:   3,
			MinFreeSlots: 1,
		},
	}

	calculator := &MultipleMatchStatusCalculator{
		scheduler: scheduler,
		logger:    logger,
	}

	tests := []struct {
		name           string
		room           game_room.GameRoom
		instance       game_room.Instance
		expectedStatus game_room.GameRoomStatus
	}{
		{
			name: "room ready with no running matches",
			room: game_room.GameRoom{
				Status:         game_room.GameStatusReady,
				PingStatus:     game_room.GameRoomPingStatusReady,
				RunningMatches: 0,
			},
			instance:       game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceReady}},
			expectedStatus: game_room.GameStatusReady,
		},
		{
			name: "room occupied with max running matches",
			room: game_room.GameRoom{
				Status:         game_room.GameStatusOccupied,
				PingStatus:     game_room.GameRoomPingStatusOccupied,
				RunningMatches: 3,
			},
			instance:       game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceReady}},
			expectedStatus: game_room.GameStatusOccupied,
		},
		{
			name: "room pending with instance pending",
			room: game_room.GameRoom{
				Status:         game_room.GameStatusPending,
				RunningMatches: 0,
			},
			instance:       game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstancePending}},
			expectedStatus: game_room.GameStatusPending,
		},
		{
			name: "room terminating with instance terminating",
			room: game_room.GameRoom{
				Status:         game_room.GameStatusTerminating,
				RunningMatches: 0,
			},
			instance:       game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceTerminating}},
			expectedStatus: game_room.GameStatusTerminating,
		},
		{
			name: "room error with instance error",
			room: game_room.GameRoom{
				Status:         game_room.GameStatusError,
				RunningMatches: 0,
			},
			instance:       game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceError}},
			expectedStatus: game_room.GameStatusError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := calculator.CalculateRoomStatus(tt.room, tt.instance)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}
