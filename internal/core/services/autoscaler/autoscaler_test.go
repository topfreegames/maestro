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
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	autoscalerPorts "github.com/topfreegames/maestro/internal/core/ports/autoscaler"
	"github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies"
	policyMock "github.com/topfreegames/maestro/internal/core/services/autoscaler/policies/mock"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies/roomoccupancy"
)

func TestCalculateDesiredNumberOfRooms(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedDesiredNumberOfRooms := 4

	scheduler := &entities.Scheduler{
		Name:        "some-name",
		Autoscaling: &autoscaling.Autoscaling{},
	}

	t.Run("Success case", func(t *testing.T) {
		mockPolicy := policyMock.NewMockPolicy(ctrl)

		currentState := policies.CurrentState{}

		mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
		mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(currentState).Return(expectedDesiredNumberOfRooms, nil)

		autoscaler := autoscaler.Autoscaler{
			PolicyFactory: func(autoscaling.PolicyType) (autoscalerPorts.Policy, error) {
				return mockPolicy, nil
			},
		}

		desiredNumberOfRoom, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
		assert.NoError(t, err)

		assert.Equal(t, expectedDesiredNumberOfRooms, desiredNumberOfRoom)
	})

	t.Run("Error cases", func(t *testing.T) {
		t.Run("When policy factory retun in error", func(t *testing.T) {
			autoscaler := autoscaler.Autoscaler{
				PolicyFactory: func(autoscaling.PolicyType) (autoscalerPorts.Policy, error) {
					return nil, errors.New("Error getting policy to policyType")
				},
			}

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "Error building policy to scheduler")
		})

		t.Run("When CurrentStateBuilder returns error", func(t *testing.T) {
			mockPolicy := policyMock.NewMockPolicy(ctrl)

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(nil, errors.New("Error getting current state"))

			autoscaler := autoscaler.Autoscaler{
				PolicyFactory: func(autoscaling.PolicyType) (autoscalerPorts.Policy, error) {
					return mockPolicy, nil
				},
			}

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "Error fetching current state to scheduler")
		})

		t.Run("When CalculateDesiredNumberOfRooms returns error", func(t *testing.T) {
			mockPolicy := policyMock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(currentState).Return(-1, errors.New("Error calculating desired number of rooms"))

			autoscaler := autoscaler.Autoscaler{
				PolicyFactory: func(autoscaling.PolicyType) (autoscalerPorts.Policy, error) {
					return mockPolicy, nil
				},
			}

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "Error calculating the desired number of rooms to scheduler")
		})
	})
}

func TestDefaultPolicyFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("Success case", func(t *testing.T) {
		roomStorageMock := mock.NewMockRoomStorage(ctrl)

		autoscaler := autoscaler.NewAutoscaler(roomStorageMock)

		policy, err := autoscaler.DefaultPolicyFactory(autoscaling.RoomOccupancy)
		assert.NoError(t, err)

		assert.IsType(t, &roomoccupancy.Policy{}, policy)
	})

	t.Run("Error case", func(t *testing.T) {
		roomStorageMock := mock.NewMockRoomStorage(ctrl)

		autoscaler := autoscaler.NewAutoscaler(roomStorageMock)
		_, err := autoscaler.DefaultPolicyFactory(autoscaling.PolicyType("unknown-policy-type"))
		assert.ErrorContains(t, err, "Autoscaling policy not found")
	})
}
