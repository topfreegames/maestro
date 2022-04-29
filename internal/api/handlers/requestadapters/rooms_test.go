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

package requestadapters_test

import (
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/api/handlers/requestadapters"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func TestFromApiUpdateRoomRequestToEntity(t *testing.T) {
	type Input struct {
		PingRequest *api.UpdateRoomWithPingRequest
	}

	type Output struct {
		GameRoom   *game_room.GameRoom
		GivesError bool
	}

	genericString := "some-value"
	genericTime := time.Now()

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "receives valid ping request, returns gameRoom entity",
			Input: Input{
				PingRequest: &api.UpdateRoomWithPingRequest{
					SchedulerName: genericString,
					RoomName:      genericString,
					Metadata:      nil,
					Status:        "ready",
					Timestamp:     int64(genericTime.Second()),
				},
			},
			Output: Output{
				GameRoom: &game_room.GameRoom{
					ID:               genericString,
					SchedulerID:      genericString,
					Version:          "",
					Status:           0,
					PingStatus:       game_room.GameRoomPingStatusReady,
					Metadata:         map[string]interface{}{},
					LastPingAt:       time.Unix(int64(genericTime.Second()), 0),
					IsValidationRoom: false,
				},
			},
		},
		{
			Title: "receives invalid ping request, returns err and gameRoom nil",
			Input: Input{
				PingRequest: &api.UpdateRoomWithPingRequest{
					SchedulerName: genericString,
					RoomName:      genericString,
					Metadata:      nil,
					Status:        "INVALID_PING",
					Timestamp:     int64(genericTime.Second()),
				},
			},
			Output: Output{
				GameRoom:   nil,
				GivesError: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			returnValues, err := requestadapters.FromApiUpdateRoomRequestToEntity(testCase.Input.PingRequest)
			if testCase.Output.GivesError {
				assert.Error(t, err)
			}
			assert.EqualValues(t, testCase.Output.GameRoom, returnValues)
		})
	}
}
