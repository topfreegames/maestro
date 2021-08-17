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

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	portsErrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"
	api "github.com/topfreegames/maestro/pkg/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type SchedulersHandler struct {
	schedulerManager *scheduler_manager.SchedulerManager
	api.UnimplementedSchedulersServiceServer
}

func ProvideSchedulersHandler(schedulerManager *scheduler_manager.SchedulerManager) *SchedulersHandler {
	return &SchedulersHandler{
		schedulerManager: schedulerManager,
	}
}

func (h *SchedulersHandler) ListSchedulers(ctx context.Context, message *api.ListSchedulersRequest) (*api.ListSchedulersResponse, error) {
	entities, err := h.schedulerManager.GetAllSchedulers(ctx)
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	schedulers := make([]*api.Scheduler, len(entities))
	for i, entity := range entities {
		schedulers[i] = h.fromEntityToResponse(entity)
	}

	return &api.ListSchedulersResponse{
		Schedulers: schedulers,
	}, nil
}

func (h *SchedulersHandler) CreateScheduler(ctx context.Context, request *api.CreateSchedulerRequest) (*api.CreateSchedulerResponse, error) {
	scheduler := h.fromRequestToEntity(request)

	scheduler, err := h.schedulerManager.CreateScheduler(ctx, scheduler)
	if errors.Is(err, portsErrors.ErrAlreadyExists) {
		return nil, status.Error(codes.AlreadyExists, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	return &api.CreateSchedulerResponse{
		Scheduler: h.fromEntityToResponse(scheduler),
	}, nil
}

func (h *SchedulersHandler) fromRequestToEntity(request *api.CreateSchedulerRequest) *entities.Scheduler {
	return &entities.Scheduler{
		Name:  request.Name,
		Game:  request.Game,
		State: entities.StateCreating,
		Spec: game_room.Spec{
			Version: request.Version,
		},
	}
}

func (h *SchedulersHandler) fromEntityToResponse(entity *entities.Scheduler) *api.Scheduler {
	return &api.Scheduler{
		Name:      entity.Name,
		Game:      entity.Game,
		State:     entity.State,
		Version:   entity.Spec.Version,
		PortRange: getPortRange(entity.PortRange),
		CreatedAt: timestamppb.New(entity.CreatedAt),
	}
}

func getPortRange(portRange *entities.PortRange) *api.PortRange {
	if portRange != nil {
		return &api.PortRange{
			Start: portRange.Start,
			End:   portRange.End,
		}
	}

	return nil
}
