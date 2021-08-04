package handlers

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

type SchedulerHandler struct {
	schedulerManager *scheduler_manager.SchedulerManager
	api.UnimplementedSchedulersServer
}

func ProvideSchedulerHandler(schedulerManager *scheduler_manager.SchedulerManager) *SchedulerHandler {
	return &SchedulerHandler{
		schedulerManager: schedulerManager,
	}
}

func (h *SchedulerHandler) ListSchedulers(ctx context.Context, message *api.EmptyRequest) (*api.ListSchedulersReply, error) {

	entities, err := h.schedulerManager.GetAllSchedulers(ctx)
	if err != nil {
		return nil, err
	}

	schedulers := make([]*api.Scheduler, len(entities))
	for i, entity := range entities {
		scheduler := api.Scheduler{
			Name:  entity.Name,
			Game:  entity.Game,
			State: entity.State,
			PortRange: &api.PortRange{
				Start: entity.PortRange.Start,
				End:   entity.PortRange.End,
			},
		}
		schedulers[i] = &scheduler
	}

	return &api.ListSchedulersReply{
		Schedulers: schedulers,
	}, nil

}

func (h *SchedulerHandler) CreateScheduler(ctx context.Context, request *api.CreateSchedulerRequest) (*api.Scheduler, error) {

	scheduler := h.fromRequestToEntity(request)

	scheduler, err := h.schedulerManager.CreateScheduler(ctx, scheduler)
	if err != nil {
		return nil, fmt.Errorf("failed create scheduler: %w", err)
	}

	return h.fromEntityToResponse(scheduler), nil
}

func (h *SchedulerHandler) fromRequestToEntity(request *api.CreateSchedulerRequest) *entities.Scheduler {
	return &entities.Scheduler{
		Name:  request.Name,
		Game:  request.Game,
		State: entities.StateCreating,
		Spec: game_room.Spec{
			Version: request.Version,
		},
	}
}

func (h *SchedulerHandler) fromEntityToResponse(entity *entities.Scheduler) *api.Scheduler {
	return &api.Scheduler{
		Name:    entity.Name,
		Game:    entity.Game,
		State:   entity.State,
		Version: entity.Spec.Version,
	}
}
