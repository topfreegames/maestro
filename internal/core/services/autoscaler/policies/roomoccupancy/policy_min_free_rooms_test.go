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

package roomoccupancy_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies/roomoccupancy"
)

func minFreeParams(readyTarget float64, minFreeRooms int) autoscaling.PolicyParameters {
	return autoscaling.PolicyParameters{RoomOccupancy: &autoscaling.RoomOccupancyParams{
		ReadyTarget:   readyTarget,
		DownThreshold: 0.99,
		MinFreeRooms:  minFreeRooms,
	}}
}

type roomsState struct {
	running, hosting, ready, freeSlots, maxMatches int
}

func minFreeState(s roomsState) policies.CurrentState {
	return policies.CurrentState{
		roomoccupancy.RunningMatchesKey:      s.running,
		roomoccupancy.RoomsHostingMatchesKey: s.hosting,
		roomoccupancy.ReadyRoomsKey:          s.ready,
		roomoccupancy.CurrentFreeSlotsKey:    s.freeSlots,
		roomoccupancy.MaxMatchesKey:          s.maxMatches,
	}
}

func TestMinFreeRooms_CalculateDesiredNumberOfRooms(t *testing.T) {
	policy := roomoccupancy.NewPolicy(nil)

	cases := []struct {
		name    string
		state   roomsState
		minFree int
		want    int
	}{
		// Single match per room: hosting == running matches.
		{"zero floor keeps today's result: 344 occupied at 20% -> 430", roomsState{running: 344, hosting: 344, maxMatches: 1}, 0, 430},
		{"floor binds off-peak: 344 + 300 -> 644", roomsState{running: 344, hosting: 344, maxMatches: 1}, 300, 644},
		{"percentage wins at peak: 2000/0.8 = 2500 > 2300", roomsState{running: 2000, hosting: 2000, maxMatches: 1}, 300, 2500},
		{"tie point: 1200 occupied -> both give 1500", roomsState{running: 1200, hosting: 1200, maxMatches: 1}, 300, 1500},
		{"idle: 0 occupied -> floor alone", roomsState{running: 0, hosting: 0, maxMatches: 1}, 300, 300},
		// 13 matches per room: the floor counts rooms with zero matches, whatever the packing.
		{"13/room, zero floor unchanged: ceil(ceil(26/0.8)/13) = 3", roomsState{running: 26, hosting: 2, maxMatches: 13}, 0, 3},
		{"13/room, 26 matches packed in 2 rooms, floor 3 -> 5", roomsState{running: 26, hosting: 2, maxMatches: 13}, 3, 5},
		{"13/room, 26 matches spread over 26 rooms, floor 3 -> 29", roomsState{running: 26, hosting: 26, maxMatches: 13}, 3, 29},
		{"13/room, 300 matches packed in 24 rooms, floor 3 -> percentage wins (29)", roomsState{running: 300, hosting: 24, maxMatches: 13}, 3, 29},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := policy.CalculateDesiredNumberOfRooms(minFreeParams(0.2, c.minFree), minFreeState(c.state))
			require.NoError(t, err)
			require.Equal(t, c.want, got)
		})
	}

	t.Run("negative floor is rejected", func(t *testing.T) {
		_, err := policy.CalculateDesiredNumberOfRooms(minFreeParams(0.2, -1), minFreeState(roomsState{running: 10, hosting: 10, maxMatches: 1}))
		require.Error(t, err)
	})

	t.Run("floor set but state built without the hosting count fails loudly", func(t *testing.T) {
		state := policies.CurrentState{
			roomoccupancy.RunningMatchesKey: 10,
			roomoccupancy.MaxMatchesKey:     1,
		}
		_, err := policy.CalculateDesiredNumberOfRooms(minFreeParams(0.2, 5), state)
		require.Error(t, err)
	})
}

func TestMinFreeRooms_CanDownscale(t *testing.T) {
	policy := roomoccupancy.NewPolicy(nil)

	cases := []struct {
		name    string
		state   roomsState
		minFree int
		want    bool
	}{
		// 344 occupied, floor 300: ready above 300 may shrink, at or below may not.
		{"ready at the floor: refuse", roomsState{running: 344, hosting: 344, ready: 300, freeSlots: 300, maxMatches: 1}, 300, false},
		{"ready below the floor: refuse", roomsState{running: 344, hosting: 344, ready: 250, freeSlots: 250, maxMatches: 1}, 300, false},
		{"ready above the floor: allow", roomsState{running: 344, hosting: 344, ready: 350, freeSlots: 350, maxMatches: 1}, 300, true},
		// Zero floor: exactly today's rule (desired free = ceil(430*0.2) = 86, ratio <= 0.99).
		{"zero floor, plenty free: allow", roomsState{running: 344, hosting: 344, ready: 300, freeSlots: 300, maxMatches: 1}, 0, true},
		{"zero floor, free equals desired free: refuse", roomsState{running: 344, hosting: 344, ready: 86, freeSlots: 86, maxMatches: 1}, 0, false},
		// 13/room: 39 free slots spread across partially filled rooms is not 3 free rooms.
		{"13/room, 39 free slots but 0 ready rooms: refuse", roomsState{running: 300, hosting: 39, ready: 0, freeSlots: 39, maxMatches: 13}, 3, false},
		// desired = max(29, 24+3) = 29 rooms -> 377 slots -> 76 desired free; 116 actual free -> ratio 0.66.
		{"13/room, 8 ready rooms above floor 3 and ratio rule satisfied: allow", roomsState{running: 300, hosting: 24, ready: 8, freeSlots: 116, maxMatches: 13}, 3, true},
		// Same floor satisfied (4 > 3) but the untouched ratio rule refuses: 76 desired free vs 64 actual.
		{"13/room, 4 ready rooms above floor 3 but ratio rule refuses: refuse", roomsState{running: 300, hosting: 24, ready: 4, freeSlots: 64, maxMatches: 13}, 3, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := policy.CanDownscale(minFreeParams(0.2, c.minFree), minFreeState(c.state))
			require.NoError(t, err)
			require.Equal(t, c.want, got)
		})
	}
}

func minFreeParamsWithThreshold(readyTarget, downThreshold float64, minFreeRooms int) autoscaling.PolicyParameters {
	return autoscaling.PolicyParameters{RoomOccupancy: &autoscaling.RoomOccupancyParams{
		ReadyTarget:   readyTarget,
		DownThreshold: downThreshold,
		MinFreeRooms:  minFreeRooms,
	}}
}

// When the floor decides the desired count, DownThreshold must keep working as it does today:
// the gate compares the buffer the fleet would have at the desired count against the current
// buffer. Computing the desired buffer from the percentage instead makes the numerator tiny and
// lets any surplus through, silently disabling the hysteresis band while the floor is active.
func TestMinFreeRooms_CanDownscale_HonoursDownThresholdWhenFloorDecides(t *testing.T) {
	policy := roomoccupancy.NewPolicy(nil)

	cases := []struct {
		name      string
		threshold float64
		state     roomsState
		minFree   int
		want      bool
	}{
		// 240 occupied, 20%, floor 300 -> desired 540, 300 free at the desired count.
		{"floor decides, 5 surplus rooms, threshold 0.7: 300/305 = 0.98 -> hold", 0.7, roomsState{running: 240, hosting: 240, ready: 305, freeSlots: 305, maxMatches: 1}, 300, false},
		{"floor decides, 260 surplus rooms, threshold 0.7: 300/560 = 0.54 -> shrink", 0.7, roomsState{running: 240, hosting: 240, ready: 560, freeSlots: 560, maxMatches: 1}, 300, true},
		{"floor decides, 5 surplus rooms, default threshold 0.99: 0.98 -> shrink", 0.99, roomsState{running: 240, hosting: 240, ready: 305, freeSlots: 305, maxMatches: 1}, 300, true},
		// Percentage decides (1600 occupied -> 2000 > 1900): unchanged rule, desired free = 400.
		{"percentage decides, 50 surplus rooms, threshold 0.7: 400/450 = 0.89 -> hold", 0.7, roomsState{running: 1600, hosting: 1600, ready: 450, freeSlots: 450, maxMatches: 1}, 300, false},
		{"percentage decides, 250 surplus rooms, threshold 0.7: 400/650 = 0.62 -> shrink", 0.7, roomsState{running: 1600, hosting: 1600, ready: 650, freeSlots: 650, maxMatches: 1}, 300, true},
		// 13/room, 26 matches packed in 2 rooms, floor 3 -> desired 5 rooms = 65 slots, 39 free at desired.
		// Current 6 rooms -> 52 free slots. 39/52 = 0.75 > 0.7 -> hold (the percentage figure, 13/52, would shrink).
		{"13/room, floor decides, 1 surplus room, threshold 0.7 -> hold", 0.7, roomsState{running: 26, hosting: 2, ready: 4, freeSlots: 52, maxMatches: 13}, 3, false},
		// Current 8 rooms -> 78 free slots. 39/78 = 0.5 -> shrink.
		{"13/room, floor decides, 3 surplus rooms, threshold 0.7 -> shrink", 0.7, roomsState{running: 26, hosting: 2, ready: 6, freeSlots: 78, maxMatches: 13}, 3, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := policy.CanDownscale(minFreeParamsWithThreshold(0.2, c.threshold, c.minFree), minFreeState(c.state))
			require.NoError(t, err)
			require.Equal(t, c.want, got)
		})
	}
}
