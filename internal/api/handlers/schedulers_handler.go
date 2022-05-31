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
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/topfreegames/maestro/internal/api/handlers/requestadapters"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"

	"go.uber.org/zap"

	"github.com/topfreegames/maestro/internal/core/filters"

	"github.com/topfreegames/maestro/internal/core/entities"
	portsErrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	api "github.com/topfreegames/maestro/pkg/api/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SchedulersHandler struct {
	schedulerManager *scheduler_manager.SchedulerManager
	logger           *zap.Logger
	api.UnimplementedSchedulersServiceServer
}

func ProvideSchedulersHandler(schedulerManager *scheduler_manager.SchedulerManager) *SchedulersHandler {
	return &SchedulersHandler{
		schedulerManager: schedulerManager,
		logger: zap.L().
			With(zap.String(logs.LogFieldComponent, "handler"), zap.String(logs.LogFieldHandlerName, "schedulers_handler")),
	}
}

func (h *SchedulersHandler) ListSchedulers(ctx context.Context, message *api.ListSchedulersRequest) (*api.ListSchedulersResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldGame, message.GetGame()), zap.String(logs.LogFieldSchedulerName, message.GetName()))
	handlerLogger.Info("handling list schedulers request")
	schedulerFilter := &filters.SchedulerFilter{
		Name:    message.GetName(),
		Game:    message.GetGame(),
		Version: message.GetVersion(),
	}
	schedulerEntities, err := h.schedulerManager.GetSchedulersWithFilter(ctx, schedulerFilter)
	if err != nil {
		handlerLogger.Error("error getting schedulers using the provided filter", zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}

	schedulers := make([]*api.SchedulerWithoutSpec, len(schedulerEntities))
	for i, entity := range schedulerEntities {
		schedulers[i] = requestadapters.FromEntitySchedulerToListResponse(entity)
	}

	handlerLogger.Info("finish handling list schedulers request")

	return &api.ListSchedulersResponse{Schedulers: schedulers}, nil
}

func (h *SchedulersHandler) GetScheduler(ctx context.Context, request *api.GetSchedulerRequest) (*api.GetSchedulerResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, request.GetSchedulerName()))
	handlerLogger.Info(fmt.Sprintf("handling get scheduler request, scheduler version: %s", request.GetVersion()))
	var scheduler *entities.Scheduler
	var err error

	schedulerName := request.GetSchedulerName()
	queryVersion := request.GetVersion()
	if queryVersion != "" {
		scheduler, err = h.schedulerManager.GetScheduler(ctx, schedulerName, queryVersion)
	} else {
		scheduler, err = h.schedulerManager.GetActiveScheduler(ctx, schedulerName)
	}

	if err != nil {
		handlerLogger.Error("error getting scheduler", zap.Error(err))
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Unknown, err.Error())
	}
	handlerLogger.Info("finish handling get scheduler request")

	returnScheduler, err := requestadapters.FromEntitySchedulerToResponse(scheduler)
	if err != nil {
		h.logger.Error("error parsing scheduler to response", zap.Any("schedulerName", schedulerName), zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}

	return &api.GetSchedulerResponse{Scheduler: returnScheduler}, nil
}

func (h *SchedulersHandler) GetSchedulerVersions(ctx context.Context, request *api.GetSchedulerVersionsRequest) (*api.GetSchedulerVersionsResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, request.GetSchedulerName()))
	handlerLogger.Info("handling get scheduler versions request")
	versions, err := h.schedulerManager.GetSchedulerVersions(ctx, request.GetSchedulerName())

	if err != nil {
		handlerLogger.Error("error getting scheduler versions", zap.Error(err))
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Unknown, err.Error())
	}
	handlerLogger.Info("finish handling get scheduler versions request")

	return &api.GetSchedulerVersionsResponse{Versions: requestadapters.FromEntitySchedulerVersionListToResponse(versions)}, nil
}

func (h *SchedulersHandler) CreateScheduler(ctx context.Context, request *api.CreateSchedulerRequest) (*api.CreateSchedulerResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, request.GetName()), zap.String(logs.LogFieldGame, request.GetGame()))
	handlerLogger.Info("handling create scheduler request")
	scheduler, err := requestadapters.FromApiCreateSchedulerRequestToEntity(request)
	if err != nil {
		apiValidationError := parseValidationError(err.(validator.ValidationErrors))
		handlerLogger.Error("error parsing scheduler", zap.Error(apiValidationError))
		return nil, status.Error(codes.InvalidArgument, apiValidationError.Error())
	}

	scheduler, err = h.schedulerManager.CreateScheduler(ctx, scheduler)
	if err != nil {
		handlerLogger.Error("error creating scheduler", zap.Error(err))
		if errors.Is(err, portsErrors.ErrAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Error(codes.Unknown, err.Error())
	}
	handlerLogger.Info("finish handling create scheduler request")

	returnScheduler, err := requestadapters.FromEntitySchedulerToResponse(scheduler)
	if err != nil {
		h.logger.Error("error parsing scheduler to response", zap.Any("schedulerName", request.GetName()), zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}
	return &api.CreateSchedulerResponse{Scheduler: returnScheduler}, nil
}

func (h *SchedulersHandler) AddRooms(ctx context.Context, request *api.AddRoomsRequest) (*api.AddRoomsResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, request.GetSchedulerName()))
	handlerLogger.Info("handling add rooms request")
	operation, err := h.schedulerManager.AddRooms(ctx, request.GetSchedulerName(), request.GetAmount())

	if err != nil {
		handlerLogger.Error(fmt.Sprintf("error adding rooms to scheduler, amount: %d", request.GetAmount()), zap.Error(err))
		if errors.Is(err, portsErrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Unknown, err.Error())
	}
	handlerLogger.Info("finish handling add rooms request")

	return &api.AddRoomsResponse{OperationId: operation.ID}, nil
}

func (h *SchedulersHandler) RemoveRooms(ctx context.Context, request *api.RemoveRoomsRequest) (*api.RemoveRoomsResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, request.GetSchedulerName()))
	handlerLogger.Info("handling remove rooms request")
	operation, err := h.schedulerManager.RemoveRooms(ctx, request.GetSchedulerName(), int(request.GetAmount()))

	if err != nil {
		handlerLogger.Error("error removing rooms from scheduler", zap.Error(err))
		if errors.Is(err, portsErrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Unknown, err.Error())
	}

	handlerLogger.Info("finish handling remove rooms request")
	return &api.RemoveRoomsResponse{OperationId: operation.ID}, nil
}

func (h *SchedulersHandler) NewSchedulerVersion(ctx context.Context, request *api.NewSchedulerVersionRequest) (*api.NewSchedulerVersionResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, request.GetName()), zap.String(logs.LogFieldGame, request.GetGame()))
	handlerLogger.Info("handling new scheduler version request")
	scheduler, err := requestadapters.FromApiNewSchedulerVersionRequestToEntity(request)
	if err != nil {
		apiValidationError := parseValidationError(err.(validator.ValidationErrors))
		handlerLogger.Error("error parsing scheduler version", zap.Error(apiValidationError))
		return nil, status.Error(codes.InvalidArgument, apiValidationError.Error())
	}

	operation, err := h.schedulerManager.EnqueueNewSchedulerVersionOperation(ctx, scheduler)

	if err != nil {
		handlerLogger.Error("error creating new scheduler version", zap.Error(err))
		if errors.Is(err, portsErrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Unknown, err.Error())
	}
	handlerLogger.Info("finish handling new scheduler version request")

	return &api.NewSchedulerVersionResponse{OperationId: operation.ID}, nil
}

func (h *SchedulersHandler) PatchScheduler(ctx context.Context, request *api.PatchSchedulerRequest) (*api.PatchSchedulerResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, request.GetName()))
	handlerLogger.Info("handling patch scheduler request")

	patchMap, err := requestadapters.FromApiPatchSchedulerRequestToChangeMap(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if len(patchMap) == 0 {
		return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("no change found to scheduler %s", request.GetName()))
	}

	operation, err := h.schedulerManager.PatchSchedulerAndCreateNewSchedulerVersionOperation(ctx, request.GetName(), patchMap)

	if err != nil {
		handlerLogger.Error("error patching scheduler", zap.Error(err))
		if errors.Is(err, portsErrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, portsErrors.ErrInvalidArgument) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		return nil, status.Error(codes.Unknown, err.Error())
	}
	handlerLogger.Info("finish handling patch scheduler request")

	return &api.PatchSchedulerResponse{OperationId: operation.ID}, nil
}

func (h *SchedulersHandler) SwitchActiveVersion(ctx context.Context, request *api.SwitchActiveVersionRequest) (*api.SwitchActiveVersionResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, request.GetSchedulerName()))
	handlerLogger.Info("handling switch active version request")
	operation, err := h.schedulerManager.EnqueueSwitchActiveVersionOperation(ctx, request.GetSchedulerName(), request.GetVersion())

	if err != nil {
		handlerLogger.Error(fmt.Sprintf("error switching active version %s", request.GetVersion()), zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}

	handlerLogger.Info("finish handling switch active version request")
	return &api.SwitchActiveVersionResponse{OperationId: operation.ID}, nil
}

func (h *SchedulersHandler) GetSchedulersInfo(ctx context.Context, request *api.GetSchedulersInfoRequest) (*api.GetSchedulersInfoResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldGame, request.GetGame()))
	handlerLogger.Info("handling get schedulers info request")
	filter := filters.SchedulerFilter{Game: request.GetGame()}
	schedulers, err := h.schedulerManager.GetSchedulersInfo(ctx, &filter)

	if err != nil {
		handlerLogger.Error("error getting schedulers info", zap.Error(err))
		if errors.Is(err, portsErrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Unknown, err.Error())
	}

	schedulersResponse := make([]*api.SchedulerInfo, len(schedulers))
	for i, scheduler := range schedulers {
		schedulersResponse[i] = requestadapters.FromEntitySchedulerInfoToListResponse(scheduler)
	}
	handlerLogger.Info("finish handling get schedulers info request")

	return &api.GetSchedulersInfoResponse{
		Schedulers: schedulersResponse,
	}, nil
}
