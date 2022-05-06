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

package operation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

func TestNewOperation(t *testing.T) {
	type Input struct {
		SchedulerName   string
		DefinitionName  string
		DefinitionInput []byte
	}

	type Output struct {
		Operation *operation.Operation
	}

	genericString := "some-value"

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "Creates operation successfully",
			Input: Input{
				SchedulerName:   genericString,
				DefinitionName:  genericString,
				DefinitionInput: []byte("test"),
			},
			Output: Output{
				Operation: &operation.Operation{
					Status:         operation.StatusPending,
					DefinitionName: genericString,
					SchedulerName:  genericString,
					Input:          []byte("test"),
					ExecutionHistory: []operation.OperationEvent{
						{
							Event: "Created",
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			in := testCase.Input
			out := testCase.Output.Operation
			op := operation.New(in.SchedulerName, in.DefinitionName, in.DefinitionInput)

			assert.Equal(t, op.SchedulerName, out.SchedulerName)
			assert.Equal(t, op.Input, out.Input)
			assert.Equal(t, op.ExecutionHistory[0].Event, out.ExecutionHistory[0].Event)
			assert.Equal(t, op.Status, out.Status)
			assert.Equal(t, op.DefinitionName, out.DefinitionName)

			assert.NotEqual(t, "", op.ID)

			assert.NotNil(t, op.CreatedAt)
			assert.NotNil(t, op.ExecutionHistory[0].CreatedAt)

			assert.Nil(t, op.Lease)
		})
	}
}
