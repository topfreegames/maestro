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
	"github.com/topfreegames/maestro/internal/core/services/autoscaler"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies"
	policyMock "github.com/topfreegames/maestro/internal/core/services/autoscaler/policies/mock"
)

func TestCalculateDesiredNumberOfRooms(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedDesiredNumberOfRooms := 4

	policyType := autoscaling.PolicyType("some-policy-type")

	scheduler := &entities.Scheduler{
		Name: "some-name",
		Autoscaling: &autoscaling.Autoscaling{
			Policy: autoscaling.Policy{
				Type:       policyType,
				Parameters: autoscaling.PolicyParameters{},
			},
		},
	}

	t.Run("Success case", func(t *testing.T) {
		mockPolicy := policyMock.NewMockPolicy(ctrl)

		currentState := policies.CurrentState{}

		mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
		mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(scheduler.Autoscaling.Policy.Parameters, currentState).Return(expectedDesiredNumberOfRooms, nil)

		autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyFactory{policyType: mockPolicy})

		desiredNumberOfRoom, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
		assert.NoError(t, err)

		assert.Equal(t, expectedDesiredNumberOfRooms, desiredNumberOfRoom)
	})

	t.Run("Error cases", func(t *testing.T) {
		t.Run("When scheduler does not have autoscaling struct", func(t *testing.T) {
			autoscaler := autoscaler.Autoscaler{}

			scheduler := &entities.Scheduler{
				Name: "some-name",
			}

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "Scheduler does not have autoscaling struct")
		})

		t.Run("When policyFactory does not have policy retun in error", func(t *testing.T) {
			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyFactory{})

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "Error finding policy to scheduler")
		})

		t.Run("When CurrentStateBuilder returns error", func(t *testing.T) {
			mockPolicy := policyMock.NewMockPolicy(ctrl)

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(nil, errors.New("Error getting current state"))

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyFactory{policyType: mockPolicy})

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "Error fetching current state to scheduler")
		})

		t.Run("When CalculateDesiredNumberOfRooms returns error", func(t *testing.T) {
			mockPolicy := policyMock.NewMockPolicy(ctrl)

			currentState := policies.CurrentState{}

			mockPolicy.EXPECT().CurrentStateBuilder(gomock.Any(), scheduler).Return(currentState, nil)
			mockPolicy.EXPECT().CalculateDesiredNumberOfRooms(scheduler.Autoscaling.Policy.Parameters, currentState).Return(-1, errors.New("Error calculating desired number of rooms"))

			autoscaler := autoscaler.NewAutoscaler(autoscaler.PolicyFactory{policyType: mockPolicy})

			_, err := autoscaler.CalculateDesiredNumberOfRooms(context.Background(), scheduler)
			assert.ErrorContains(t, err, "Error calculating the desired number of rooms to scheduler")
		})
	})
}
