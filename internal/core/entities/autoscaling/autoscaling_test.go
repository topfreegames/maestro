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

package autoscaling

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"

	"github.com/topfreegames/maestro/internal/validations"
)

func TestNewAutoscaling(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	// Register struct-level validation for Policy
	RegisterPolicyValidation(validations.Validate)

	translator := validations.GetDefaultTranslator()

	validRoomOccupancyPolicy := Policy{
		Type: RoomOccupancy,
		Parameters: PolicyParameters{
			RoomOccupancy: &RoomOccupancyParams{
				ReadyTarget:   0.2,
				DownThreshold: 0.1,
			},
		},
	}

	validFixedBufferAmountPolicy := Policy{
		Type: FixedBuffer,
		Parameters: PolicyParameters{
			FixedBufferAmount: func() *int { v := 5; return &v }(),
		},
	}

	t.Run("invalid scenarios", func(t *testing.T) {
		t.Run("fails when try to create autoscaling with invalid Min", func(t *testing.T) {
			_, err := NewAutoscaling(true, -1, 10, 10, validRoomOccupancyPolicy)
			assert.Error(t, err)
			validationErrs := err.(validator.ValidationErrors)
			assert.Equal(t, "Min must be 1 or greater", validationErrs[0].Translate(translator))

			_, err = NewAutoscaling(true, 0, 10, 10, validRoomOccupancyPolicy)
			assert.Error(t, err)
			validationErrs = err.(validator.ValidationErrors)
			assert.Equal(t, "Min must be 1 or greater", validationErrs[0].Translate(translator))

			_, err = NewAutoscaling(true, 11, 10, 10, validRoomOccupancyPolicy)
			assert.Error(t, err)
			validationErrs = err.(validator.ValidationErrors)
			assert.Equal(t, "Min must be a number lower than Max", validationErrs[0].Translate(translator))

		})
		t.Run("fails when try to create autoscaling with invalid Max", func(t *testing.T) {
			_, err := NewAutoscaling(true, 1, -2, 10, validRoomOccupancyPolicy)
			validationErrs := err.(validator.ValidationErrors)
			assert.Equal(t, "Min must be a number lower than Max", validationErrs[0].Translate(translator))
			assert.Equal(t, "Max must be -1 or greater", validationErrs[1].Translate(translator))
		})

		t.Run("fails when try to create autoscaling with invalid Cooldown", func(t *testing.T) {
			_, err := NewAutoscaling(true, 1, 10, -1, validRoomOccupancyPolicy)
			validationErrs := err.(validator.ValidationErrors)
			assert.Equal(t, "Cooldown must be 0 or greater", validationErrs[0].Translate(translator))
		})

		t.Run("fails when try to create autoscaling with invalid roomOccupancy Policy", func(t *testing.T) {
			_, err := NewAutoscaling(true, 1, 10, 10, Policy{})
			validationErrs := err.(validator.ValidationErrors)
			assert.Contains(t, validationErrs[0].Translate(translator), "Type must be one of")

			_, err = NewAutoscaling(true, 1, 10, 10, Policy{Type: "invalid", Parameters: PolicyParameters{}})
			validationErrs = err.(validator.ValidationErrors)
			assert.Contains(t, validationErrs[0].Translate(translator), "Type must be one of")

			_, err = NewAutoscaling(true, 1, 10, 10, Policy{Type: "roomOccupancy", Parameters: PolicyParameters{}})
			validationErrs = err.(validator.ValidationErrors)
			assert.Equal(t, "RoomOccupancy must not be nil for RoomOccupancy policy type", validationErrs[0].Translate(translator))

			_, err = NewAutoscaling(true, 1, 10, 10, Policy{Type: "roomOccupancy", Parameters: PolicyParameters{RoomOccupancy: &RoomOccupancyParams{ReadyTarget: 0.0, DownThreshold: 0.1}}})
			validationErrs = err.(validator.ValidationErrors)
			assert.Equal(t, "ReadyTarget must be greater than 0", validationErrs[0].Translate(translator))

			_, err = NewAutoscaling(true, 1, 10, 10, Policy{Type: "roomOccupancy", Parameters: PolicyParameters{RoomOccupancy: &RoomOccupancyParams{ReadyTarget: 1, DownThreshold: 0.1}}})
			validationErrs = err.(validator.ValidationErrors)
			assert.Equal(t, "ReadyTarget must be less than 1", validationErrs[0].Translate(translator))

			_, err = NewAutoscaling(true, 1, 10, 10, Policy{Type: "roomOccupancy", Parameters: PolicyParameters{RoomOccupancy: &RoomOccupancyParams{ReadyTarget: 0.5, DownThreshold: 0}}})
			validationErrs = err.(validator.ValidationErrors)
			assert.Equal(t, "DownThreshold must be greater than 0", validationErrs[0].Translate(translator))

			_, err = NewAutoscaling(true, 1, 10, 10, Policy{Type: "roomOccupancy", Parameters: PolicyParameters{RoomOccupancy: &RoomOccupancyParams{ReadyTarget: 0.5, DownThreshold: 1}}})
			validationErrs = err.(validator.ValidationErrors)
			assert.Equal(t, "DownThreshold must be less than 1", validationErrs[0].Translate(translator))
		})

		t.Run("fails when try to create autoscaling with invalid fixedBuffer Policy", func(t *testing.T) {
			_, err := NewAutoscaling(true, 1, 10, 10, Policy{Type: "fixedBuffer", Parameters: PolicyParameters{}})
			validationErrs := err.(validator.ValidationErrors)
			assert.Equal(t, "FixedBufferAmount must not be nil for FixedBuffer policy type", validationErrs[0].Translate(translator))

			zeroValue := 0
			_, err = NewAutoscaling(true, 1, 10, 10, Policy{Type: "fixedBuffer", Parameters: PolicyParameters{FixedBufferAmount: &zeroValue}})
			validationErrs = err.(validator.ValidationErrors)
			assert.Equal(t, "FixedBufferAmount must be greater than 0", validationErrs[0].Translate(translator))

			negativeValue := -1
			_, err = NewAutoscaling(true, 1, 10, 10, Policy{Type: "fixedBuffer", Parameters: PolicyParameters{FixedBufferAmount: &negativeValue}})
			validationErrs = err.(validator.ValidationErrors)
			assert.Equal(t, "FixedBufferAmount must be greater than 0", validationErrs[0].Translate(translator))
		})
	})

	t.Run("valid scenarios", func(t *testing.T) {
		t.Run("success when try to create valid autoscaling with roomOccupancy type", func(t *testing.T) {
			_, err := NewAutoscaling(true, 10, -1, 0, validRoomOccupancyPolicy)
			assert.NoError(t, err)

			_, err = NewAutoscaling(false, 1, 1, 10, validRoomOccupancyPolicy)
			assert.NoError(t, err)

			_, err = NewAutoscaling(false, 50, 100, 100, validRoomOccupancyPolicy)
			assert.NoError(t, err)
		})

		t.Run("success when try to create valid autoscaling with fixedBufferAmount type", func(t *testing.T) {
			_, err := NewAutoscaling(true, 10, -1, 0, validFixedBufferAmountPolicy)
			assert.NoError(t, err)

			_, err = NewAutoscaling(false, 1, 1, 10, validFixedBufferAmountPolicy)
			assert.NoError(t, err)

			_, err = NewAutoscaling(false, 50, 100, 100, validFixedBufferAmountPolicy)
			assert.NoError(t, err)
		})
	})

}
