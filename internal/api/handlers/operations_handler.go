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
	page, pageSize, err := extractPaginationParameters(request)
	if err != nil {
		handlerLogger.Error(fmt.Sprintf("error parsing pagination parameters"), zap.Error(err))
		return nil, err
	}
	operationStage := request.Stage

	var operations []*operation.Operation
	var total uint32

	switch operationStage {
	case "pending":
		operations, err = h.queryPendingOperations(ctx, request.SchedulerName, sortingOrder)
		if err != nil {
			return nil, status.Error(codes.Unknown, "error listing operations on pending stage")
		}
		total = uint32(len(operations))
		pageSize = total

	case "active":
		operations, err = h.queryActiveOperations(ctx, request.SchedulerName, sortingOrder)
		if err != nil {
			return nil, status.Error(codes.Unknown, "error listing operations on active stage")
		}
		total = uint32(len(operations))
		pageSize = total

	case "final":
		operations, total, err = h.queryFinishedOperations(ctx, request.SchedulerName, sortingOrder, int64(page), int64(pageSize))
		if err != nil {
			return nil, status.Error(codes.Unknown, "error listing operations on final stage")
		}

	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid stage filter: %s", request.Stage)
	}

	responseOperations, err := h.parseListOperationsResponse(ctx, operations)
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}
	return &api.ListOperationsResponse{Operations: responseOperations, Total: &total, Page: &page, PageSize: &pageSize}, nil
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

func (h *OperationsHandler) parseListOperationsResponse(ctx context.Context, operationEntities []*operation.Operation) ([]*api.ListOperationItem, error) {
	operationResponse, err := requestadapters.FromOperationsToListOperationsResponses(operationEntities)
	if err != nil {
		h.logger.Error("error parsing operations", zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}
	return operationResponse, nil
}

func (h *OperationsHandler) queryPendingOperations(ctx context.Context, schedulerName, sortingOrder string) ([]*operation.Operation, error) {
	pendingOperationEntities, err := h.operationManager.ListSchedulerPendingOperations(ctx, schedulerName)
	if err != nil {
		h.logger.Error("error listing pending operations", zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}
	sortOperationsByCreatedAt(pendingOperationEntities, sortingOrder)

	return pendingOperationEntities, nil
}

func (h *OperationsHandler) queryActiveOperations(ctx context.Context, schedulerName, sortingOrder string) ([]*operation.Operation, error) {
	activeOperationEntities, err := h.operationManager.ListSchedulerActiveOperations(ctx, schedulerName)
	if err != nil {
		h.logger.Error("error listing active operations", zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}
	sortOperationsByCreatedAt(activeOperationEntities, sortingOrder)

	return activeOperationEntities, nil
}

func (h *OperationsHandler) queryFinishedOperations(ctx context.Context, schedulerName, sortingOrder string, page, pageSize int64) ([]*operation.Operation, uint32, error) {
	finishedOperationEntities, total, err := h.operationManager.ListSchedulerFinishedOperations(ctx, schedulerName, page, pageSize)
	if err != nil {
		h.logger.Error("error listing finished operations", zap.Error(err))
		return nil, uint32(0), status.Error(codes.Unknown, err.Error())
	}
	sortOperationsByCreatedAt(finishedOperationEntities, sortingOrder)

	return finishedOperationEntities, uint32(total), nil
}

func extractPaginationParameters(request *api.ListOperationsRequest) (uint32, uint32, error) {
	page := request.GetPage()
	pageSize := request.GetPerPage()
	operationStage := request.Stage

	if pageSize == 0 {
		pageSize = 15
	}

	if operationStage != "final" && (request.Page != nil || request.PerPage != nil) {
		return 0, 0, status.Errorf(codes.InvalidArgument, "there is no pagination filter implemented for %s stage", operationStage)
	}

	return page, pageSize, nil
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
