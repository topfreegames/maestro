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
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/topfreegames/maestro/internal/api/handlers/requestadapters"
	portsErrors "github.com/topfreegames/maestro/internal/core/ports/errors"

	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/ports"
	"go.uber.org/zap"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	api "github.com/topfreegames/maestro/pkg/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OperationsHandler struct {
	operationManager ports.OperationManager
	logger           *zap.Logger
	api.UnimplementedOperationsServiceServer
}

func ProvideOperationsHandler(operationManager ports.OperationManager) *OperationsHandler {
	return &OperationsHandler{
		operationManager: operationManager,
		logger: zap.L().
			With(zap.String(logs.LogFieldComponent, "handler"), zap.String(logs.LogFieldHandlerName, "operations_handler")),
	}
}

func (h *OperationsHandler) ListOperations(ctx context.Context, request *api.ListOperationsRequest) (*api.ListOperationsResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, request.GetSchedulerName()))
	sortingOrder, err := extractSortingParameters(request.OrderBy)
	if err != nil {
		handlerLogger.Error(fmt.Sprintf("error parsing sorting parameters, orderBy: %+v", request.OrderBy), zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	pendingOperationEntities, err := h.operationManager.ListSchedulerPendingOperations(ctx, request.GetSchedulerName())
	if err != nil {
		handlerLogger.Error("error listing pending operations", zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}
	sortOperationsByCreatedAt(pendingOperationEntities, sortingOrder)

	pendingOperationResponse, err := requestadapters.FromOperationsToResponses(pendingOperationEntities)
	if err != nil {
		handlerLogger.Error("error converting pending operations", zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}

	activeOperationEntities, err := h.operationManager.ListSchedulerActiveOperations(ctx, request.GetSchedulerName())
	if err != nil {
		handlerLogger.Error("error listing active operations", zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}
	sortOperationsByCreatedAt(activeOperationEntities, sortingOrder)

	activeOperationResponses, err := requestadapters.FromOperationsToResponses(activeOperationEntities)
	if err != nil {
		handlerLogger.Error("error converting active operations", zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}

	finishedOperationEntities, err := h.operationManager.ListSchedulerFinishedOperations(ctx, request.GetSchedulerName())
	if err != nil {
		handlerLogger.Error("error listing finished operations", zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}
	sortOperationsByCreatedAt(finishedOperationEntities, sortingOrder)

	finishedOperationResponse, err := requestadapters.FromOperationsToResponses(finishedOperationEntities)
	if err != nil {
		handlerLogger.Error("error converting finished operations", zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}

	return &api.ListOperationsResponse{
		PendingOperations:  pendingOperationResponse,
		ActiveOperations:   activeOperationResponses,
		FinishedOperations: finishedOperationResponse,
	}, nil
}

func (h *OperationsHandler) CancelOperation(ctx context.Context, request *api.CancelOperationRequest) (*api.CancelOperationResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, request.GetSchedulerName()), zap.String(logs.LogFieldOperationID, request.GetOperationId()))
	handlerLogger.Info("received request to cancel operation")
	err := h.operationManager.EnqueueOperationCancellationRequest(ctx, request.SchedulerName, request.OperationId)
	if err != nil {
		if errors.Is(err, portsErrors.ErrConflict) {
			handlerLogger.Warn("Cancel operation conflict", zap.String(logs.LogFieldSchedulerName, request.GetSchedulerName()), zap.String(logs.LogFieldOperationID, request.GetOperationId()), zap.Error(err))
			return nil, status.Error(codes.Aborted, err.Error())
		}
		handlerLogger.Error("error cancelling operation", zap.String(logs.LogFieldSchedulerName, request.GetSchedulerName()), zap.String(logs.LogFieldOperationID, request.GetOperationId()), zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}

	return &api.CancelOperationResponse{}, nil
}

func (h *OperationsHandler) GetOperation(ctx context.Context, request *api.GetOperationRequest) (*api.GetOperationResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, request.GetSchedulerName()), zap.String(logs.LogFieldOperationID, request.GetOperationId()))
	handlerLogger.Info("received request to get operation by id")
	op, _, err := h.operationManager.GetOperation(ctx, request.GetSchedulerName(), request.GetOperationId())
	if err != nil {
		if errors.Is(err, portsErrors.ErrNotFound) {
			handlerLogger.Warn("operation not found", zap.Error(err))
			return nil, status.Error(codes.NotFound, err.Error())
		}
		handlerLogger.Error("error fetching operation", zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}

	convertedOp, err := requestadapters.FromOperationToResponse(op)
	if err != nil {
		handlerLogger.Error("invalid operation object. Fail to convert", zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}
	return &api.GetOperationResponse{Operation: convertedOp}, nil
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
