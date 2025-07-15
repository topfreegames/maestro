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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies/fixedbufferamount"
)

func TestCalculateDesiredNumberOfRooms_FixedBufferAmount(t *testing.T) {
	policy := &fixedbufferamount.Policy{}

	t.Run("returns desired = occupied + fixed amount", func(t *testing.T) {
		params := autoscaling.PolicyParameters{FixedBufferAmount: intPtr(10)}
		state := policies.CurrentState{
			fixedbufferamount.OccupiedRoomsKey: 5,
		}
		desired, err := policy.CalculateDesiredNumberOfRooms(params, state)
		assert.NoError(t, err)
		assert.Equal(t, 15, desired)
	})

	t.Run("returns error if occupied rooms missing", func(t *testing.T) {
		params := autoscaling.PolicyParameters{FixedBufferAmount: intPtr(10)}
		state := policies.CurrentState{}
		_, err := policy.CalculateDesiredNumberOfRooms(params, state)
		assert.Error(t, err)
	})
}

func TestCanDownscale_FixedBufferAmount(t *testing.T) {
	policy := &fixedbufferamount.Policy{}

	t.Run("can downscale if current total rooms > desired", func(t *testing.T) {
		params := autoscaling.PolicyParameters{FixedBufferAmount: intPtr(10)}
		state := policies.CurrentState{
			fixedbufferamount.OccupiedRoomsKey: 5,
			fixedbufferamount.TotalRoomsKey:    20,
		}
		can, err := policy.CanDownscale(params, state)
		assert.NoError(t, err)
		assert.True(t, can)
	})

	t.Run("cannot downscale if current total rooms == desired", func(t *testing.T) {
		params := autoscaling.PolicyParameters{FixedBufferAmount: intPtr(10)}
		state := policies.CurrentState{
			fixedbufferamount.OccupiedRoomsKey: 5,
			fixedbufferamount.TotalRoomsKey:    15,
		}
		can, err := policy.CanDownscale(params, state)
		assert.NoError(t, err)
		assert.False(t, can)
	})

	t.Run("cannot downscale if current total rooms < desired", func(t *testing.T) {
		params := autoscaling.PolicyParameters{FixedBufferAmount: intPtr(10)}
		state := policies.CurrentState{
			fixedbufferamount.OccupiedRoomsKey: 5,
			fixedbufferamount.TotalRoomsKey:    10,
		}
		can, err := policy.CanDownscale(params, state)
		assert.NoError(t, err)
		assert.False(t, can)
	})

	t.Run("returns error if total rooms missing", func(t *testing.T) {
		params := autoscaling.PolicyParameters{FixedBufferAmount: intPtr(10)}
		state := policies.CurrentState{
			fixedbufferamount.OccupiedRoomsKey: 5,
		}
		_, err := policy.CanDownscale(params, state)
		assert.Error(t, err)
	})
}

func intPtr(i int) *int { return &i }
