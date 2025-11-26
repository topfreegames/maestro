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

package schedulers

import (
	"context"
	"errors"
	"fmt"
	"slices"

	newversion "github.com/topfreegames/maestro/internal/core/operations/schedulers/newversion"
	"github.com/topfreegames/maestro/internal/core/operations/schedulers/switchversion"
	"github.com/topfreegames/maestro/internal/core/services/schedulers/patch"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/filters"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/operations/schedulers/create"
	"github.com/topfreegames/maestro/internal/core/operations/schedulers/delete"
	"github.com/topfreegames/maestro/internal/core/ports"
	portsErrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	"go.uber.org/zap"
)

type SchedulerManager struct {
	schedulerStorage ports.SchedulerStorage
	schedulerCache   ports.SchedulerCache
	operationManager ports.OperationManager
	roomStorage      ports.RoomStorage
	logger           *zap.Logger
}

var _ ports.SchedulerManager = (*SchedulerManager)(nil)

func NewSchedulerManager(schedulerStorage ports.SchedulerStorage, schedulerCache ports.SchedulerCache, operationManager ports.OperationManager, roomStorage ports.RoomStorage) *SchedulerManager {
	return &SchedulerManager{
		schedulerStorage: schedulerStorage,
		operationManager: operationManager,
		schedulerCache:   schedulerCache,
		roomStorage:      roomStorage,
		logger:           zap.L().With(zap.String(logs.LogFieldComponent, "service"), zap.String(logs.LogFieldServiceName, "scheduler_manager")),
	}
}

func (s *SchedulerManager) GetActiveScheduler(ctx context.Context, schedulerName string) (*entities.Scheduler, error) {
	activeScheduler, err := s.getScheduler(ctx, schedulerName)
	if err != nil {
		return nil, err
	}
	return activeScheduler, nil
}

func (s *SchedulerManager) GetSchedulerByVersion(ctx context.Context, schedulerName, schedulerVersion string) (*entities.Scheduler, error) {
	activeScheduler, err := s.schedulerStorage.GetSchedulerWithFilter(ctx, &filters.SchedulerFilter{
		Name:    schedulerName,
		Version: schedulerVersion,
	})
	if err != nil {
		s.logger.Error("error fetching scheduler by version", zap.Error(err))
		return nil, err
	}
	return activeScheduler, nil
}

func (s *SchedulerManager) CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) (*entities.Scheduler, error) {
	err := scheduler.Validate()
	if err != nil {
		return nil, fmt.Errorf("failing in creating schedule: %w", err)
	}

	err = s.schedulerStorage.CreateScheduler(ctx, scheduler)
	if err != nil {
		return nil, err
	}

	op, err := s.operationManager.CreateOperation(ctx, scheduler.Name, &create.Definition{NewScheduler: scheduler})
	if err != nil {
		return nil, fmt.Errorf("failing in creating the operation: %s: %s", create.OperationName, err)
	}

	s.logger.Info("scheduler enqueued to be created", zap.String("scheduler", scheduler.Name), zap.String("operation", op.ID))

	return s.schedulerStorage.GetScheduler(ctx, scheduler.Name)
}

func (s *SchedulerManager) CreateNewSchedulerVersion(ctx context.Context, scheduler *entities.Scheduler) error {
	err := scheduler.Validate()
	if err != nil {
		return fmt.Errorf("failing in creating schedule: %w", err)
	}

	err = s.schedulerStorage.CreateSchedulerVersion(ctx, "", scheduler)
	if err != nil {
		return err
	}
	return nil
}

func (s *SchedulerManager) CreateNewSchedulerVersionAndEnqueueSwitchVersion(ctx context.Context, scheduler *entities.Scheduler) (opID string, err error) {
	err = scheduler.Validate()
	if err != nil {
		return "", fmt.Errorf("failing in creating schedule: %w", err)
	}

	err = s.schedulerStorage.RunWithTransaction(ctx, func(transactionId ports.TransactionID) error {
		err := s.schedulerStorage.CreateSchedulerVersion(ctx, transactionId, scheduler)
		if err != nil {
			return err
		}

		// Conflict checks are not performed here because this method runs only inside
		// the operation executor. Checking for conflicts at this point would cause the
		// operation to block itself. All conflict validation is handled earlier, when
		// operations are enqueued at the API level.
		opDef := &switchversion.Definition{NewActiveVersion: scheduler.Spec.Version}
		op, err := s.operationManager.CreateOperation(ctx, scheduler.Name, opDef)
		if err != nil {
			return fmt.Errorf("error enqueuing switch active version operation: %w", err)
		}
		opID = op.ID
		return nil

	})
	if err != nil {
		return "", err
	}
	return opID, nil
}

func (s *SchedulerManager) PatchSchedulerAndCreateNewSchedulerVersionOperation(ctx context.Context, schedulerName string, patchMap map[string]interface{}) (*operation.Operation, error) {
	ongoing, err := s.hasOngoingVersionOperation(ctx, schedulerName)
	if err != nil {
		return nil, portsErrors.NewErrUnexpected("failed to check for ongoing version operations: %s", err.Error())
	}
	if ongoing != "" {
		return nil, portsErrors.NewErrConflict("cannot patch scheduler: there is already an ongoing version operation (id: %s) for scheduler %s", ongoing, schedulerName)
	}

	scheduler, err := s.schedulerStorage.GetScheduler(ctx, schedulerName)
	if err != nil {
		if errors.Is(err, portsErrors.ErrNotFound) {
			return nil, portsErrors.NewErrNotFound("no scheduler found, can not create new version for inexistent scheduler: %s", err.Error())
		}

		return nil, portsErrors.NewErrUnexpected("unexpected error getting scheduler to patch: %s", err.Error())
	}

	scheduler, err = patch.PatchScheduler(*scheduler, patchMap)
	if err != nil {
		return nil, portsErrors.NewErrInvalidArgument("error patching scheduler: %s", err.Error())
	}

	if err := scheduler.Validate(); err != nil {
		return nil, portsErrors.NewErrInvalidArgument("invalid patched scheduler: %s", err.Error())
	}

	opDef := &newversion.Definition{NewScheduler: scheduler}

	op, err := s.operationManager.CreateOperation(ctx, scheduler.Name, opDef)
	if err != nil {
		return nil, portsErrors.NewErrUnexpected("failed to schedule %s operation: %s", opDef.Name(), err.Error())
	}

	return op, nil
}

func (s *SchedulerManager) GetSchedulersWithFilter(ctx context.Context, schedulerFilter *filters.SchedulerFilter) ([]*entities.Scheduler, error) {
	return s.schedulerStorage.GetSchedulersWithFilter(ctx, schedulerFilter)
}

func (s *SchedulerManager) GetScheduler(ctx context.Context, schedulerName, version string) (*entities.Scheduler, error) {
	return s.schedulerStorage.GetSchedulerWithFilter(ctx, &filters.SchedulerFilter{
		Name:    schedulerName,
		Version: version,
	})
}

func (s *SchedulerManager) GetSchedulerVersions(ctx context.Context, schedulerName string) ([]*entities.SchedulerVersion, error) {
	return s.schedulerStorage.GetSchedulerVersions(ctx, schedulerName)
}

func (s *SchedulerManager) EnqueueNewSchedulerVersionOperation(ctx context.Context, scheduler *entities.Scheduler) (*operation.Operation, error) {
	ongoing, err := s.hasOngoingVersionOperation(ctx, scheduler.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check for ongoing version operations: %w", err)
	}
	if ongoing != "" {
		return nil, portsErrors.NewErrConflict("cannot create new scheduler version: there is already an ongoing version operation (id: %s) for scheduler %s", ongoing, scheduler.Name)
	}

	currentScheduler, err := s.schedulerStorage.GetScheduler(ctx, scheduler.Name)
	if err != nil {
		return nil, fmt.Errorf("no scheduler found, can not create new version for inexistent scheduler: %w", err)
	}

	scheduler.Spec.Version = currentScheduler.Spec.Version
	err = scheduler.Validate()
	if err != nil {
		return nil, err
	}

	opDef := &newversion.Definition{NewScheduler: scheduler}

	op, err := s.operationManager.CreateOperation(ctx, scheduler.Name, opDef)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule %s operation: %w", opDef.Name(), err)
	}

	return op, nil
}

func (s *SchedulerManager) EnqueueSwitchActiveVersionOperation(ctx context.Context, schedulerName, newVersion string) (*operation.Operation, error) {
	ongoing, err := s.hasOngoingVersionOperation(ctx, schedulerName)
	if err != nil {
		return nil, fmt.Errorf("failed to check for ongoing version operations: %w", err)
	}
	if ongoing != "" {
		return nil, portsErrors.NewErrConflict("cannot switch active version: there is already an ongoing version operation (id: %s) for scheduler %s", ongoing, schedulerName)
	}

	opDef := &switchversion.Definition{NewActiveVersion: newVersion}
	op, err := s.operationManager.CreateOperation(ctx, schedulerName, opDef)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule %s operation: %w", opDef.Name(), err)
	}

	return op, nil
}

func (s *SchedulerManager) EnqueueDeleteSchedulerOperation(ctx context.Context, schedulerName string) (*operation.Operation, error) {
	_, err := s.getScheduler(ctx, schedulerName)
	if err != nil {
		if errors.Is(err, portsErrors.ErrNotFound) {
			return nil, portsErrors.NewErrNotFound("no scheduler found, can not delete nonexistent scheduler: %s", err.Error())
		}

		return nil, portsErrors.NewErrUnexpected("unexpected error getting scheduler to delete: %s", err.Error())
	}
	opDef := &delete.Definition{}
	op, err := s.operationManager.CreateOperation(ctx, schedulerName, opDef)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule %s operation: %w", opDef.Name(), err)
	}

	return op, nil
}

func (s *SchedulerManager) UpdateScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	err := scheduler.Validate()
	if err != nil {
		return fmt.Errorf("failing in update scheduler: %w", err)
	}

	err = s.schedulerStorage.UpdateScheduler(ctx, scheduler)
	if err != nil {
		return fmt.Errorf("error switch scheduler active version to scheduler \"%s\", version \"%s\". error: %w", scheduler.Name, scheduler.Spec.Version, err)
	}

	err = s.schedulerCache.DeleteScheduler(ctx, scheduler.Name)
	if err != nil {
		s.logger.Error("error deleting scheduler from cache", zap.String("scheduler", scheduler.Name), zap.Error(err))
	}
	return nil
}

func (s *SchedulerManager) GetSchedulersInfo(ctx context.Context, filter *filters.SchedulerFilter) ([]*entities.SchedulerInfo, error) {
	schedulers, err := s.schedulerStorage.GetSchedulersWithFilter(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("no schedulers found: %w", err)
	}

	schedulersInfo := make([]*entities.SchedulerInfo, len(schedulers))
	for i, scheduler := range schedulers {
		schedulerInfo, err := s.newSchedulerInfo(ctx, scheduler)
		if err != nil {
			return nil, fmt.Errorf("couldn't get scheduler and game rooms information: %w", err)
		}
		schedulersInfo[i] = schedulerInfo
	}

	return schedulersInfo, nil
}

func (s *SchedulerManager) DeleteScheduler(ctx context.Context, schedulerName string) error {
	scheduler, err := s.schedulerStorage.GetScheduler(ctx, schedulerName)
	if err != nil {
		return fmt.Errorf("no scheduler found to delete: %w", err)
	}

	err = s.schedulerStorage.DeleteScheduler(ctx, "", scheduler)
	if err != nil {
		return fmt.Errorf("not able to delete scheduler %s: %w", schedulerName, err)
	}

	return nil
}

func (s *SchedulerManager) getScheduler(ctx context.Context, schedulerName string) (*entities.Scheduler, error) {
	scheduler, err := s.schedulerCache.GetScheduler(ctx, schedulerName)
	if err != nil || scheduler == nil {
		scheduler, err = s.schedulerStorage.GetScheduler(ctx, schedulerName)
	}
	return scheduler, err

}

func (s *SchedulerManager) newSchedulerInfo(ctx context.Context, scheduler *entities.Scheduler) (*entities.SchedulerInfo, error) {
	ready, err := s.roomStorage.GetRoomCountByStatus(ctx, scheduler.Name, game_room.GameStatusReady)
	if err != nil {
		return nil, fmt.Errorf("failing in couting game rooms in %s state: %s", game_room.GameStatusReady, err)
	}
	pending, err := s.roomStorage.GetRoomCountByStatus(ctx, scheduler.Name, game_room.GameStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failing in couting game rooms in %s state: %s", game_room.GameStatusPending, err)
	}

	occupied, err := s.roomStorage.GetRoomCountByStatus(ctx, scheduler.Name, game_room.GameStatusOccupied)
	if err != nil {
		return nil, fmt.Errorf("failing in couting game rooms in %s state: %s", game_room.GameStatusOccupied, err)
	}

	terminating, err := s.roomStorage.GetRoomCountByStatus(ctx, scheduler.Name, game_room.GameStatusTerminating)
	if err != nil {
		return nil, fmt.Errorf("failing in couting game rooms in %s state: %s", game_room.GameStatusTerminating, err)
	}

	return entities.NewSchedulerInfo(
		entities.WithName(scheduler.Name),
		entities.WithGame(scheduler.Game),
		entities.WithState(scheduler.State),
		entities.WithRoomsReplicas(scheduler.RoomsReplicas),
		entities.WithRoomsReady(ready),
		entities.WithRoomsOccupied(occupied),
		entities.WithRoomsPending(pending),
		entities.WithRoomsTerminating(terminating),
		entities.WithAutoscalingInfo(scheduler.Autoscaling),
	), nil
}

func (s *SchedulerManager) hasOngoingVersionOperation(ctx context.Context, schedulerName string) (string, error) {
	pending, err := s.operationManager.ListSchedulerPendingOperations(ctx, schedulerName)
	if err != nil {
		return "", fmt.Errorf("failed to list pending operations: %w", err)
	}

	for _, op := range pending {
		if slices.Contains([]string{newversion.OperationName, switchversion.OperationName}, op.DefinitionName) {
			s.logger.Debug("found pending version operation",
				zap.String("operationID", op.ID),
				zap.String("operationType", op.DefinitionName),
				zap.String("schedulerName", schedulerName))
			return op.ID, nil
		}
	}

	active, err := s.operationManager.ListSchedulerActiveOperations(ctx, schedulerName)
	if err != nil {
		return "", fmt.Errorf("failed to list active operations: %w", err)
	}

	for _, op := range active {
		if slices.Contains([]string{newversion.OperationName, switchversion.OperationName}, op.DefinitionName) {
			s.logger.Debug("found active version operation",
				zap.String("operationID", op.ID),
				zap.String("operationType", op.DefinitionName),
				zap.String("schedulerName", schedulerName))
			return op.ID, nil
		}
	}

	return "", nil
}
