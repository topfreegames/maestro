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
	"errors"
	"fmt"

	serviceerrors "github.com/topfreegames/maestro/internal/core/services/errors"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/logs"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/operations/switch_active_version"

	"github.com/topfreegames/maestro/internal/core/entities"

	"github.com/Masterminds/semver/v3"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"go.uber.org/zap"
)

type CreateNewSchedulerVersionExecutor struct {
	roomManager          ports.RoomManager
	schedulerManager     ports.SchedulerManager
	validationRoomIdsMap map[string]*game_room.GameRoom
}

var _ operations.Executor = (*CreateNewSchedulerVersionExecutor)(nil)

func NewExecutor(roomManager ports.RoomManager, schedulerManager ports.SchedulerManager) *CreateNewSchedulerVersionExecutor {
	return &CreateNewSchedulerVersionExecutor{
		roomManager:          roomManager,
		schedulerManager:     schedulerManager,
		validationRoomIdsMap: map[string]*game_room.GameRoom{},
	}
}

func (ex *CreateNewSchedulerVersionExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) operations.ExecutionError {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, op.DefinitionName),
		zap.String("operation_phase", "Execute"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	opDef, ok := definition.(*CreateNewSchedulerVersionDefinition)
	if !ok {
		return operations.NewErrUnexpected(fmt.Errorf("invalid operation definition for %s operation", ex.Name()))
	}

	newScheduler := opDef.NewScheduler
	currentActiveScheduler, err := ex.schedulerManager.GetActiveScheduler(ctx, opDef.NewScheduler.Name)
	if err != nil {
		logger.Error("error getting active scheduler", zap.Error(err))
		return operations.NewErrUnexpected(fmt.Errorf("error getting active scheduler: %w", err))
	}

	isSchedulerMajorVersion := currentActiveScheduler.IsMajorVersion(newScheduler)

	err = ex.populateSchedulerNewVersion(ctx, newScheduler, currentActiveScheduler.Spec.Version, isSchedulerMajorVersion)
	if err != nil {
		return operations.NewErrUnexpected(err)
	}

	if isSchedulerMajorVersion {
		err = ex.validateGameRoomCreation(ctx, newScheduler, logger)

		if err != nil {
			logger.Error("could not validate new game room creation", zap.Error(err))
			validationErr := fmt.Errorf("error creating new game room for validating new version: %w", err)
			if errors.Is(err, serviceerrors.ErrGameRoomStatusWaitingTimeout) {
				return operations.NewErrInvalidGru(validationErr)
			}
			return operations.NewErrUnexpected(validationErr)
		}
	}
	err = ex.createNewSchedulerVersionAndEnqueueSwitchVersionOp(ctx, newScheduler, logger, isSchedulerMajorVersion)
	if err != nil {
		return operations.NewErrUnexpected(err)
	}
	logger.Sugar().Infof("new scheduler version created: %s, is major: %t", newScheduler.Spec.Version, isSchedulerMajorVersion)
	logger.Sugar().Infof("%s operation succeded, %s operation enqueued to continue scheduler update process, switching to version %s", opDef.Name(), switch_active_version.OperationName, newScheduler.Spec.Version)
	return nil
}

func (ex *CreateNewSchedulerVersionExecutor) Rollback(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr operations.ExecutionError) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, op.DefinitionName),
		zap.String("operation_phase", "Rollback"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	if gameRoom, ok := ex.validationRoomIdsMap[op.SchedulerName]; ok {
		err := ex.roomManager.DeleteRoomAndWaitForRoomTerminating(ctx, gameRoom)
		if err != nil {
			logger.Error("error deleting new game room created for validation", zap.Error(err))
			return fmt.Errorf("error in Rollback function execution: %w", err)
		}
		ex.RemoveValidationRoomId(op.SchedulerName)
	}
	return nil
}

func (ex *CreateNewSchedulerVersionExecutor) Name() string {
	return OperationName
}

func (ex *CreateNewSchedulerVersionExecutor) validateGameRoomCreation(ctx context.Context, scheduler *entities.Scheduler, logger *zap.Logger) error {
	gameRoom, _, err := ex.roomManager.CreateRoomAndWaitForReadiness(ctx, *scheduler, true)
	if err != nil {
		logger.Error("error creating new game room for validating new version", zap.Error(err))
		return err
	}
	ex.AddValidationRoomId(scheduler.Name, gameRoom)
	err = ex.roomManager.DeleteRoomAndWaitForRoomTerminating(ctx, gameRoom)
	if err != nil {
		logger.Error("error deleting new game room created for validation", zap.Error(err))
	}
	ex.RemoveValidationRoomId(scheduler.Name)
	return nil
}

func (ex *CreateNewSchedulerVersionExecutor) AddValidationRoomId(schedulerName string, gameRoom *game_room.GameRoom) {
	ex.validationRoomIdsMap[schedulerName] = gameRoom
}

func (ex *CreateNewSchedulerVersionExecutor) RemoveValidationRoomId(schedulerName string) {
	delete(ex.validationRoomIdsMap, schedulerName)
}

func (ex *CreateNewSchedulerVersionExecutor) createNewSchedulerVersionAndEnqueueSwitchVersionOp(ctx context.Context, newScheduler *entities.Scheduler, logger *zap.Logger, replacePods bool) error {
	err := ex.schedulerManager.CreateNewSchedulerVersionAndEnqueueSwitchVersion(ctx, newScheduler)
	if err != nil {
		logger.Error("error creating new scheduler version in db", zap.Error(err))
		return fmt.Errorf("error creating new scheduler version in db: %w", err)
	}
	return nil
}

func (ex *CreateNewSchedulerVersionExecutor) populateSchedulerNewVersion(ctx context.Context, newScheduler *entities.Scheduler, currentVersion string, isMajorVersion bool) error {
	var newVersion semver.Version
	currentVersionSemver, err := semver.NewVersion(currentVersion)
	if err != nil {
		return fmt.Errorf("failed to parse scheduler current version: %w", err)
	}
	if isMajorVersion {
		newVersion, err = ex.calculateNewMajorVersion(ctx, newScheduler.Name, currentVersionSemver)
		if err != nil {
			return fmt.Errorf("failed to calculate new major version: %w", err)
		}
	} else {
		newVersion, err = ex.calculateNewMinorVersion(ctx, newScheduler.Name, currentVersionSemver)
		if err != nil {
			return fmt.Errorf("failed to calculate new minor version: %w", err)
		}
	}
	newScheduler.SetSchedulerVersion(newVersion.Original())
	newScheduler.SetSchedulerRollbackVersion(currentVersion)
	return nil
}

func (ex *CreateNewSchedulerVersionExecutor) calculateNewMajorVersion(ctx context.Context, schedulerName string, currentActiveVersionSemver *semver.Version) (semver.Version, error) {
	var newVersion semver.Version
	var greatestMajorVersion = currentActiveVersionSemver
	schedulerVersions, err := ex.schedulerManager.GetSchedulerVersions(ctx, schedulerName)
	if err != nil {
		return semver.Version{}, fmt.Errorf("failed to load scheduler versions: %w", err)
	}
	for _, schedulerVersion := range schedulerVersions {
		version, vErr := semver.NewVersion(schedulerVersion.Version)
		if vErr != nil {
			return newVersion, fmt.Errorf("failed to parse scheduler version %s: %w", schedulerVersion.Version, vErr)
		}
		if version.Major() > greatestMajorVersion.Major() {
			greatestMajorVersion = version
		}
	}

	newVersion = greatestMajorVersion.IncMajor()
	return newVersion, nil
}

func (ex *CreateNewSchedulerVersionExecutor) calculateNewMinorVersion(ctx context.Context, schedulerName string, currentActiveVersionSemver *semver.Version) (semver.Version, error) {
	var newVersion semver.Version
	var greatestMinorVersion = currentActiveVersionSemver
	schedulerVersions, err := ex.schedulerManager.GetSchedulerVersions(ctx, schedulerName)
	if err != nil {
		return semver.Version{}, fmt.Errorf("failed to load scheduler versions: %w", err)
	}

	for _, schedulerVersion := range schedulerVersions {
		version, vErr := semver.NewVersion(schedulerVersion.Version)
		if vErr != nil {
			return newVersion, fmt.Errorf("failed to parse scheduler version %s: %w", schedulerVersion.Version, vErr)
		}
		if version.Major() == currentActiveVersionSemver.Major() && version.Minor() > greatestMinorVersion.Minor() {
			greatestMinorVersion = version
		}
	}

	newVersion = greatestMinorVersion.IncMinor()
	return newVersion, nil
}
