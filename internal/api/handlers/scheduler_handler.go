package handlers

import (
	"context"

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
