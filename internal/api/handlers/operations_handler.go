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

package handlers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	api "github.com/topfreegames/maestro/pkg/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OperationsHandler struct {
	operationManager ports.OperationManager
	api.UnimplementedOperationsServiceServer
}

func ProvideOperationsHandler(operationManager ports.OperationManager) *OperationsHandler {
	return &OperationsHandler{
		operationManager: operationManager,
	}
}

func (h *OperationsHandler) ListOperations(ctx context.Context, request *api.ListOperationsRequest) (*api.ListOperationsResponse, error) {
	sortingOrder, err := extractSortingParameters(request.OrderBy)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	pendingOperationEntities, err := h.operationManager.ListSchedulerPendingOperations(ctx, request.GetSchedulerName())
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}
	sortOperationsByCreatedAt(pendingOperationEntities, sortingOrder)

	pendingOperationResponse, err := h.fromOperationsToResponses(pendingOperationEntities)
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	activeOperationEntities, err := h.operationManager.ListSchedulerActiveOperations(ctx, request.GetSchedulerName())
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}
	sortOperationsByCreatedAt(activeOperationEntities, sortingOrder)

	activeOperationResponses, err := h.fromOperationsToResponses(activeOperationEntities)
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	finishedOperationEntities, err := h.operationManager.ListSchedulerFinishedOperations(ctx, request.GetSchedulerName())
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}
	sortOperationsByCreatedAt(finishedOperationEntities, sortingOrder)

	finishedOperationResponse, err := h.fromOperationsToResponses(finishedOperationEntities)
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	return &api.ListOperationsResponse{
		PendingOperations:  pendingOperationResponse,
		ActiveOperations:   activeOperationResponses,
		FinishedOperations: finishedOperationResponse,
	}, nil
}

func (h *OperationsHandler) CancelOperation(ctx context.Context, request *api.CancelOperationRequest) (*api.CancelOperationResponse, error) {
	err := h.operationManager.EnqueueOperationCancellationRequest(ctx, request.SchedulerName, request.OperationId)
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	return &api.CancelOperationResponse{}, nil
}

func (h *OperationsHandler) fromOperationsToResponses(entities []*operation.Operation) ([]*api.Operation, error) {
	responses := make([]*api.Operation, len(entities))
	for i, entity := range entities {
		response, err := h.fromOperationToResponse(entity)
		if err != nil {
			return nil, err
		}
		responses[i] = response
	}

	return responses, nil
}

func (h *OperationsHandler) fromOperationToResponse(entity *operation.Operation) (*api.Operation, error) {
	status, err := entity.Status.String()
	if err != nil {
		return nil, fmt.Errorf("failed to convert operation entity to response: %w", err)
	}
	if entity.Lease != nil {
		return &api.Operation{
			Id:             entity.ID,
			Status:         status,
			DefinitionName: entity.DefinitionName,
			SchedulerName:  entity.SchedulerName,
			Lease:          &api.Lease{Ttl: entity.Lease.Ttl.UTC().Format(time.RFC3339)},
			CreatedAt:      timestamppb.New(entity.CreatedAt),
		}, nil
	} else {
		return &api.Operation{
			Id:             entity.ID,
			Status:         status,
			DefinitionName: entity.DefinitionName,
			SchedulerName:  entity.SchedulerName,
			CreatedAt:      timestamppb.New(entity.CreatedAt),
		}, nil
	}
}

func sortOperationsByCreatedAt(operations []*operation.Operation, order string) {
	sort.Slice(operations, func(i, j int) bool {
		if order == "asc" {
			return operations[i].CreatedAt.Before(operations[j].CreatedAt)
		} else {
			return operations[i].CreatedAt.After(operations[j].CreatedAt)
		}
	})
}

func extractSortingParameters(orderBy string) (string, error) {
	sortingOrder := "desc"
	sortingField := "createdAt"
	sortingParameters := strings.Fields(orderBy)
	parametersLen := len(sortingParameters)
	switch {
	case parametersLen == 1:
		sortingField = sortingParameters[0]
	case parametersLen == 2:
		sortingField = sortingParameters[0]
		sortingOrder = sortingParameters[1]
	case parametersLen > 2:
		return "", fmt.Errorf("invalid sorting parameters number")
	}

	if sortingField != "createdAt" {
		return "", fmt.Errorf("invalid sorting field: %s", sortingField)
	}
	if sortingOrder != "asc" && sortingOrder != "desc" {
		return "", fmt.Errorf("invalid sorting order: %s", sortingOrder)
	}
	return sortingOrder, nil
}
