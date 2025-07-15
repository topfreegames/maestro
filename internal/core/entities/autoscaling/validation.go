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

package autoscaling

import (
	"github.com/go-playground/validator/v10"
)

// RegisterPolicyValidation registers the struct-level validation for Policy
func RegisterPolicyValidation(validate *validator.Validate) error {
	// Register struct-level validation for Policy
	validate.RegisterStructValidation(policyStructLevelValidation, Policy{})
	return nil
}

// policyStructLevelValidation performs struct-level validation for Policy
func policyStructLevelValidation(sl validator.StructLevel) {
	policy := sl.Current().Interface().(Policy)

	switch policy.Type {
	case RoomOccupancy:
		if policy.Parameters.RoomOccupancy == nil {
			sl.ReportError(nil, "RoomOccupancy", "RoomOccupancy", "required_for_room_occupancy", "")
		}
	case FixedBuffer:
		if policy.Parameters.FixedBufferAmount == nil {
			sl.ReportError(nil, "FixedBufferAmount", "FixedBufferAmount", "required_for_fixed_buffer_amount", "")
		} else {
			value := *policy.Parameters.FixedBufferAmount
			if value <= 0 {
				sl.ReportError(nil, "FixedBufferAmount", "FixedBufferAmount", "gt", "0")
			}
		}
	}
}
