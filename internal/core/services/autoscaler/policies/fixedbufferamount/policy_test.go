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

package fixedbufferamount_test

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
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies/fixedbufferamount"
)

func TestCurrentStateBuilder_FixedBufferAmount(t *testing.T) {
	t.Run("successfully builds current state with total count excluding terminating and error rooms", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRoomStorage := mock.NewMockRoomStorage(ctrl)
		policy := fixedbufferamount.NewPolicy(mockRoomStorage)
		scheduler := &entities.Scheduler{Name: "test-scheduler"}

		// Setup mock expectations
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusOccupied).Return(10, nil)
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusTerminating).Return(2, nil)
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusError).Return(1, nil)
		mockRoomStorage.EXPECT().GetRoomCount(gomock.Any(), "test-scheduler").Return(20, nil)

		// Execute
		state, err := policy.CurrentStateBuilder(context.Background(), scheduler)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 10, state[fixedbufferamount.OccupiedRoomsKey])
		assert.Equal(t, 17, state[fixedbufferamount.TotalRoomsKey]) // 20 - 2 - 1 = 17
	})

	t.Run("successfully builds current state with no terminating or error rooms", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRoomStorage := mock.NewMockRoomStorage(ctrl)
		policy := fixedbufferamount.NewPolicy(mockRoomStorage)
		scheduler := &entities.Scheduler{Name: "test-scheduler"}

		// Setup mock expectations
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusOccupied).Return(5, nil)
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusTerminating).Return(0, nil)
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusError).Return(0, nil)
		mockRoomStorage.EXPECT().GetRoomCount(gomock.Any(), "test-scheduler").Return(15, nil)

		// Execute
		state, err := policy.CurrentStateBuilder(context.Background(), scheduler)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 5, state[fixedbufferamount.OccupiedRoomsKey])
		assert.Equal(t, 15, state[fixedbufferamount.TotalRoomsKey]) // 15 - 0 - 0 = 15
	})

	t.Run("returns error when getting occupied rooms count fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRoomStorage := mock.NewMockRoomStorage(ctrl)
		policy := fixedbufferamount.NewPolicy(mockRoomStorage)
		scheduler := &entities.Scheduler{Name: "test-scheduler"}

		// Setup mock expectations
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusOccupied).Return(0, errors.New("storage error"))

		// Execute
		state, err := policy.CurrentStateBuilder(context.Background(), scheduler)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, state)
		assert.Contains(t, err.Error(), "error fetching occupied game rooms amount")
	})

	t.Run("returns error when getting terminating rooms count fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRoomStorage := mock.NewMockRoomStorage(ctrl)
		policy := fixedbufferamount.NewPolicy(mockRoomStorage)
		scheduler := &entities.Scheduler{Name: "test-scheduler"}

		// Setup mock expectations
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusOccupied).Return(10, nil)
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusTerminating).Return(0, errors.New("storage error"))

		// Execute
		state, err := policy.CurrentStateBuilder(context.Background(), scheduler)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, state)
		assert.Contains(t, err.Error(), "error fetching terminating game rooms amount")
	})

	t.Run("returns error when getting error rooms count fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRoomStorage := mock.NewMockRoomStorage(ctrl)
		policy := fixedbufferamount.NewPolicy(mockRoomStorage)
		scheduler := &entities.Scheduler{Name: "test-scheduler"}

		// Setup mock expectations
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusOccupied).Return(10, nil)
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusTerminating).Return(2, nil)
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusError).Return(0, errors.New("storage error"))

		// Execute
		state, err := policy.CurrentStateBuilder(context.Background(), scheduler)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, state)
		assert.Contains(t, err.Error(), "error fetching error game rooms amount")
	})

	t.Run("returns error when getting total rooms count fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRoomStorage := mock.NewMockRoomStorage(ctrl)
		policy := fixedbufferamount.NewPolicy(mockRoomStorage)
		scheduler := &entities.Scheduler{Name: "test-scheduler"}

		// Setup mock expectations
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusOccupied).Return(10, nil)
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusTerminating).Return(2, nil)
		mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), "test-scheduler", game_room.GameStatusError).Return(1, nil)
		mockRoomStorage.EXPECT().GetRoomCount(gomock.Any(), "test-scheduler").Return(0, errors.New("storage error"))

		// Execute
		state, err := policy.CurrentStateBuilder(context.Background(), scheduler)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, state)
		assert.Contains(t, err.Error(), "error fetching total game rooms amount")
	})
}

func TestCalculateDesiredNumberOfRooms_FixedBufferAmount(t *testing.T) {
	policy := &fixedbufferamount.Policy{}

	t.Run("returns desired = occupied + fixed amount", func(t *testing.T) {
		params := autoscaling.PolicyParameters{FixedBuffer: &autoscaling.FixedBufferParams{Amount: 10}}
		state := policies.CurrentState{
			fixedbufferamount.OccupiedRoomsKey: 5,
		}
		desired, err := policy.CalculateDesiredNumberOfRooms(params, state)
		assert.NoError(t, err)
		assert.Equal(t, 15, desired)
	})

	t.Run("returns error if occupied rooms missing", func(t *testing.T) {
		params := autoscaling.PolicyParameters{FixedBuffer: &autoscaling.FixedBufferParams{Amount: 10}}
		state := policies.CurrentState{}
		_, err := policy.CalculateDesiredNumberOfRooms(params, state)
		assert.Error(t, err)
	})
}

func TestCanDownscale_FixedBufferAmount(t *testing.T) {
	policy := &fixedbufferamount.Policy{}

	t.Run("can downscale if current total rooms > desired", func(t *testing.T) {
		params := autoscaling.PolicyParameters{FixedBuffer: &autoscaling.FixedBufferParams{Amount: 10}}
		state := policies.CurrentState{
			fixedbufferamount.OccupiedRoomsKey: 5,
			fixedbufferamount.TotalRoomsKey:    20,
		}
		can, err := policy.CanDownscale(params, state)
		assert.NoError(t, err)
		assert.True(t, can)
	})

	t.Run("cannot downscale if current total rooms == desired", func(t *testing.T) {
		params := autoscaling.PolicyParameters{FixedBuffer: &autoscaling.FixedBufferParams{Amount: 10}}
		state := policies.CurrentState{
			fixedbufferamount.OccupiedRoomsKey: 5,
			fixedbufferamount.TotalRoomsKey:    15,
		}
		can, err := policy.CanDownscale(params, state)
		assert.NoError(t, err)
		assert.False(t, can)
	})

	t.Run("cannot downscale if current total rooms < desired", func(t *testing.T) {
		params := autoscaling.PolicyParameters{FixedBuffer: &autoscaling.FixedBufferParams{Amount: 10}}
		state := policies.CurrentState{
			fixedbufferamount.OccupiedRoomsKey: 5,
			fixedbufferamount.TotalRoomsKey:    10,
		}
		can, err := policy.CanDownscale(params, state)
		assert.NoError(t, err)
		assert.False(t, can)
	})

	t.Run("returns error if total rooms missing", func(t *testing.T) {
		params := autoscaling.PolicyParameters{FixedBuffer: &autoscaling.FixedBufferParams{Amount: 10}}
		state := policies.CurrentState{
			fixedbufferamount.OccupiedRoomsKey: 5,
		}
		_, err := policy.CanDownscale(params, state)
		assert.Error(t, err)
	})
}
