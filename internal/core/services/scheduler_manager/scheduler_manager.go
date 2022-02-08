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

	"github.com/topfreegames/maestro/internal/core/operations/switch_active_version"

	"github.com/topfreegames/maestro/internal/core/operations/newschedulerversion"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/filters"
	"github.com/topfreegames/maestro/internal/core/operations/add_rooms"
	"github.com/topfreegames/maestro/internal/core/operations/create_scheduler"
	"github.com/topfreegames/maestro/internal/core/operations/remove_rooms"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"go.uber.org/zap"
)

type SchedulerManager struct {
	schedulerStorage ports.SchedulerStorage
	operationManager *operation_manager.OperationManager
}

func NewSchedulerManager(schedulerStorage ports.SchedulerStorage, operationManager *operation_manager.OperationManager) *SchedulerManager {
	return &SchedulerManager{
		schedulerStorage: schedulerStorage,
		operationManager: operationManager,
	}
}

func (s *SchedulerManager) GetActiveScheduler(ctx context.Context, schedulerName string) (*entities.Scheduler, error) {
	activeScheduler, err := s.schedulerStorage.GetScheduler(ctx, schedulerName)
	if err != nil {
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

	operation, err := s.operationManager.CreateOperation(ctx, scheduler.Name, &create_scheduler.CreateSchedulerDefinition{})
	if err != nil {
		return nil, fmt.Errorf("failing in creating the operation: %s: %s", create_scheduler.OperationName, err)
	}

	zap.L().Info("scheduler enqueued to be created", zap.String("scheduler", scheduler.Name), zap.String("operation", operation.ID))

	return s.schedulerStorage.GetScheduler(ctx, scheduler.Name)
}

func (s *SchedulerManager) CreateNewSchedulerVersion(ctx context.Context, scheduler *entities.Scheduler) error {
	err := scheduler.Validate()
	if err != nil {
		return fmt.Errorf("failing in creating schedule: %w", err)
	}

	err = s.schedulerStorage.CreateSchedulerVersion(ctx, scheduler)
	if err != nil {
		return err
	}
	return nil
}

func (s *SchedulerManager) GetAllSchedulers(ctx context.Context) ([]*entities.Scheduler, error) {
	return s.schedulerStorage.GetAllSchedulers(ctx)
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
		return nil, fmt.Errorf("failed to schedule 'create new scheduler version' operation: %w", err)
	}

	return op, nil
}

func (s *SchedulerManager) EnqueueSwitchActiveVersionOperation(ctx context.Context, newScheduler *entities.Scheduler, replacePods bool) (*operation.Operation, error) {
	err := newScheduler.Validate()
	if err != nil {
		return nil, err
	}
	opDef := &switch_active_version.SwitchActiveVersionDefinition{NewActiveScheduler: *newScheduler, ReplacePods: replacePods}

	op, err := s.operationManager.CreateOperation(ctx, newScheduler.Name, opDef)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule %s operation: %w", opDef.Name(), err)
	}

	return op, nil
}

// IsMajorVersionUpdate checks if the scheduler changes are major or not.
// We consider major changes if the Instances need to be recreated, in this case
// the following fields require it: `Spec` and `PortRange`. Any other field
// change is considered minor (we don't need to recreate instances).
func (s *SchedulerManager) IsMajorVersionUpdate(currentScheduler, newScheduler *entities.Scheduler) bool {
	// Compare schedulers `Spec` and `PortRange`. This means that if this
	// returns `false` it is a major version.
	return !cmp.Equal(
		currentScheduler,
		newScheduler,
		cmpopts.IgnoreFields(
			entities.Scheduler{},
			"Name",
			"Game",
			"State",
			"RollbackVersion",
			"CreatedAt",
			"MaxSurge",
		),
	)
}

func (s *SchedulerManager) SwitchActiveScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	err := s.schedulerStorage.UpdateScheduler(ctx, scheduler)
	if err != nil {
		return fmt.Errorf("error switch scheduler active version to scheduler \"%s\", version \"%s\". error: %w", scheduler.Name, scheduler.Spec.Version, err)
	}
	return nil
}
