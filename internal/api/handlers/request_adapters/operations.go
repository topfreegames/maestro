package request_adapters

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
	}
	if err != nil {
		return nil, fmt.Errorf("failed to convert input to response struct: %w", err)
	}

	if entity.Lease != nil {
		apiOperation.Lease = &api.Lease{Ttl: entity.Lease.Ttl.UTC().Format(time.RFC3339)}
		return apiOperation, nil
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
