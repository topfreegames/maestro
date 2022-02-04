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

package newschedulerversion

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports/errors"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/services/interfaces"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
	"go.uber.org/zap"
)

type CreateNewSchedulerVersionExecutor struct {
	roomManager      *room_manager.RoomManager
	schedulerManager interfaces.SchedulerManager
}

func NewExecutor(roomManager *room_manager.RoomManager, schedulerManager interfaces.SchedulerManager) *CreateNewSchedulerVersionExecutor {
	return &CreateNewSchedulerVersionExecutor{
		roomManager:      roomManager,
		schedulerManager: schedulerManager,
	}
}

func (ex *CreateNewSchedulerVersionExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	opDef, ok := definition.(*CreateNewSchedulerVersionDefinition)
	if !ok {
		return errors.NewErrInvalidArgument(fmt.Sprintf("invalid operation definition for %s operation", ex.Name()))
	}
	logger := zap.L().With(
		zap.String("scheduler_name", op.SchedulerName),
		zap.String("operation_definition", opDef.Name()),
		zap.String("operation_id", op.ID),
	)
	newScheduler := opDef.NewScheduler
	currentActiveScheduler, err := ex.schedulerManager.GetActiveScheduler(ctx, opDef.NewScheduler.Name)
	if err != nil {
		logger.Error("error getting active scheduler", zap.Error(err))
		return fmt.Errorf("error getting active scheduler: %w", err)
	}

	currentVersion, err := semver.NewVersion(currentActiveScheduler.Spec.Version)
	if err != nil {
		return fmt.Errorf("failed to parse scheduler current version: %w", err)
	}

	newVersion := currentVersion.IncMinor()

	// Check if it is major
	isMajor := ex.schedulerManager.IsMajorVersionUpdate(currentActiveScheduler, newScheduler)

	// If it is major, validate game room
	if isMajor {
		newVersion = currentVersion.IncMajor()
		err := ex.validateNewGameRoomVersion(ctx, logger, newScheduler)
		if err != nil {
			logger.Error("could not validate new game room creation", zap.Error(err))
			return err
		}
	}

	newScheduler.Spec.Version = newVersion.Original()
	newScheduler.RollbackVersion = currentActiveScheduler.Spec.Version

	// Create new scheduler version in the DB
	err = ex.schedulerManager.CreateNewSchedulerVersion(ctx, newScheduler)
	if err != nil {
		logger.Error("error creating new scheduler version in db", zap.Error(err))
		return fmt.Errorf("error creating new scheduler version in db: %w", err)
	}

	// Enqueue switch active version operation
	switchActiveVersionOp, err := ex.schedulerManager.EnqueueSwitchActiveVersionOperation(ctx, newScheduler)
	if err != nil {
		logger.Error("error enqueuing switch active version operation", zap.Error(err))
		return fmt.Errorf("error enqueuing switch active version operation: %w", err)
	}

	logger.Info(fmt.Sprintf("%s operation succeded, %s operation enqueued to continue scheduler update process", opDef.Name(), switchActiveVersionOp.DefinitionName))
	return nil
}

func (ex *CreateNewSchedulerVersionExecutor) OnError(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	return nil
}

func (ex *CreateNewSchedulerVersionExecutor) Name() string {
	return OperationName
}

func (ex *CreateNewSchedulerVersionExecutor) validateNewGameRoomVersion(ctx context.Context, logger *zap.Logger, newScheduler *entities.Scheduler) error {
	gameRoom, _, err := ex.roomManager.CreateRoom(ctx, *newScheduler)
	if err != nil {
		logger.Error("error creating new game room for validating new version", zap.Error(err))
		return fmt.Errorf("error creating new game room for validating new version: %w", err)
	}
	err = ex.roomManager.DeleteRoom(ctx, gameRoom)
	if err != nil {
		logger.Error("error deleting new game room created for validation", zap.Error(err))
		return nil
	}
	return nil
}
