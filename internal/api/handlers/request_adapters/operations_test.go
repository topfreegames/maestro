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

package request_adapters_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/api/handlers/request_adapters"
	api "github.com/topfreegames/maestro/pkg/api/v1"
	_struct "google.golang.org/protobuf/types/known/structpb"
)

func TestFromOperationsToResponses(t *testing.T) {
	type Input struct {
		Operations []*operation.Operation
	}

	type Output struct {
		ApiOperations []*api.Operation
		GivesError    bool
	}

	type genericStruct struct {
		GenericAttribute string
	}
	genericString := "some-value"
	genericTime := time.Now()
	genericJsonStruct, _ := json.Marshal(genericStruct{"some-value"})

	inputMap := make(map[string]interface{})
	_ = json.Unmarshal([]byte(genericJsonStruct), &inputMap)
	genericApiStruct, _ := _struct.NewStruct(inputMap)

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "receives valid operation in list, returns list with converted operation",
			Input: Input{
				Operations: []*operation.Operation{
					{
						ID:             genericString,
						Status:         operation.StatusPending,
						DefinitionName: genericString,
						SchedulerName:  genericString,
						Lease: &operation.OperationLease{
							OperationID: genericString,
							Ttl:         genericTime,
						},
						CreatedAt: genericTime,
						Input:     []byte(genericJsonStruct),
						ExecutionHistory: []operation.OperationEvent{
							{
								CreatedAt: genericTime,
								Event:     genericString,
							},
						},
					},
				},
			},
			Output: Output{
				ApiOperations: []*api.Operation{
					{
						Id:             genericString,
						Status:         "pending",
						DefinitionName: genericString,
						Lease:          &api.Lease{Ttl: genericTime.UTC().Format(time.RFC3339)},
						SchedulerName:  genericString,
						CreatedAt:      timestamppb.New(genericTime),
						Input:          genericApiStruct,
						ExecutionHistory: []*api.OperationEvent{
							{
								CreatedAt: timestamppb.New(genericTime),
								Event:     genericString,
							},
						},
					},
				},
			},
		},
		{
			Title: "receives invalid operation in list, returns err",
			Input: Input{
				Operations: []*operation.Operation{
					{
						ID:             genericString,
						Status:         operation.StatusPending,
						DefinitionName: genericString,
						SchedulerName:  genericString,
						Lease: &operation.OperationLease{
							OperationID: genericString,
							Ttl:         genericTime,
						},
						CreatedAt: genericTime,
						Input:     []byte("some-value"),
						ExecutionHistory: []operation.OperationEvent{
							{
								CreatedAt: genericTime,
								Event:     genericString,
							},
						},
					},
				},
			},
			Output: Output{
				ApiOperations: nil,
				GivesError:    true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			returnValues, err := request_adapters.FromOperationsToResponses(testCase.Input.Operations)
			if testCase.Output.GivesError {
				assert.Error(t, err)
			}
			assert.EqualValues(t, testCase.Output.ApiOperations, returnValues)
		})
	}
}

func TestFromOperationToResponse(t *testing.T) {
	type Input struct {
		Operation *operation.Operation
	}

	type Output struct {
		ApiOperation *api.Operation
		GivesError   bool
	}

	type genericStruct struct {
		GenericAttribute string
	}
	genericString := "some-value"
	genericTime := time.Now()
	genericJsonStruct, _ := json.Marshal(genericStruct{"some-value"})

	inputMap := make(map[string]interface{})
	_ = json.Unmarshal([]byte(genericJsonStruct), &inputMap)
	genericApiStruct, _ := _struct.NewStruct(inputMap)

	testCases := []struct {
		Title string
		Input
		Output
	}{
		{
			Title: "receives valid complete operation, returns converted operation",
			Input: Input{
				Operation: &operation.Operation{
					ID:             genericString,
					Status:         operation.StatusPending,
					DefinitionName: genericString,
					SchedulerName:  genericString,
					Lease: &operation.OperationLease{
						OperationID: genericString,
						Ttl:         genericTime,
					},
					CreatedAt: genericTime,
					Input:     []byte(genericJsonStruct),
					ExecutionHistory: []operation.OperationEvent{
						{
							CreatedAt: genericTime,
							Event:     genericString,
						},
					},
				},
			},
			Output: Output{
				ApiOperation: &api.Operation{
					Id:             genericString,
					Status:         "pending",
					DefinitionName: genericString,
					Lease:          &api.Lease{Ttl: genericTime.UTC().Format(time.RFC3339)},
					SchedulerName:  genericString,
					CreatedAt:      timestamppb.New(genericTime),
					Input:          genericApiStruct,
					ExecutionHistory: []*api.OperationEvent{
						{
							CreatedAt: timestamppb.New(genericTime),
							Event:     genericString,
						},
					},
				},
			},
		},
		{
			Title: "receives operation without lease, returns converted operation",
			Input: Input{
				Operation: &operation.Operation{
					ID:             genericString,
					Status:         operation.StatusPending,
					DefinitionName: genericString,
					SchedulerName:  genericString,
					CreatedAt:      genericTime,
					ExecutionHistory: []operation.OperationEvent{
						{
							CreatedAt: genericTime,
							Event:     genericString,
						},
					},
				},
			},
			Output: Output{
				ApiOperation: &api.Operation{
					Id:             genericString,
					Status:         "pending",
					DefinitionName: genericString,
					SchedulerName:  genericString,
					CreatedAt:      timestamppb.New(genericTime),
					ExecutionHistory: []*api.OperationEvent{
						{
							CreatedAt: timestamppb.New(genericTime),
							Event:     genericString,
						},
					},
				},
			},
		},
		{
			Title: "receives invalid operation in list, returns err",
			Input: Input{
				Operation: &operation.Operation{
					ID:             genericString,
					Status:         operation.StatusPending,
					DefinitionName: genericString,
					SchedulerName:  genericString,
					Lease: &operation.OperationLease{
						OperationID: genericString,
						Ttl:         genericTime,
					},
					CreatedAt: genericTime,
					Input:     []byte("some-value"),
					ExecutionHistory: []operation.OperationEvent{
						{
							CreatedAt: genericTime,
							Event:     genericString,
						},
					},
				},
			},
			Output: Output{
				ApiOperation: nil,
				GivesError:   true,
			},
		},
		{
			Title: "receives operation with invalid status, returns error",
			Input: Input{
				Operation: &operation.Operation{
					ID:             genericString,
					Status:         55,
					DefinitionName: genericString,
					SchedulerName:  genericString,
					CreatedAt:      genericTime,
					Input:          []byte(genericJsonStruct),
					ExecutionHistory: []operation.OperationEvent{
						{
							CreatedAt: genericTime,
							Event:     genericString,
						},
					},
				},
			},
			Output: Output{
				ApiOperation: nil,
				GivesError:   true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			returnValues, err := request_adapters.FromOperationToResponse(testCase.Input.Operation)
			if testCase.Output.GivesError {
				assert.Error(t, err)
			}
			assert.EqualValues(t, testCase.Output.ApiOperation, returnValues)
		})
	}
}
