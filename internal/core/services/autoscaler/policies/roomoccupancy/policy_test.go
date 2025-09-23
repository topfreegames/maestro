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
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/allocation"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies/roomoccupancy"
)

func TestCurrentStateBuilder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	runningMatches := 1

	scheduler := &entities.Scheduler{
		Name: "some-name",
		MatchAllocation: &allocation.MatchAllocation{
			MaxMatches: 1,
		},
	}

	t.Run("Success cases - when no error occurs it builds the state with occupied rooms amount", func(t *testing.T) {
		roomStorageMock := mock.NewMockRoomStorage(ctrl)

		roomStorageMock.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusReady).Return(1, nil)
		roomStorageMock.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusActive).Return(1, nil)
		roomStorageMock.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).Return(1, nil)
		roomStorageMock.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusAllocated).Return(1, nil)
		roomStorageMock.EXPECT().GetRunningMatchesCount(gomock.Any(), scheduler.Name).Return(runningMatches, nil)

		policy := roomoccupancy.NewPolicy(roomStorageMock)

		currentState, err := policy.CurrentStateBuilder(context.Background(), scheduler)
		assert.NoError(t, err)

		assert.Equal(t, runningMatches, currentState[roomoccupancy.RunningMatchesKey])
		assert.Equal(t, scheduler.MatchAllocation.MaxMatches, currentState[roomoccupancy.MaxMatchesKey])
	})

	t.Run("Error case - When some error occurs in GetRoomCountByStatus it returns error", func(t *testing.T) {
		roomStorageMock := mock.NewMockRoomStorage(ctrl)

		roomStorageMock.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusReady).Return(-1, errors.New("Error getting amount of ready rooms"))

		policy := roomoccupancy.NewPolicy(roomStorageMock)

		_, err := policy.CurrentStateBuilder(context.Background(), scheduler)
		assert.ErrorContains(t, err, "error fetching ready game rooms amount:")
	})

	t.Run("Error case - When some error occurs in GetRunningMatchesCount it returns error", func(t *testing.T) {
		roomStorageMock := mock.NewMockRoomStorage(ctrl)
		roomStorageMock.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusReady).Return(1, nil)
		roomStorageMock.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusActive).Return(1, nil)
		roomStorageMock.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).Return(1, nil)
		roomStorageMock.EXPECT().GetRunningMatchesCount(gomock.Any(), scheduler.Name).Return(-1, errors.New("Error getting amount of running matches"))

		policy := roomoccupancy.NewPolicy(roomStorageMock)

		_, err := policy.CurrentStateBuilder(context.Background(), scheduler)
		assert.ErrorContains(t, err, "error fetching running matches amount:")
	})
}

func TestCalculateDesiredNumberOfRooms(t *testing.T) {
	policy := &roomoccupancy.Policy{}

	t.Run("Success case - when ready target is smaller than 1 return desired number and nil", func(t *testing.T) {
		t.Parallel()

		t.Run("First case", func(t *testing.T) {
			readyTarget := float64(0.5)
			runningMatches := 160

			schedulerState := policies.CurrentState{
				roomoccupancy.RunningMatchesKey: runningMatches,
				roomoccupancy.MaxMatchesKey:     2,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   readyTarget,
					DownThreshold: 0.1,
				},
			}

			desiredNumberOfRoom, err := policy.CalculateDesiredNumberOfRooms(policyParams, schedulerState)
			assert.NoError(t, err)
			assert.EqualValues(t, desiredNumberOfRoom, 160)
		})

		t.Run("Second case", func(t *testing.T) {
			readyTarget := float64(0.1)
			runningMatches := 5

			schedulerState := policies.CurrentState{
				roomoccupancy.RunningMatchesKey: runningMatches,
				roomoccupancy.MaxMatchesKey:     1,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   readyTarget,
					DownThreshold: 0.1,
				},
			}

			desiredNumberOfRoom, err := policy.CalculateDesiredNumberOfRooms(policyParams, schedulerState)
			assert.NoError(t, err)
			assert.EqualValues(t, desiredNumberOfRoom, 6)
		})

		t.Run("Third case", func(t *testing.T) {
			readyTarget := float64(0.3)
			runningMatches := 0

			schedulerState := policies.CurrentState{
				roomoccupancy.RunningMatchesKey: runningMatches,
				roomoccupancy.MaxMatchesKey:     1,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   readyTarget,
					DownThreshold: 0.1,
				},
			}

			desiredNumberOfRoom, err := policy.CalculateDesiredNumberOfRooms(policyParams, schedulerState)
			assert.NoError(t, err)
			assert.EqualValues(t, desiredNumberOfRoom, 0)
		})

		t.Run("Fourth case", func(t *testing.T) {
			readyTarget := float64(0.3)
			runningMatches := 0

			schedulerState := policies.CurrentState{
				roomoccupancy.RunningMatchesKey: runningMatches,
				roomoccupancy.MaxMatchesKey:     7,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   readyTarget,
					DownThreshold: 0.1,
				},
			}

			desiredNumberOfRoom, err := policy.CalculateDesiredNumberOfRooms(policyParams, schedulerState)
			assert.NoError(t, err)
			assert.EqualValues(t, desiredNumberOfRoom, 0)
		})
	})

	t.Run("Fail case - when there is no RoomOccupancy", func(t *testing.T) {
		schedulerState := policies.CurrentState{}

		policyParams := autoscaling.PolicyParameters{}

		_, err := policy.CalculateDesiredNumberOfRooms(policyParams, schedulerState)
		assert.EqualError(t, err, "roomOccupancy parameters is empty")
	})

	t.Run("Fail case - when there is no RunningMatches", func(t *testing.T) {
		schedulerState := policies.CurrentState{}

		readyTarget := float64(0.3)
		policyParams := autoscaling.PolicyParameters{
			RoomOccupancy: &autoscaling.RoomOccupancyParams{
				ReadyTarget:   readyTarget,
				DownThreshold: 0.1,
			},
		}

		_, err := policy.CalculateDesiredNumberOfRooms(policyParams, schedulerState)
		assert.EqualError(t, err, "there are no runningMatches in the currentState")
	})

	t.Run("Fail case - when ready target is out of 0, 1 range", func(t *testing.T) {
		t.Parallel()

		t.Run("when ready target is 1", func(t *testing.T) {
			readyTarget := float64(1.0)
			runningMatches := 10

			schedulerState := policies.CurrentState{
				roomoccupancy.RunningMatchesKey: runningMatches,
				roomoccupancy.MaxMatchesKey:     1,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget: readyTarget,
				},
			}

			_, err := policy.CalculateDesiredNumberOfRooms(policyParams, schedulerState)
			assert.EqualError(t, err, "ready target must be greater than 0 and less than 1")
		})

		t.Run("when ready target is greater than 1", func(t *testing.T) {
			readyTarget := float64(1.1)
			runningMatches := 10

			schedulerState := policies.CurrentState{
				roomoccupancy.RunningMatchesKey: runningMatches,
				roomoccupancy.MaxMatchesKey:     1,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget: readyTarget,
				},
			}

			_, err := policy.CalculateDesiredNumberOfRooms(policyParams, schedulerState)
			assert.EqualError(t, err, "ready target must be greater than 0 and less than 1")
		})

		t.Run("when ready target is 0", func(t *testing.T) {
			readyTarget := float64(0.0)
			runningMatches := 10

			schedulerState := policies.CurrentState{
				roomoccupancy.RunningMatchesKey: runningMatches,
				roomoccupancy.MaxMatchesKey:     1,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget: readyTarget,
				},
			}

			_, err := policy.CalculateDesiredNumberOfRooms(policyParams, schedulerState)
			assert.EqualError(t, err, "ready target must be greater than 0 and less than 1")
		})

		t.Run("when ready target is lower than 0", func(t *testing.T) {
			readyTarget := float64(-0.1)
			runningMatches := 10

			schedulerState := policies.CurrentState{
				roomoccupancy.RunningMatchesKey: runningMatches,
				roomoccupancy.MaxMatchesKey:     1,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget: readyTarget,
				},
			}

			_, err := policy.CalculateDesiredNumberOfRooms(policyParams, schedulerState)
			assert.EqualError(t, err, "ready target must be greater than 0 and less than 1")
		})
	})
}

func TestCanDownscale(t *testing.T) {
	t.Parallel()

	policy := &roomoccupancy.Policy{}

	t.Run("Success case - when current usage is above threshold", func(t *testing.T) {
		t.Parallel()

		t.Run("it is expected to not allow downscale", func(t *testing.T) {
			readyTarget := float64(0.5)
			downThreshold := float64(0.6)
			runningMatches := 80

			schedulerState := policies.CurrentState{
				roomoccupancy.CurrentFreeSlotsKey: 120,
				roomoccupancy.RunningMatchesKey:   runningMatches,
				roomoccupancy.MaxMatchesKey:       1,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   readyTarget,
					DownThreshold: downThreshold,
				},
			}

			allow, err := policy.CanDownscale(policyParams, schedulerState)
			assert.NoError(t, err)
			assert.Falsef(t, allow, "downscale should not be allowed")
		})

		t.Run("it is expected to not allow downscale - multiple matches", func(t *testing.T) {
			readyTarget := float64(0.5)
			downThreshold := float64(0.6)
			runningMatches := 160

			schedulerState := policies.CurrentState{
				roomoccupancy.CurrentFreeSlotsKey: 250,
				roomoccupancy.RunningMatchesKey:   runningMatches,
				roomoccupancy.MaxMatchesKey:       2,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   readyTarget,
					DownThreshold: downThreshold,
				},
			}

			allow, err := policy.CanDownscale(policyParams, schedulerState)
			assert.NoError(t, err)
			assert.Falsef(t, allow, "downscale should not be allowed")
		})
	})

	t.Run("Success case - when current usage is equal or below threshold", func(t *testing.T) {
		t.Parallel()

		t.Run("it is expected to allow downscale when occupation is equal the threshold", func(t *testing.T) {
			readyTarget := float64(0.5)
			downThreshold := float64(0.7)
			runningMatches := 70

			schedulerState := policies.CurrentState{
				roomoccupancy.CurrentFreeSlotsKey: 100,
				roomoccupancy.RunningMatchesKey:   runningMatches,
				roomoccupancy.MaxMatchesKey:       1,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   readyTarget,
					DownThreshold: downThreshold,
				},
			}

			allow, err := policy.CanDownscale(policyParams, schedulerState)
			assert.NoError(t, err)
			assert.Truef(t, allow, "downscale should be allowed")
		})

		t.Run("it is expected to allow downscale when occupation below the threshold", func(t *testing.T) {
			readyTarget := float64(0.5)
			downThreshold := float64(0.6)
			runningMatches := 60

			schedulerState := policies.CurrentState{
				roomoccupancy.CurrentFreeSlotsKey: 100,
				roomoccupancy.RunningMatchesKey:   runningMatches,
				roomoccupancy.MaxMatchesKey:       1,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   readyTarget,
					DownThreshold: downThreshold,
				},
			}

			allow, err := policy.CanDownscale(policyParams, schedulerState)
			assert.NoError(t, err)
			assert.Truef(t, allow, "downscale should be allowed")
		})

		t.Run("it is expected to allow downscale when occupation below the threshold - multiple matches", func(t *testing.T) {
			readyTarget := float64(0.5)
			downThreshold := float64(0.6)
			runningMatches := 120

			schedulerState := policies.CurrentState{
				roomoccupancy.CurrentFreeSlotsKey: 200,
				roomoccupancy.RunningMatchesKey:   runningMatches,
				roomoccupancy.MaxMatchesKey:       2,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   readyTarget,
					DownThreshold: downThreshold,
				},
			}

			allow, err := policy.CanDownscale(policyParams, schedulerState)
			assert.NoError(t, err)
			assert.Truef(t, allow, "downscale should be allowed")
		})
	})

	t.Run("Fail case - when there is no RoomOccupancy", func(t *testing.T) {
		schedulerState := policies.CurrentState{}

		policyParams := autoscaling.PolicyParameters{}

		_, err := policy.CanDownscale(policyParams, schedulerState)
		assert.EqualError(t, err, "roomOccupancy parameters is empty")
	})

	t.Run("Fail case - when there is no MaxMatches", func(t *testing.T) {
		schedulerState := policies.CurrentState{
			roomoccupancy.RunningMatchesKey: 10,
		}

		readyTarget := float64(0.3)
		downThreshold := float64(0.3)
		policyParams := autoscaling.PolicyParameters{
			RoomOccupancy: &autoscaling.RoomOccupancyParams{
				ReadyTarget:   readyTarget,
				DownThreshold: downThreshold,
			},
		}

		_, err := policy.CanDownscale(policyParams, schedulerState)
		assert.ErrorContains(t, err, "could not get maxMatchesPerRoom from the currentState")
	})

	t.Run("Fail case - when down threshold is out of 0, 1 range", func(t *testing.T) {
		t.Parallel()

		t.Run("when down threshold is 1", func(t *testing.T) {
			downThreshold := float64(1.0)
			runningMatches := 10

			schedulerState := policies.CurrentState{
				roomoccupancy.RunningMatchesKey: runningMatches,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   0.5,
					DownThreshold: downThreshold,
				},
			}

			_, err := policy.CanDownscale(policyParams, schedulerState)
			assert.EqualError(t, err, "downscale threshold must be greater than 0 and less than 1")
		})

		t.Run("when down threshold is greater than 1", func(t *testing.T) {
			downThreshold := float64(1.1)
			runningMatches := 10

			schedulerState := policies.CurrentState{
				roomoccupancy.RunningMatchesKey: runningMatches,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   0.5,
					DownThreshold: downThreshold,
				},
			}

			_, err := policy.CanDownscale(policyParams, schedulerState)
			assert.EqualError(t, err, "downscale threshold must be greater than 0 and less than 1")
		})

		t.Run("when down threshold is 0", func(t *testing.T) {
			downThreshold := float64(0.0)
			runningMatches := 10

			schedulerState := policies.CurrentState{
				roomoccupancy.RunningMatchesKey: runningMatches,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   0.5,
					DownThreshold: downThreshold,
				},
			}

			_, err := policy.CanDownscale(policyParams, schedulerState)
			assert.EqualError(t, err, "downscale threshold must be greater than 0 and less than 1")
		})

		t.Run("when down threshold is lower than 0", func(t *testing.T) {
			downThreshold := float64(-0.1)
			runningMatches := 10

			schedulerState := policies.CurrentState{
				roomoccupancy.RunningMatchesKey: runningMatches,
			}

			policyParams := autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   0.5,
					DownThreshold: downThreshold,
				},
			}

			_, err := policy.CanDownscale(policyParams, schedulerState)
			assert.EqualError(t, err, "downscale threshold must be greater than 0 and less than 1")
		})
	})
}
