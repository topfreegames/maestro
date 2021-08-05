package handlers

import (
	"context"
	"errors"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	portsErrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"
	api "github.com/topfreegames/maestro/pkg/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SchedulerHandler struct {
	schedulerManager *scheduler_manager.SchedulerManager
	operationManager *operation_manager.OperationManager
	api.UnimplementedSchedulersServer
}

func ProvideSchedulerHandler(schedulerManager *scheduler_manager.SchedulerManager, operationManager *operation_manager.OperationManager) *SchedulerHandler {
	return &SchedulerHandler{
		schedulerManager: schedulerManager,
		operationManager: operationManager,
	}
}

func (h *SchedulerHandler) ListSchedulers(ctx context.Context, message *api.EmptyRequest) (*api.ListSchedulersReply, error) {

	entities, err := h.schedulerManager.GetAllSchedulers(ctx)
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
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

func (h *SchedulerHandler) ListOperations(ctx context.Context, request *api.ListOperationsRequest) (*api.ListOperationsReply, error) {

	pendingOperationEntities, err := h.operationManager.ListSchedulerPendingOperations(ctx, request.GetScheduler())
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}
	pendingOperationResponse, err := h.fromOperationsToResponses(pendingOperationEntities)
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	activeOperationEntities, err := h.operationManager.ListSchedulerActiveOperations(ctx, request.GetScheduler())
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}
	activeOperationResponses, err := h.fromOperationsToResponses(activeOperationEntities)
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	finishedOperationEntities, err := h.operationManager.ListSchedulerFinishedOperations(ctx, request.GetScheduler())
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}
	finishedOperationResponse, err := h.fromOperationsToResponses(finishedOperationEntities)
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	return &api.ListOperationsReply{
		PendingOperations:  pendingOperationResponse,
		ActiveOperations:   activeOperationResponses,
		FinishedOperations: finishedOperationResponse,
	}, nil

}

func (h *SchedulerHandler) CreateScheduler(ctx context.Context, request *api.CreateSchedulerRequest) (*api.Scheduler, error) {

	scheduler := h.fromRequestToEntity(request)

	scheduler, err := h.schedulerManager.CreateScheduler(ctx, scheduler)
	if errors.Is(err, portsErrors.ErrAlreadyExists) {
		return nil, status.Error(codes.AlreadyExists, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
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

func (h *SchedulerHandler) fromOperationsToResponses(entities []*operation.Operation) ([]*api.Operation, error) {
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

func (h *SchedulerHandler) fromOperationToResponse(entity *operation.Operation) (*api.Operation, error) {
	status, err := entity.Status.String()
	if err != nil {
		return nil, fmt.Errorf("failed to convert operation entity to response: %w", err)
	}

	return &api.Operation{
		Id:             entity.ID,
		Status:         status,
		DefinitionName: entity.DefinitionName,
		SchedulerName:  entity.SchedulerName,
	}, nil
}
