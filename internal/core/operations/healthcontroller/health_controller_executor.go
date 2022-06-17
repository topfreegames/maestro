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

package healthcontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/topfreegames/maestro/internal/core/ports/autoscaler"

	"github.com/topfreegames/maestro/internal/core/services/room_manager"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/operations/add_rooms"
	"github.com/topfreegames/maestro/internal/core/operations/remove_rooms"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"go.uber.org/zap"
)

// Config have the configs to execute healthcontroller.
type Config struct {
	RoomInitializationTimeout time.Duration
	RoomPingTimeout           time.Duration
}

// SchedulerHealthControllerExecutor holds dependencies to execute SchedulerHealthControllerExecutor.
type SchedulerHealthControllerExecutor struct {
	autoscaler       autoscaler.Autoscaler
	roomStorage      ports.RoomStorage
	instanceStorage  ports.GameRoomInstanceStorage
	schedulerStorage ports.SchedulerStorage
	operationManager ports.OperationManager
	config           Config
}

var _ operations.Executor = (*SchedulerHealthControllerExecutor)(nil)

// NewExecutor creates a new instance of SchedulerHealthControllerExecutor.
func NewExecutor(roomStorage ports.RoomStorage, instanceStorage ports.GameRoomInstanceStorage, schedulerStorage ports.SchedulerStorage, operationManager ports.OperationManager, roomManagerConfig room_manager.RoomManagerConfig, autoscaler autoscaler.Autoscaler, config Config) *SchedulerHealthControllerExecutor {
	return &SchedulerHealthControllerExecutor{
		autoscaler:       autoscaler,
		roomStorage:      roomStorage,
		instanceStorage:  instanceStorage,
		schedulerStorage: schedulerStorage,
		operationManager: operationManager,
		config:           config,
	}
}

// Execute run the operation health_controller.
func (ex *SchedulerHealthControllerExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) operations.ExecutionError {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, op.DefinitionName),
		zap.String("operation_phase", "Execute"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)

	gameRoomIDs, instances, scheduler, err := ex.loadActualState(ctx, op, logger)
	if err != nil {
		return operations.NewErrUnexpected(err)
	}

	nonexistentGameRoomIDs := ex.checkNonexistentGameRoomsIDs(gameRoomIDs, instances)
	if len(nonexistentGameRoomIDs) > 0 {
		logger.Error("found registered rooms that no longer exists")
		ex.tryEnsureCorrectRoomsOnStorage(ctx, op, logger, nonexistentGameRoomIDs)
	}

	existentGameRoomIDs := filterExistentGameRooms(gameRoomIDs, nonexistentGameRoomIDs)

	availableRooms, expiredRooms := ex.findAvailableAndExpiredRooms(ctx, op, existentGameRoomIDs)
	if len(expiredRooms) > 0 {
		logger.Sugar().Infof("found %v expired rooms to be deleted", len(expiredRooms))
		err = ex.enqueueRemoveExpiredRooms(ctx, op, logger, expiredRooms)
		if err != nil {
			logger.Error("could not enqueue operation to delete expired rooms", zap.Error(err))
		}
	}

	desiredNumberOfRooms, err := ex.getDesiredNumberOfRooms(ctx, logger, scheduler)
	if err != nil {
		logger.Error("error getting the desired number of rooms", zap.Error(err))
		return operations.NewErrUnexpected(err)
	}

	err = ex.ensureDesiredAmountOfInstances(ctx, op, logger, len(availableRooms), desiredNumberOfRooms)
	if err != nil {
		logger.Error("cannot ensure desired amount of instances", zap.Error(err))
		return operations.NewErrUnexpected(err)
	}

	return nil
}

// Rollback does not execute anything when a rollback executes.
func (ex *SchedulerHealthControllerExecutor) Rollback(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr operations.ExecutionError) error {
	return nil
}

// Name return the name of the operation.
func (ex *SchedulerHealthControllerExecutor) Name() string {
	return OperationName
}

func (ex *SchedulerHealthControllerExecutor) loadActualState(ctx context.Context, op *operation.Operation, logger *zap.Logger) (gameRoomIDs []string, instances []*game_room.Instance, scheduler *entities.Scheduler, err error) {
	gameRoomIDs, err = ex.roomStorage.GetAllRoomIDs(ctx, op.SchedulerName)
	if err != nil {
		logger.Error("error fetching game rooms")
		return
	}
	instances, err = ex.instanceStorage.GetAllInstances(ctx, op.SchedulerName)
	if err != nil {
		logger.Error("error fetching instances")
		return
	}

	scheduler, err = ex.schedulerStorage.GetScheduler(ctx, op.SchedulerName)
	if err != nil {
		return
	}
	return
}

func (ex *SchedulerHealthControllerExecutor) checkNonexistentGameRoomsIDs(gameRoomIDs []string, gameRoomInstances []*game_room.Instance) []string {
	var nonexistentGameRoomsIDs []string
	for _, gameRoomID := range gameRoomIDs {
		found := false
		for _, instance := range gameRoomInstances {
			if instance.ID == gameRoomID {
				found = true
				break
			}
		}
		if !found {
			nonexistentGameRoomsIDs = append(nonexistentGameRoomsIDs, gameRoomID)
		}
	}
	return nonexistentGameRoomsIDs
}

func (ex *SchedulerHealthControllerExecutor) tryEnsureCorrectRoomsOnStorage(ctx context.Context, op *operation.Operation, logger *zap.Logger, nonexistentGameRoomIDs []string) {
	for _, gameRoomID := range nonexistentGameRoomIDs {
		err := ex.roomStorage.DeleteRoom(ctx, op.SchedulerName, gameRoomID)
		if err != nil {
			msg := fmt.Sprintf("could not delete nonexistent room %s from storage", gameRoomID)
			logger.Warn(msg, zap.Error(err))
			continue
		}
		logger.Sugar().Infof("remove nonexistent room on storage: %s", gameRoomID)
	}
}

func (ex *SchedulerHealthControllerExecutor) ensureDesiredAmountOfInstances(ctx context.Context, op *operation.Operation, logger *zap.Logger, actualAmount, desiredAmount int) error {
	var msgToAppend string

	switch {
	case actualAmount > desiredAmount: // Need to scale down
		removeAmount := actualAmount - desiredAmount
		removeOperation, err := ex.operationManager.CreatePriorityOperation(ctx, op.SchedulerName, &remove_rooms.RemoveRoomsDefinition{
			Amount: removeAmount,
		})
		if err != nil {
			return err
		}
		msgToAppend = fmt.Sprintf("created operation (id: %s) to remove %v rooms.", removeOperation.ID, removeAmount)
	case actualAmount < desiredAmount: // Need to scale up
		addAmount := desiredAmount - actualAmount
		addOperation, err := ex.operationManager.CreatePriorityOperation(ctx, op.SchedulerName, &add_rooms.AddRoomsDefinition{
			Amount: int32(addAmount),
		})
		if err != nil {
			return err
		}
		msgToAppend = fmt.Sprintf("created operation (id: %s) to add %v rooms.", addOperation.ID, addAmount)
	default: // No need to scale
		msgToAppend = "current amount of rooms is equal to desired amount, no changes needed"

	}

	logger.Info(msgToAppend)
	ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, msgToAppend)
	return nil
}

func (ex *SchedulerHealthControllerExecutor) findAvailableAndExpiredRooms(ctx context.Context, op *operation.Operation, gameRoomsIDs []string) (availableRoomsIDs, expiredRoomsIDs []string) {
	for _, gameRoomID := range gameRoomsIDs {
		room, err := ex.roomStorage.GetRoom(ctx, op.SchedulerName, gameRoomID)
		if err != nil {
			continue
		}

		switch {
		case ex.isInitializingRoomExpired(room):
			expiredRoomsIDs = append(expiredRoomsIDs, gameRoomID)
		case ex.isRoomPingExpired(room):
			expiredRoomsIDs = append(expiredRoomsIDs, gameRoomID)
		case ex.isRoomStatus(room, game_room.GameStatusTerminating):
			continue
		case ex.isRoomStatus(room, game_room.GameStatusError):
			continue
		default:
			availableRoomsIDs = append(availableRoomsIDs, gameRoomID)
		}
	}

	return availableRoomsIDs, expiredRoomsIDs
}

func (ex *SchedulerHealthControllerExecutor) isInitializingRoomExpired(room *game_room.GameRoom) bool {
	timeDurationInPendingState := time.Since(room.CreatedAt)
	return (ex.isRoomStatus(room, game_room.GameStatusPending) || ex.isRoomStatus(room, game_room.GameStatusUnready)) &&
		timeDurationInPendingState > ex.config.RoomInitializationTimeout
}

func (ex *SchedulerHealthControllerExecutor) isRoomPingExpired(room *game_room.GameRoom) bool {
	timeDurationWithoutPing := time.Since(room.LastPingAt)
	return !ex.isRoomStatus(room, game_room.GameStatusPending) && timeDurationWithoutPing > ex.config.RoomPingTimeout
}

func (ex *SchedulerHealthControllerExecutor) isRoomStatus(room *game_room.GameRoom, status game_room.GameRoomStatus) bool {
	return room.Status == status
}

func (ex *SchedulerHealthControllerExecutor) enqueueRemoveExpiredRooms(ctx context.Context, op *operation.Operation, logger *zap.Logger, expiredRoomsIDs []string) error {
	removeOperation, err := ex.operationManager.CreatePriorityOperation(ctx, op.SchedulerName, &remove_rooms.RemoveRoomsDefinition{
		RoomsIDs: expiredRoomsIDs,
	})
	if err != nil {
		return err
	}

	msgToAppend := fmt.Sprintf("created operation (id: %s) to remove %v expired rooms.", removeOperation.ID, len(expiredRoomsIDs))
	logger.Info(msgToAppend)
	ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, msgToAppend)

	return nil
}

func (ex *SchedulerHealthControllerExecutor) getDesiredNumberOfRooms(ctx context.Context, logger *zap.Logger, scheduler *entities.Scheduler) (int, error) {
	if scheduler.Autoscaling != nil && scheduler.Autoscaling.Enabled {
		desiredNumberOfRooms, err := ex.autoscaler.CalculateDesiredNumberOfRooms(ctx, scheduler)
		if err != nil {
			logger.Error("error using autoscaling policy to calculate the desired number of rooms", zap.Error(err))

			return 0, err
		}
		return desiredNumberOfRooms, nil
	}

	return scheduler.RoomsReplicas, nil
}

func filterExistentGameRooms(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}
