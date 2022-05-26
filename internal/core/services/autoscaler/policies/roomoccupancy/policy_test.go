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
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies/roomoccupancy"
)

func TestCurrentStateBuilder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	readyTarget := float64(0.1)
	occupiedRoomsAmount := 1

	scheduler := &entities.Scheduler{
		Name: "some-name",
		Autoscaling: &autoscaling.Autoscaling{
			Policy: autoscaling.Policy{
				Type: autoscaling.RoomOccupancy,
				Parameters: autoscaling.PolicyParameters{
					RoomOccupancy: &autoscaling.RoomOccupancyParams{
						ReadyTarget: readyTarget,
					},
				},
			},
		},
	}

	t.Run("Success cases - when all gonna ok", func(t *testing.T) {
		roomStorageMock := mock.NewMockRoomStorage(ctrl)

		roomStorageMock.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).Return(occupiedRoomsAmount, nil)

		policy := roomoccupancy.NewPolicy(roomStorageMock)

		currentState, err := policy.CurrentStateBuilder(context.Background(), scheduler)
		assert.NoError(t, err)

		assert.Equal(t, readyTarget, currentState[roomoccupancy.ReadyTargetKey])
		assert.Equal(t, occupiedRoomsAmount, currentState[roomoccupancy.OccupiedRoomsKey])
	})

	t.Run("Error case - When GetRoomCountByStatus return in error returns nil error", func(t *testing.T) {
		roomStorageMock := mock.NewMockRoomStorage(ctrl)

		roomStorageMock.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).Return(-1, errors.New("Error getting amount of occupied rooms"))

		policy := roomoccupancy.NewPolicy(roomStorageMock)

		_, err := policy.CurrentStateBuilder(context.Background(), scheduler)
		assert.ErrorContains(t, err, "error fetching occupied game rooms amount:")
	})
}

func TestCalculateDesiredNumberOfRooms(t *testing.T) {
	policy := &roomoccupancy.Policy{}
	t.Run("Success case - when ready target is smaller than 1 return desired number and nil", func(t *testing.T) {
		t.Parallel()

		t.Run("First case", func(t *testing.T) {
			readyTarget := float64(0.5)
			occupiedRooms := 80

			schedulerState := policies.CurrentState{
				roomoccupancy.ReadyTargetKey:   readyTarget,
				roomoccupancy.OccupiedRoomsKey: occupiedRooms,
			}

			desiredNumberOfRoom, err := policy.CalculateDesiredNumberOfRooms(schedulerState)
			assert.NoError(t, err)
			assert.EqualValues(t, desiredNumberOfRoom, 160)
		})

		t.Run("Second case", func(t *testing.T) {
			readyTarget := float64(0.1)
			occupiedRooms := 5

			schedulerState := policies.CurrentState{
				roomoccupancy.ReadyTargetKey:   readyTarget,
				roomoccupancy.OccupiedRoomsKey: occupiedRooms,
			}

			desiredNumberOfRoom, err := policy.CalculateDesiredNumberOfRooms(schedulerState)
			assert.NoError(t, err)
			assert.EqualValues(t, desiredNumberOfRoom, 6)
		})

		t.Run("Third case", func(t *testing.T) {
			readyTarget := float64(0.3)
			occupiedRooms := 0

			schedulerState := policies.CurrentState{
				roomoccupancy.ReadyTargetKey:   readyTarget,
				roomoccupancy.OccupiedRoomsKey: occupiedRooms,
			}

			desiredNumberOfRoom, err := policy.CalculateDesiredNumberOfRooms(schedulerState)
			assert.NoError(t, err)
			assert.EqualValues(t, desiredNumberOfRoom, 0)
		})
	})

	t.Run("Fail case - when ready target is out of 0, 1 range", func(t *testing.T) {
		t.Parallel()

		t.Run("when ready target is 1", func(t *testing.T) {
			readyTarget := float64(1.0)
			occupiedRooms := 10

			schedulerState := policies.CurrentState{
				roomoccupancy.ReadyTargetKey:   readyTarget,
				roomoccupancy.OccupiedRoomsKey: occupiedRooms,
			}

			_, err := policy.CalculateDesiredNumberOfRooms(schedulerState)
			assert.EqualError(t, err, "Ready target must be between 0 and 1")
		})
		t.Run("when ready target is greater than 1", func(t *testing.T) {
			readyTarget := float64(1.1)
			occupiedRooms := 10

			schedulerState := policies.CurrentState{
				roomoccupancy.ReadyTargetKey:   readyTarget,
				roomoccupancy.OccupiedRoomsKey: occupiedRooms,
			}

			_, err := policy.CalculateDesiredNumberOfRooms(schedulerState)
			assert.EqualError(t, err, "Ready target must be between 0 and 1")
		})
		t.Run("when ready target is 0", func(t *testing.T) {
			readyTarget := float64(0.0)
			occupiedRooms := 10

			schedulerState := policies.CurrentState{
				roomoccupancy.ReadyTargetKey:   readyTarget,
				roomoccupancy.OccupiedRoomsKey: occupiedRooms,
			}

			_, err := policy.CalculateDesiredNumberOfRooms(schedulerState)
			assert.EqualError(t, err, "Ready target must be between 0 and 1")
		})
		t.Run("when ready target is lower than 0", func(t *testing.T) {
			readyTarget := float64(-0.1)
			occupiedRooms := 10

			schedulerState := policies.CurrentState{
				roomoccupancy.ReadyTargetKey:   readyTarget,
				roomoccupancy.OccupiedRoomsKey: occupiedRooms,
			}

			_, err := policy.CalculateDesiredNumberOfRooms(schedulerState)
			assert.EqualError(t, err, "Ready target must be between 0 and 1")
		})
	})
}
