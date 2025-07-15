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

package autoscaler_test

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
	"github.com/topfreegames/maestro/internal/core/services/autoscaler"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies/fixedbufferamount"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies/roomoccupancy"
)

func TestCalculateDesiredNumberOfRooms(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedDesiredNumberOfRooms := 4
	minimumNumberOfRooms := 1
	maximumNumberOfRooms := 5

	policyType := autoscaling.PolicyType("some-policy-type")

	scheduler := &entities.Scheduler{
		Name: "some-name",
		Autoscaling: &autoscaling.Autoscaling{
			Enabled: true,
			Min:     minimumNumberOfRooms,
			Max:     maximumNumberOfRooms,
			Policy: autoscaling.Policy{
				Type:       policyType,
				Parameters: autoscaling.PolicyParameters{},
			},
		},
	}

	t.Run("Success cases", func(t *testing.T) {
		t.Run("When DesiredNumber is between Min and Max", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(scheduler.Autoscaling.Policy.Parameters, currentState).Return(expectedDesiredNumberOfRooms, nil)

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			desiredNumberOfRoom, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.Equal(t, expectedDesiredNumberOfRooms, desiredNumberOfRoom)
		})

		t.Run("When DesiredNumber is lower than Min should return Min", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(scheduler.Autoscaling.Policy.Parameters, currentState).Return(minimumNumberOfRooms-1, nil)

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			desiredNumberOfRoom, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.Equal(t, minimumNumberOfRooms, desiredNumberOfRoom)
		})

		t.Run("When DesiredNumber is greater than Max should return Max", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(scheduler.Autoscaling.Policy.Parameters, currentState).Return(maximumNumberOfRooms+1, nil)

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			desiredNumberOfRoom, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.Equal(t, maximumNumberOfRooms, desiredNumberOfRoom)
		})

		t.Run("When DesiredNumber is greater than Max but Max is -1 do not cap", func(t *testing.T) {
			scheduler := &entities.Scheduler{
				Name: "some-name",
				Autoscaling: &autoscaling.Autoscaling{
					Enabled: true,
					Min:     minimumNumberOfRooms,
					Max:     -1,
					Policy: autoscaling.Policy{
						Type:       policyType,
						Parameters: autoscaling.PolicyParameters{},
					},
				},
			}
			hugeAmountOfRooms := 10000
			mockPolicy := mock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(scheduler.Autoscaling.Policy.Parameters, currentState).Return(hugeAmountOfRooms, nil)

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			desiredNumberOfRoom, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.Equal(t, hugeAmountOfRooms, desiredNumberOfRoom)
		})
	})

	t.Run("Error cases", func(t *testing.T) {
		t.Run("When scheduler does not have autoscaling struct", func(t *testing.T) {
			autoscaler := autoscaler.Autoscaler{}

			scheduler := &entities.Scheduler{
				Name: "some-name",
			}

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "scheduler does not have autoscaling struct")
		})

		t.Run("When policyMap does not have policy return an error", func(t *testing.T) {
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{})

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "error finding policy to scheduler")
		})

		t.Run("When CurrentStateBuilder returns error", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(nil, errors.New("Error getting current state"))

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "error fetching current state to scheduler")
		})

		t.Run("When CalculateDesiredNumberOfRooms returns error", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(scheduler.Autoscaling.Policy.Parameters, currentState).Return(-1, errors.New("Error calculating desired number of rooms"))

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "error calculating the desired number of rooms to scheduler")
		})
	})
}

func TestCanDownscale(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	policyType := autoscaling.PolicyType("some-policy-type")

	scheduler := &entities.Scheduler{
		Name: "some-name",
		Autoscaling: &autoscaling.Autoscaling{
			Enabled: true,
			Min:     1,
			Max:     5,
			Policy: autoscaling.Policy{
				Type: policyType,
				Parameters: autoscaling.PolicyParameters{
					RoomOccupancy: &autoscaling.RoomOccupancyParams{
						ReadyTarget:   0.5,
						DownThreshold: 0.7,
					},
				},
			},
		},
		MatchAllocation: &allocation.MatchAllocation{MaxMatches: 1},
	}

	t.Run("Success cases", func(t *testing.T) {
		t.Run("When the occupation rate is below the threshold", func(t *testing.T) {
			mockRoomStorage := mock.NewMockRoomStorage(ctrl)

			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusReady).Return(4, nil)
			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusActive).Return(0, nil)
			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).Return(0, nil)
			mockRoomStorage.EXPECT().GetRunningMatchesCount(gomock.Any(), scheduler.Name).Return(1, nil)

			policy := roomoccupancy.NewPolicy(mockRoomStorage)
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: policy})

			allow, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.True(t, allow)
		})

		t.Run("When the occupation rate is above the threshold", func(t *testing.T) {
			mockRoomStorage := mock.NewMockRoomStorage(ctrl)

			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusReady).Return(2, nil)
			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusActive).Return(0, nil)
			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).Return(3, nil)
			mockRoomStorage.EXPECT().GetRunningMatchesCount(gomock.Any(), scheduler.Name).Return(3, nil)

			policy := roomoccupancy.NewPolicy(mockRoomStorage)
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: policy})

			allow, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.False(t, allow)
		})

		t.Run("When the occupation rate is equal the threshold", func(t *testing.T) {
			mockRoomStorage := mock.NewMockRoomStorage(ctrl)

			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusReady).Return(70, nil)
			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusActive).Return(0, nil)
			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).Return(30, nil)
			mockRoomStorage.EXPECT().GetRunningMatchesCount(gomock.Any(), scheduler.Name).Return(30, nil)

			policy := roomoccupancy.NewPolicy(mockRoomStorage)
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: policy})

			allow, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.True(t, allow)
		})

		t.Run("When the ready target is low it should trigger downscale", func(t *testing.T) {
			mockRoomStorage := mock.NewMockRoomStorage(ctrl)

			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusReady).Return(500, nil)
			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusActive).Return(0, nil)
			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).Return(80, nil)
			mockRoomStorage.EXPECT().GetRunningMatchesCount(gomock.Any(), scheduler.Name).Return(80, nil)

			policy := roomoccupancy.NewPolicy(mockRoomStorage)
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: policy})

			cpy := *scheduler
			cpy.Autoscaling.Policy.Parameters.RoomOccupancy.ReadyTarget = 0.35
			cpy.Autoscaling.Policy.Parameters.RoomOccupancy.DownThreshold = 0.9

			allow, err := autoscaler.CanDownscale(context.Background(), &cpy)
			assert.NoError(t, err)

			assert.True(t, allow)
		})
	})

	t.Run("Error cases", func(t *testing.T) {
		t.Run("When scheduler does not have autoscaling struct", func(t *testing.T) {
			autoscaler := autoscaler.Autoscaler{}

			scheduler := &entities.Scheduler{
				Name: "some-name",
			}

			_, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.ErrorContains(t, err, "scheduler does not have autoscaling struct")
		})

		t.Run("When policyMap does not have policy return in error", func(t *testing.T) {
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{})

			_, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.ErrorContains(t, err, "error finding policy to scheduler")
		})

		t.Run("When CurrentStateBuilder returns error", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(nil, errors.New("Error getting current state"))

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			_, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.ErrorContains(t, err, "error fetching current state to scheduler")
		})

		t.Run("When policy CanDownscale returns error", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CanDownscale(scheduler.Autoscaling.Policy.Parameters, currentState).Return(false, errors.New("error checking if can downscale"))

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			_, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.ErrorContains(t, err, "error checking if can downscale")
		})
	})
}

func TestCalculateDesiredNumberOfRooms_FixedBufferAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	policyType := autoscaling.PolicyType("fixedBufferAmount")

	scheduler := &entities.Scheduler{
		Name: "some-name",
		Autoscaling: &autoscaling.Autoscaling{
			Enabled: true,
			Min:     1,
			Max:     10,
			Policy: autoscaling.Policy{
				Type: policyType,
				Parameters: autoscaling.PolicyParameters{
					FixedBuffer: &autoscaling.FixedBufferParams{
						Amount: 5,
					},
				},
			},
		},
	}

	t.Run("Success cases", func(t *testing.T) {
		t.Run("When DesiredNumber is between Min and Max", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{
				fixedbufferamount.OccupiedRoomsKey: 3,
			}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(scheduler.Autoscaling.Policy.Parameters, currentState).Return(8, nil)

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			desiredNumberOfRoom, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.Equal(t, 8, desiredNumberOfRoom)
		})

		t.Run("When DesiredNumber is lower than Min should return Min", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{
				fixedbufferamount.OccupiedRoomsKey: 0,
			}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(scheduler.Autoscaling.Policy.Parameters, currentState).Return(0, nil)

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			desiredNumberOfRoom, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.Equal(t, 1, desiredNumberOfRoom)
		})

		t.Run("When DesiredNumber is greater than Max should return Max", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{
				fixedbufferamount.OccupiedRoomsKey: 8,
			}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(scheduler.Autoscaling.Policy.Parameters, currentState).Return(15, nil)

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			desiredNumberOfRoom, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.Equal(t, 10, desiredNumberOfRoom)
		})

		t.Run("When DesiredNumber is greater than Max but Max is -1 do not cap", func(t *testing.T) {
			scheduler := &entities.Scheduler{
				Name: "some-name",
				Autoscaling: &autoscaling.Autoscaling{
					Enabled: true,
					Min:     1,
					Max:     -1,
					Policy: autoscaling.Policy{
						Type: policyType,
						Parameters: autoscaling.PolicyParameters{
							FixedBuffer: &autoscaling.FixedBufferParams{
								Amount: 100,
							},
						},
					},
				},
			}
			hugeAmountOfRooms := 10000
			mockPolicy := mock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{
				fixedbufferamount.OccupiedRoomsKey: 9900,
			}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(scheduler.Autoscaling.Policy.Parameters, currentState).Return(hugeAmountOfRooms, nil)

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			desiredNumberOfRoom, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.Equal(t, hugeAmountOfRooms, desiredNumberOfRoom)
		})
	})

	t.Run("Error cases", func(t *testing.T) {
		t.Run("When scheduler does not have autoscaling struct", func(t *testing.T) {
			autoscaler := autoscaler.Autoscaler{}

			scheduler := &entities.Scheduler{
				Name: "some-name",
			}

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "scheduler does not have autoscaling struct")
		})

		t.Run("When policyMap does not have policy return an error", func(t *testing.T) {
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{})

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "error finding policy to scheduler")
		})

		t.Run("When CurrentStateBuilder returns error", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(nil, errors.New("Error getting current state"))

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "error fetching current state to scheduler")
		})

		t.Run("When CalculateDesiredNumberOfRooms returns error", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{
				fixedbufferamount.OccupiedRoomsKey: 3,
			}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(scheduler.Autoscaling.Policy.Parameters, currentState).Return(-1, errors.New("Error calculating desired number of rooms"))

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "error calculating the desired number of rooms to scheduler")
		})
	})
}

func TestCanDownscale_FixedBufferAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	policyType := autoscaling.PolicyType("fixedBufferAmount")

	scheduler := &entities.Scheduler{
		Name: "some-name",
		Autoscaling: &autoscaling.Autoscaling{
			Enabled: true,
			Min:     1,
			Max:     10,
			Policy: autoscaling.Policy{
				Type: policyType,
				Parameters: autoscaling.PolicyParameters{
					FixedBuffer: &autoscaling.FixedBufferParams{
						Amount: 5,
					},
				},
			},
		},
		MatchAllocation: &allocation.MatchAllocation{MaxMatches: 1},
	}

	t.Run("Success cases", func(t *testing.T) {
		t.Run("When current total rooms is greater than desired (occupied + fixed amount)", func(t *testing.T) {
			mockRoomStorage := mock.NewMockRoomStorage(ctrl)

			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).Return(3, nil)
			mockRoomStorage.EXPECT().GetRoomCount(gomock.Any(), scheduler.Name).Return(10, nil)

			policy := fixedbufferamount.NewPolicy(mockRoomStorage)
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: policy})

			allow, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.True(t, allow)
		})

		t.Run("When current total rooms equals desired (occupied + fixed amount)", func(t *testing.T) {
			mockRoomStorage := mock.NewMockRoomStorage(ctrl)

			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).Return(3, nil)
			mockRoomStorage.EXPECT().GetRoomCount(gomock.Any(), scheduler.Name).Return(8, nil)

			policy := fixedbufferamount.NewPolicy(mockRoomStorage)
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: policy})

			allow, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.False(t, allow)
		})

		t.Run("When current total rooms is less than desired (occupied + fixed amount)", func(t *testing.T) {
			mockRoomStorage := mock.NewMockRoomStorage(ctrl)

			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).Return(3, nil)
			mockRoomStorage.EXPECT().GetRoomCount(gomock.Any(), scheduler.Name).Return(5, nil)

			policy := fixedbufferamount.NewPolicy(mockRoomStorage)
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: policy})

			allow, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.False(t, allow)
		})

		t.Run("When no occupied rooms but has fixed buffer amount", func(t *testing.T) {
			mockRoomStorage := mock.NewMockRoomStorage(ctrl)

			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).Return(0, nil)
			mockRoomStorage.EXPECT().GetRoomCount(gomock.Any(), scheduler.Name).Return(10, nil)

			policy := fixedbufferamount.NewPolicy(mockRoomStorage)
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: policy})

			allow, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.True(t, allow)
		})

		t.Run("When high occupied rooms with small fixed buffer", func(t *testing.T) {
			mockRoomStorage := mock.NewMockRoomStorage(ctrl)

			mockRoomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).Return(80, nil)
			mockRoomStorage.EXPECT().GetRoomCount(gomock.Any(), scheduler.Name).Return(90, nil)

			policy := fixedbufferamount.NewPolicy(mockRoomStorage)
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: policy})

			allow, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.NoError(t, err)

			assert.True(t, allow)
		})
	})

	t.Run("Error cases", func(t *testing.T) {
		t.Run("When scheduler does not have autoscaling struct", func(t *testing.T) {
			autoscaler := autoscaler.Autoscaler{}

			scheduler := &entities.Scheduler{
				Name: "some-name",
			}

			_, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.ErrorContains(t, err, "scheduler does not have autoscaling struct")
		})

		t.Run("When policyMap does not have policy return in error", func(t *testing.T) {
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{})

			_, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.ErrorContains(t, err, "error finding policy to scheduler")
		})

		t.Run("When CurrentStateBuilder returns error", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(nil, errors.New("Error getting current state"))

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			_, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.ErrorContains(t, err, "error fetching current state to scheduler")
		})

		t.Run("When policy CanDownscale returns error", func(t *testing.T) {
			mockPolicy := mock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{
				fixedbufferamount.OccupiedRoomsKey: 3,
				fixedbufferamount.TotalRoomsKey:    10,
			}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CanDownscale(scheduler.Autoscaling.Policy.Parameters, currentState).Return(false, errors.New("error checking if can downscale"))

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyMap{policyType: mockPolicy})

			_, err := autoscaler.CanDownscale(context.Background(), scheduler)
			assert.ErrorContains(t, err, "error checking if can downscale")
		})
	})
}
