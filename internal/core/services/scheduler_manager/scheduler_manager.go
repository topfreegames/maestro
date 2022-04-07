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

package scheduler_manager

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/operations/newschedulerversion"
	"github.com/topfreegames/maestro/internal/core/operations/switch_active_version"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager/patch_scheduler"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/filters"
	"github.com/topfreegames/maestro/internal/core/operations/add_rooms"
	"github.com/topfreegames/maestro/internal/core/operations/create_scheduler"
	"github.com/topfreegames/maestro/internal/core/operations/remove_rooms"
	"github.com/topfreegames/maestro/internal/core/ports"
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
	activeScheduler, err := s.schedulerStorage.GetScheduler(ctx, schedulerName)
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

	op, err := s.operationManager.CreateOperation(ctx, scheduler.Name, &create_scheduler.CreateSchedulerDefinition{NewScheduler: scheduler})
	if err != nil {
		return nil, fmt.Errorf("failing in creating the operation: %s: %s", create_scheduler.OperationName, err)
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

func (s *SchedulerManager) CreateNewSchedulerVersionAndEnqueueSwitchVersion(ctx context.Context, scheduler *entities.Scheduler) error {
	err := scheduler.Validate()
	if err != nil {
		return fmt.Errorf("failing in creating schedule: %w", err)
	}

	err = s.schedulerStorage.RunWithTransaction(ctx, func(transactionId ports.TransactionID) error {
		err := s.schedulerStorage.CreateSchedulerVersion(ctx, transactionId, scheduler)
		if err != nil {
			return err
		}

		_, err = s.EnqueueSwitchActiveVersionOperation(ctx, scheduler.Name, scheduler.Spec.Version)
		if err != nil {
			return fmt.Errorf("error enqueuing switch active version operation: %w", err)
		}
		return nil

	})
	if err != nil {
		return err
	}
	return nil
}

func (s *SchedulerManager) PatchSchedulerAndCreateNewSchedulerVersionOperation(ctx context.Context, schedulerName string, patchMap map[string]interface{}) (*operation.Operation, error) {
	scheduler, err := s.schedulerStorage.GetScheduler(ctx, schedulerName)
	if err != nil {
		return nil, fmt.Errorf("no scheduler found, can not create new version for inexistent scheduler: %w", err)
	}

	scheduler, err = patch_scheduler.PatchScheduler(*scheduler, patchMap)
	if err != nil {
		return nil, fmt.Errorf("error patching scheduler: %w", err)
	}

	opDef := &newschedulerversion.CreateNewSchedulerVersionDefinition{NewScheduler: scheduler}

	op, err := s.operationManager.CreateOperation(ctx, scheduler.Name, opDef)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule %s operation: %w", opDef.Name(), err)
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

func (s *SchedulerManager) AddRooms(ctx context.Context, schedulerName string, amount int32) (*operation.Operation, error) {

	_, err := s.schedulerStorage.GetScheduler(ctx, schedulerName)
	if err != nil {
		return nil, fmt.Errorf("no scheduler found to add rooms on it: %w", err)
	}

	op, err := s.operationManager.CreateOperation(ctx, schedulerName, &add_rooms.AddRoomsDefinition{
		Amount: amount,
	})
	if err != nil {
		return nil, fmt.Errorf("not able to schedule the 'add rooms' operation: %w", err)
	}

	return op, nil
}

func (s *SchedulerManager) RemoveRooms(ctx context.Context, schedulerName string, amount int) (*operation.Operation, error) {

	_, err := s.schedulerStorage.GetScheduler(ctx, schedulerName)
	if err != nil {
		return nil, fmt.Errorf("no scheduler found for removing rooms: %w", err)
	}

	op, err := s.operationManager.CreateOperation(ctx, schedulerName, &remove_rooms.RemoveRoomsDefinition{
		Amount: amount,
	})
	if err != nil {
		return nil, fmt.Errorf("not able to schedule the 'remove rooms' operation: %w", err)
	}

	return op, nil
}

func (s *SchedulerManager) EnqueueNewSchedulerVersionOperation(ctx context.Context, scheduler *entities.Scheduler) (*operation.Operation, error) {
	currentScheduler, err := s.schedulerStorage.GetScheduler(ctx, scheduler.Name)
	if err != nil {
		return nil, fmt.Errorf("no scheduler found, can not create new version for inexistent scheduler: %w", err)
	}

	scheduler.Spec.Version = currentScheduler.Spec.Version
	err = scheduler.Validate()
	if err != nil {
		return nil, err
	}

	opDef := &newschedulerversion.CreateNewSchedulerVersionDefinition{NewScheduler: scheduler}

	op, err := s.operationManager.CreateOperation(ctx, scheduler.Name, opDef)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule %s operation: %w", opDef.Name(), err)
	}

	return op, nil
}

func (s *SchedulerManager) EnqueueSwitchActiveVersionOperation(ctx context.Context, schedulerName, newVersion string) (*operation.Operation, error) {
	opDef := &switch_active_version.SwitchActiveVersionDefinition{NewActiveVersion: newVersion}
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

	err = s.schedulerStorage.DeleteScheduler(ctx, scheduler)
	if err != nil {
		return fmt.Errorf("not able to delete scheduler %s: %w", schedulerName, err)
	}

	return nil
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
	return entities.NewSchedulerInfo(scheduler.Name, scheduler.Game, scheduler.State, ready, occupied, pending, terminating), nil
}
