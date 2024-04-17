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

	"github.com/topfreegames/maestro/internal/core/operations/rooms/add"
	"github.com/topfreegames/maestro/internal/core/operations/rooms/remove"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"go.uber.org/zap"
)

// Config have the configs to execute healthcontroller.
type Config struct {
	RoomInitializationTimeout time.Duration
	RoomPingTimeout           time.Duration
	RoomDeletionTimeout       time.Duration
}

// Executor holds dependencies to execute Executor.
type Executor struct {
	autoscaler       ports.Autoscaler
	roomStorage      ports.RoomStorage
	roomManager      ports.RoomManager
	instanceStorage  ports.GameRoomInstanceStorage
	schedulerStorage ports.SchedulerStorage
	operationManager ports.OperationManager
	config           Config
}

var _ operations.Executor = (*Executor)(nil)

// NewExecutor creates a new instance of Executor.
func NewExecutor(
	roomStorage ports.RoomStorage,
	roomManager ports.RoomManager,
	instanceStorage ports.GameRoomInstanceStorage,
	schedulerStorage ports.SchedulerStorage,
	operationManager ports.OperationManager,
	autoscaler ports.Autoscaler,
	config Config,
) *Executor {
	return &Executor{
		autoscaler: autoscaler,
		// TODO: replace roomStorage operations with roomManager
		roomStorage:      roomStorage,
		roomManager:      roomManager,
		instanceStorage:  instanceStorage,
		schedulerStorage: schedulerStorage,
		operationManager: operationManager,
		config:           config,
	}
}

// Execute run the operation health_controller.
func (ex *Executor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, op.DefinitionName),
		zap.String(logs.LogFieldOperationPhase, "Execute"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	def := definition.(*Definition)

	gameRoomIDs, instances, scheduler, err := ex.loadActualState(ctx, op, logger)
	if err != nil {
		return err
	}

	logger = logger.With(zap.String(logs.LogFieldSchedulerVersion, scheduler.Spec.Version))

	nonexistentGameRoomsIDs, existentGameRoomsInstancesMap := ex.mapExistentAndNonExistentGameRooms(gameRoomIDs, instances)
	if len(nonexistentGameRoomsIDs) > 0 {
		logger.Error("found registered rooms that no longer exists")
		ex.tryEnsureCorrectRoomsOnStorage(ctx, op, logger, nonexistentGameRoomsIDs)
	}

	availableRooms, expiredRooms := ex.findAvailableAndExpiredRooms(ctx, scheduler, op, existentGameRoomsInstancesMap)
	reportCurrentNumberOfRooms(scheduler.Game, scheduler.Name, len(availableRooms))

	if len(expiredRooms) > 0 {
		logger.Sugar().Infof("found %v expired rooms to be deleted", len(expiredRooms))
		err = ex.enqueueRemoveRooms(ctx, op, logger, expiredRooms)
		if err != nil {
			logger.Error("could not enqueue operation to delete expired rooms", zap.Error(err))
		}
		ex.setTookAction(def, true)
	}

	desiredNumberOfRooms, err := ex.getDesiredNumberOfRooms(ctx, logger, scheduler)
	if err != nil {
		logger.Error("error getting the desired number of rooms", zap.Error(err))
		return err
	}
	reportDesiredNumberOfRooms(scheduler.Game, scheduler.Name, desiredNumberOfRooms)

	// Check if the system is in a rollingUpdate by listing rooms that are not the current scheduler version
	roomsPreviousSchedulerVersion, isRollingUpdate := ex.checkRollingUpdate(ctx, logger, scheduler, availableRooms)
	if isRollingUpdate {
		return ex.performRollingUpdate(ctx, op, def, logger, scheduler, desiredNumberOfRooms, availableRooms, roomsPreviousSchedulerVersion)
	}
	err = ex.ensureDesiredAmountOfInstances(ctx, op, def, scheduler, logger, len(availableRooms), desiredNumberOfRooms)
	if err != nil {
		logger.Error("cannot ensure desired amount of instances", zap.Error(err))
		return err
	}

	return nil
}

// Rollback does not execute anything when a rollback executes.
func (ex *Executor) Rollback(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	return nil
}

// Name return the name of the operation.
func (ex *Executor) Name() string {
	return OperationName
}

func (ex *Executor) loadActualState(ctx context.Context, op *operation.Operation, logger *zap.Logger) (gameRoomIDs []string, instances []*game_room.Instance, scheduler *entities.Scheduler, err error) {
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

func (ex *Executor) tryEnsureCorrectRoomsOnStorage(ctx context.Context, op *operation.Operation, logger *zap.Logger, nonexistentGameRoomIDs []string) {
	for _, gameRoomID := range nonexistentGameRoomIDs {
		roomStorageErr := ex.roomStorage.DeleteRoom(ctx, op.SchedulerName, gameRoomID)
		instanceStorageErr := ex.instanceStorage.DeleteInstance(ctx, op.SchedulerName, gameRoomID)
		if roomStorageErr != nil {
			msg := fmt.Sprintf("could not delete nonexistent room %s from storage", gameRoomID)
			logger.Warn(msg, zap.Error(roomStorageErr))
		}

		if instanceStorageErr != nil {
			msg := fmt.Sprintf("could not delete nonexistent instance %s from storage", gameRoomID)
			logger.Warn(msg, zap.Error(instanceStorageErr))
			continue
		}

		logger.Sugar().Infof("removed nonexistent room from instance and game room storage: %s", gameRoomID)
	}
}

func (ex *Executor) ensureDesiredAmountOfInstances(ctx context.Context, op *operation.Operation, def *Definition, scheduler *entities.Scheduler, logger *zap.Logger, actualAmount, desiredAmount int) error {
	var msgToAppend string
	var tookAction bool

	logger = logger.With(zap.Int("actual", actualAmount), zap.Int("desired", desiredAmount))
	switch {
	case actualAmount > desiredAmount: // Need to scale down
		can, msg := ex.canPerformDownscale(ctx, scheduler, logger)
		if can {
			scheduler.LastDownscaleAt = time.Now().UTC()
			if err := ex.schedulerStorage.UpdateScheduler(ctx, scheduler); err != nil {
				logger.Error("error updating scheduler", zap.Error(err))
				return err
			}
			removeAmount := actualAmount - desiredAmount
			removeOperation, err := ex.operationManager.CreatePriorityOperation(ctx, op.SchedulerName, &remove.Definition{
				Amount: removeAmount,
			})
			if err != nil {
				return err
			}
			tookAction = true
			msgToAppend = fmt.Sprintf("created operation (id: %s) to remove %v rooms.", removeOperation.ID, removeAmount)
		} else {
			tookAction = false
			msgToAppend = msg
		}
	case actualAmount < desiredAmount: // Need to scale up
		addAmount := desiredAmount - actualAmount
		addOperation, err := ex.operationManager.CreatePriorityOperation(ctx, op.SchedulerName, &add.Definition{
			Amount: int32(addAmount),
		})
		if err != nil {
			return err
		}
		tookAction = true
		msgToAppend = fmt.Sprintf("created operation (id: %s) to add %v rooms.", addOperation.ID, addAmount)
	default: // No need to scale
		tookAction = false
		msgToAppend = "current amount of rooms is equal to desired amount, no changes needed"
	}

	logger.Info(msgToAppend)
	ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, msgToAppend)
	ex.setTookAction(def, tookAction)
	return nil
}

func (ex *Executor) findAvailableAndExpiredRooms(ctx context.Context, scheduler *entities.Scheduler, op *operation.Operation, existentGameRoomsInstancesMap map[string]*game_room.Instance) (availableRoomsIDs, expiredRoomsIDs []string) {
	terminationTimedOutRooms := 0
	terminatedRooms := 0

	for gameRoomId, instance := range existentGameRoomsInstancesMap {
		if instance.Status.Type == game_room.InstancePending {
			availableRoomsIDs = append(availableRoomsIDs, gameRoomId)
			continue
		}

		room, err := ex.roomStorage.GetRoom(ctx, op.SchedulerName, gameRoomId)
		if err != nil {
			continue
		}

		switch {
		case ex.isInitializingRoomExpired(room):
			expiredRoomsIDs = append(expiredRoomsIDs, gameRoomId)
		case ex.isRoomPingExpired(room):
			expiredRoomsIDs = append(expiredRoomsIDs, gameRoomId)
		case ex.isRoomTerminatingExpired(room):
			expiredRoomsIDs = append(expiredRoomsIDs, gameRoomId)
			terminationTimedOutRooms += 1
		case ex.isRoomStatus(room, game_room.GameStatusTerminated):
			expiredRoomsIDs = append(expiredRoomsIDs, gameRoomId)
			terminatedRooms += 1
		case ex.isRoomStatus(room, game_room.GameStatusTerminating):
			continue
		case ex.isRoomStatus(room, game_room.GameStatusError):
			continue
		default:
			availableRoomsIDs = append(availableRoomsIDs, gameRoomId)
		}
	}

	reportRoomsWithTerminationTimeout(scheduler.Game, scheduler.Name, terminationTimedOutRooms)
	reportRoomsProperlyTerminated(scheduler.Game, scheduler.Name, terminatedRooms)

	return availableRoomsIDs, expiredRoomsIDs
}

func (ex *Executor) isInitializingRoomExpired(room *game_room.GameRoom) bool {
	timeDurationInPendingState := time.Since(room.CreatedAt)
	return (ex.isRoomStatus(room, game_room.GameStatusPending) || ex.isRoomStatus(room, game_room.GameStatusUnready)) &&
		timeDurationInPendingState > ex.config.RoomInitializationTimeout
}

func (ex *Executor) isRoomPingExpired(room *game_room.GameRoom) bool {
	timeDurationWithoutPing := time.Since(room.LastPingAt)
	return !ex.isRoomStatus(room, game_room.GameStatusPending) && timeDurationWithoutPing > ex.config.RoomPingTimeout
}

func (ex *Executor) isRoomTerminatingExpired(room *game_room.GameRoom) bool {
	timeDurationWithoutPing := time.Since(room.LastPingAt)
	return ex.isRoomStatus(room, game_room.GameStatusTerminating) && timeDurationWithoutPing > ex.config.RoomDeletionTimeout
}

func (ex *Executor) isRoomStatus(room *game_room.GameRoom, status game_room.GameRoomStatus) bool {
	return room.Status == status
}

func (ex *Executor) enqueueRemoveRooms(ctx context.Context, op *operation.Operation, logger *zap.Logger, roomsIDs []string) error {
	removeOperation, err := ex.operationManager.CreatePriorityOperation(ctx, op.SchedulerName, &remove.Definition{
		RoomsIDs: roomsIDs,
	})
	if err != nil {
		return err
	}

	msgToAppend := fmt.Sprintf("created operation (id: %s) to remove %v rooms.", removeOperation.ID, len(roomsIDs))
	logger.Info(msgToAppend)
	ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, msgToAppend)

	return nil
}

func (ex *Executor) getDesiredNumberOfRooms(ctx context.Context, logger *zap.Logger, scheduler *entities.Scheduler) (int, error) {
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

func (ex *Executor) mapExistentAndNonExistentGameRooms(gameRoomIDs []string, instances []*game_room.Instance) ([]string, map[string]*game_room.Instance) {
	roomIdCountMap := make(map[string]int)
	nonexistentGameRoomsIDs := make([]string, 0)
	existentGameRoomsInstancesMap := make(map[string]*game_room.Instance)
	for _, gameRoomID := range gameRoomIDs {
		roomIdCountMap[gameRoomID]++
	}
	for _, instance := range instances {
		roomIdCountMap[instance.ID]++
	}

	for roomId, count := range roomIdCountMap {
		if count != 2 {
			nonexistentGameRoomsIDs = append(nonexistentGameRoomsIDs, roomId)
		}
	}

	for _, instance := range instances {
		if roomIdCountMap[instance.ID] == 2 {
			existentGameRoomsInstancesMap[instance.ID] = instance
		}
	}

	return nonexistentGameRoomsIDs, existentGameRoomsInstancesMap
}

func (ex *Executor) setTookAction(def *Definition, tookAction bool) {
	if def.TookAction != nil && *def.TookAction {
		return
	}
	def.TookAction = &tookAction
}

func (ex *Executor) canPerformDownscale(ctx context.Context, scheduler *entities.Scheduler, logger *zap.Logger) (bool, string) {
	can, err := ex.autoscaler.CanDownscale(ctx, scheduler)
	if err != nil {
		logger.Error("error checking if scheduler can downscale", zap.Error(err))
		return can, err.Error()
	}

	if !can {
		message := fmt.Sprintf("scheduler %s can't downscale, occupation is above the threshold", scheduler.Name)
		logger.Info(message)
		return false, message
	}

	cooldown := 0
	if scheduler.Autoscaling != nil {
		cooldown = scheduler.Autoscaling.Cooldown
	}
	cooldownDuration := time.Duration(cooldown) * time.Second
	waitingCooldown := scheduler.LastDownscaleAt.Add(cooldownDuration).After(time.Now().UTC())

	if can && waitingCooldown {
		message := fmt.Sprintf("scheduler %s can downscale, but cooldown period has not passed yet", scheduler.Name)
		logger.Info(message)
		return false, message
	}

	return can && !waitingCooldown, "ok"
}

func (ex *Executor) checkRollingUpdate(
	ctx context.Context,
	logger *zap.Logger,
	scheduler *entities.Scheduler,
	availableRoomsIDs []string,
) ([]string, bool) {
	logger.Debug("checking if system is in the middle of a rolling update of scheduler", zap.String("scheduler.Version", scheduler.Spec.Version))
	var roomsWithPreviousSchedulerVersion []string
	for _, roomID := range availableRoomsIDs {
		room, err := ex.roomStorage.GetRoom(ctx, scheduler.Name, roomID)
		if err == nil && room.Version != scheduler.Spec.Version {
			roomsWithPreviousSchedulerVersion = append(roomsWithPreviousSchedulerVersion, roomID)
		}
	}
	logger.Debug(
		"rooms that did not match current scheduler versions",
		zap.String("scheduler.Version", scheduler.Spec.Version),
		zap.Int("rooms", len(roomsWithPreviousSchedulerVersion)),
	)
	return roomsWithPreviousSchedulerVersion, len(roomsWithPreviousSchedulerVersion) != 0
}

func (ex *Executor) performRollingUpdate(
	ctx context.Context,
	op *operation.Operation,
	def *Definition,
	logger *zap.Logger,
	scheduler *entities.Scheduler,
	desiredNumberOfRooms int,
	availableRoomsIDs []string,
	roomsWithPreviousSchedulerVersion []string,
) error {
	logger.Info("performing rolling update", zap.String("scheduler.Version", scheduler.Spec.Version))
	maxSurgeAmount, err := ex.roomManager.SchedulerMaxSurge(ctx, scheduler)
	if err != nil {
		logger.Error("failed to perform rolling update while getting max surge amount of rooms", zap.Error(err))
		return err
	}
	maxRoomsToSurge := desiredNumberOfRooms + maxSurgeAmount - len(availableRoomsIDs)
	if len(roomsWithPreviousSchedulerVersion) < maxRoomsToSurge {
		maxRoomsToSurge = len(roomsWithPreviousSchedulerVersion)
	}
	if maxRoomsToSurge <= 0 {
		maxRoomsToSurge = 1
	}
	logger.Info(
		"upscaling new rooms",
		zap.Int("desired", desiredNumberOfRooms),
		zap.Int("maxSurgeAmount", maxSurgeAmount),
		zap.Int("available", len(availableRoomsIDs)),
		zap.Int("oldRooms", len(roomsWithPreviousSchedulerVersion)),
		zap.Int("maxRoomsToSurge", maxRoomsToSurge),
	)
	addOp, err := ex.operationManager.CreatePriorityOperation(ctx, op.SchedulerName, &add.Definition{
		Amount: int32(maxRoomsToSurge),
	})
	if err != nil {
		logger.Error("failed to enqueue add operation for rolling update", zap.Error(err))
		return err
	}
	msgToAppend := fmt.Sprintf("created operation (id: %s) to surge %v rooms.", addOp.ID, maxSurgeAmount)
	ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, msgToAppend)
	ex.setTookAction(def, true)

	roomsMarkedForDeletion, err := ex.markPreviousSchedulerRoomsForDeletion(
		ctx,
		logger,
		scheduler,
		roomsWithPreviousSchedulerVersion,
		desiredNumberOfRooms,
	)
	if err != nil {
		logger.Error("could not delete rooms with previous scheduler version", zap.Error(err))
		return err
	}
	if len(roomsMarkedForDeletion) <= 0 {
		return nil
	}
	removeOp, err := ex.operationManager.CreateOperation(ctx, op.SchedulerName, &remove.Definition{
		RoomsIDs: roomsMarkedForDeletion,
	})
	if err != nil {
		logger.Error("failed to enqueue remove operation for rolling update", zap.Error(err))
		return err
	}
	msgToAppend = fmt.Sprintf("created operation (id: %s) to remove rooms with previous scheduler version.", removeOp.ID)
	ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, msgToAppend)
	ex.setTookAction(def, true)

	return nil
}

func (ex *Executor) markPreviousSchedulerRoomsForDeletion(
	ctx context.Context,
	logger *zap.Logger,
	scheduler *entities.Scheduler,
	roomsWithPreviousSchedulerVersion []string,
	desiredNumberOfRooms int,
) ([]string, error) {
	logger.Debug(
		"ensuring only rooms with current scheduler version are running",
		zap.String("scheduler.Name", scheduler.Name),
		zap.String("scheduler.Version", scheduler.Spec.Version),
	)
	readyRoomIDs, err := ex.roomStorage.GetRoomIDsByStatus(ctx, scheduler.Name, game_room.GameStatusReady)
	if err != nil {
		return []string{}, fmt.Errorf("failed to list scheduler rooms on ready status: %w", err)
	}
	bufferRoomsToBeRemoved := len(readyRoomIDs) - desiredNumberOfRooms
	if bufferRoomsToBeRemoved < 0 {
		logger.Sugar().Debugf("can not delete old rooms without offending autoscale: %d", bufferRoomsToBeRemoved)
		return []string{}, nil
	}
	if bufferRoomsToBeRemoved > len(roomsWithPreviousSchedulerVersion) {
		return roomsWithPreviousSchedulerVersion, nil
	}
	return roomsWithPreviousSchedulerVersion[:bufferRoomsToBeRemoved], nil
}
