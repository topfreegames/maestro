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

package requestadapters

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	api "github.com/topfreegames/maestro/pkg/api/v1"
	_struct "google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func FromOperationsToResponses(entities []*operation.Operation) ([]*api.Operation, error) {
	responses := make([]*api.Operation, len(entities))
	for i, entity := range entities {
		response, err := FromOperationToResponse(entity)
		if err != nil {
			return nil, err
		}
		responses[i] = response
	}

	return responses, nil
}

func FromOperationToResponse(entity *operation.Operation) (*api.Operation, error) {
	apiOperation := &api.Operation{
		Id:             entity.ID,
		DefinitionName: entity.DefinitionName,
		SchedulerName:  entity.SchedulerName,
		CreatedAt:      timestamppb.New(entity.CreatedAt),
	}

	var err error
	apiOperation.Status, err = entity.Status.String()
	if err != nil {
		return nil, fmt.Errorf("failed to convert operation entity to response: %w", err)
	}

	apiOperation.ExecutionHistory = fromOperationEventsToResponse(entity.ExecutionHistory)

	if len(entity.Input) > 0 {
		var inputMap map[string]interface{} = make(map[string]interface{})
		err = json.Unmarshal(entity.Input, &inputMap)
		if err != nil {
			return nil, fmt.Errorf("failed to convert input to struct: %w", err)
		}

		apiOperation.Input, err = _struct.NewStruct(inputMap)
		if err != nil {
			return nil, fmt.Errorf("failed to convert input to response struct: %w", err)
		}
	}

	if entity.Lease != nil {
		apiOperation.Lease = &api.Lease{Ttl: entity.Lease.Ttl.UTC().Format(time.RFC3339)}
	}

	return apiOperation, nil
}

func fromOperationEventsToResponse(entities []operation.OperationEvent) []*api.OperationEvent {
	apiOperationEvents := make([]*api.OperationEvent, 0, len(entities))
	for _, entity := range entities {
		apiOperationEvents = append(apiOperationEvents, fromOperationEventToResponse(entity))
	}

	return apiOperationEvents
}

func fromOperationEventToResponse(entity operation.OperationEvent) *api.OperationEvent {
	return &api.OperationEvent{
		CreatedAt: timestamppb.New(entity.CreatedAt),
		Event:     entity.Event,
	}
}
