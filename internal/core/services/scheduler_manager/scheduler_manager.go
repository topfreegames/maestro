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

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations/add_rooms"
	"github.com/topfreegames/maestro/internal/core/operations/create_scheduler"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"go.uber.org/zap"
	"gopkg.in/validator.v2"
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

func (s *SchedulerManager) CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) (*entities.Scheduler, error) {
	scheduler.State = entities.StateCreating

	err := s.validateScheduler(scheduler)
	if err != nil {
		return nil, err
	}

	err = s.schedulerStorage.CreateScheduler(ctx, scheduler)
	if err != nil {
		return nil, err
	}

	operation, err := s.operationManager.CreateOperation(ctx, scheduler.Name, &create_scheduler.CreateSchedulerDefinition{})
	if err != nil {
		return nil, fmt.Errorf("failed to schedule 'create scheduler' operation: %w", err)
	}

	zap.L().Info("scheduler enqueued to be created", zap.String("scheduler", scheduler.Name), zap.String("operation", operation.ID))

	return s.schedulerStorage.GetScheduler(ctx, scheduler.Name)
}

func (s *SchedulerManager) GetAllSchedulers(ctx context.Context) ([]*entities.Scheduler, error) {
	return s.schedulerStorage.GetAllSchedulers(ctx)
}

func (s *SchedulerManager) AddRooms(ctx context.Context, schedulerName string, amount int32) (*operation.Operation, error) {
	
	_, err := s.schedulerStorage.GetScheduler(ctx, schedulerName)
	if err != nil {
		return nil, fmt.Errorf("No scheduler found to add rooms on it: %w", err)
	}

	op, err := s.operationManager.CreateOperation(ctx, schedulerName, &add_rooms.AddRoomsDefinition{
		Amount: amount,
	})
	if err != nil {
		return nil, fmt.Errorf("Not able to schedule the 'add rooms' operation: %w", err)
	}

	return op, nil
}

// WARN: This function should be called only on private scope of SchedulerManager.
// WARN: Other packages should NEVER call this function.
func (s *SchedulerManager) validateScheduler(scheduler *entities.Scheduler) error {
	return validator.Validate(scheduler)
}
