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

package switchversion

import (
	"context"
	"fmt"
	"sync"

	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/operations/rooms/remove"
	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/avast/retry-go/v4"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Executor struct {
	roomManager         ports.RoomManager
	schedulerManager    ports.SchedulerManager
	operationManager    ports.OperationManager
	roomStorage         ports.RoomStorage
	roomsBeingReplaced  *sync.Map
	newCreatedRooms     map[string][]*game_room.GameRoom
	newCreatedRoomsLock sync.Mutex
}

var _ operations.Executor = (*Executor)(nil)

func NewExecutor(roomManager ports.RoomManager, schedulerManager ports.SchedulerManager, operationManager ports.OperationManager, roomStorage ports.RoomStorage) *Executor {
	// TODO(caio.rodrigues): change map to store a list of ids (less memory used)
	newCreatedRoomsMap := make(map[string][]*game_room.GameRoom)

	return &Executor{
		roomManager:         roomManager,
		schedulerManager:    schedulerManager,
		operationManager:    operationManager,
		roomStorage:         roomStorage,
		roomsBeingReplaced:  &sync.Map{},
		newCreatedRooms:     newCreatedRoomsMap,
		newCreatedRoomsLock: sync.Mutex{},
	}
}

// Execute the process of switching a scheduler active version consists of the following:
//  1. Creates "replace" goroutines (same number as MaxSurge);
//  2. Each goroutine will listen to a channel and create a new room using the
//     new configuration. After the room is ready, it will then delete the room
//     being replaced;
//  3. List all game rooms that need to be replaced and produce them into the
//     replace goroutines channel;
//  4. Switch the active version
func (ex *Executor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, definition.Name()),
		zap.String(logs.LogFieldOperationPhase, "Execute"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	logger.Info("start switching scheduler active version")

	updateDefinition, ok := definition.(*Definition)
	if !ok {
		return fmt.Errorf("the definition is invalid. Should be type Definition")
	}

	scheduler, err := ex.schedulerManager.GetSchedulerByVersion(ctx, op.SchedulerName, updateDefinition.NewActiveVersion)
	if err != nil {
		logger.Error("error fetching scheduler version to be switched to", zap.Error(err))
		getSchedulerErr := fmt.Errorf("error fetching scheduler version to be switched to: %w", err)
		ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, getSchedulerErr.Error())
		return getSchedulerErr
	}

	replacePods, err := ex.shouldReplacePods(ctx, scheduler)
	if err != nil {
		logger.Error("error deciding if should replace pods", zap.Error(err))
		shouldReplacePodsErr := fmt.Errorf("error deciding if should replace pods: %w", err)
		ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, shouldReplacePodsErr.Error())
		return shouldReplacePodsErr
	}

	if replacePods {
		maxSurgeNum, err := ex.roomManager.SchedulerMaxSurge(ctx, scheduler)
		if err != nil {
			logger.Error("error fetching scheduler max surge", zap.Error(err))
			maxSurgeErr := fmt.Errorf("error fetching scheduler max surge: %w", err)
			ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, maxSurgeErr.Error())
			return maxSurgeErr
		}

		err = ex.startReplaceRoomsLoop(ctx, logger, maxSurgeNum, *scheduler, op)
		if err != nil {
			logger.Error("error replacing rooms", zap.Error(err))
			replaceRoomsErr := fmt.Errorf("error replacing rooms: %w", err)
			ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, replaceRoomsErr.Error())
			return replaceRoomsErr
		}
	}

	logger.Sugar().Debugf("switching version to %v", scheduler.Spec.Version)
	scheduler.State = entities.StateInSync
	err = ex.schedulerManager.UpdateScheduler(ctx, scheduler)
	if err != nil {
		logger.Error("error updating scheduler with new active version")
		updateSchedulerErr := fmt.Errorf("error updating scheduler with new active version: %w", err)
		ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, updateSchedulerErr.Error())
		return updateSchedulerErr
	}

	ex.clearNewCreatedRooms(op.SchedulerName)
	logger.Info("scheduler update finishes with success")
	return nil
}

func (ex *Executor) Rollback(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, definition.Name()),
		zap.String(logs.LogFieldOperationPhase, "Rollback"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	logger.Info("starting Rollback routine")

	err := ex.deleteNewCreatedRooms(ctx, logger, op.SchedulerName, remove.SwitchVersionRollback)
	ex.clearNewCreatedRooms(op.SchedulerName)
	if err != nil {
		logger.Error("error deleting newly created rooms", zap.Error(err))
		deleteCreatedRoomsErr := fmt.Errorf("error rolling back created rooms: %w", err)
		ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, deleteCreatedRoomsErr.Error())
		return err
	}

	logger.Info("finished Rollback routine")
	return nil
}

func (ex *Executor) Name() string {
	return OperationName
}

func (ex *Executor) deleteNewCreatedRooms(ctx context.Context, logger *zap.Logger, schedulerName string, reason string) error {
	logger.Info("deleting created rooms since switching active version had error - start")
	for _, room := range ex.newCreatedRooms[schedulerName] {
		err := ex.roomManager.DeleteRoom(ctx, room, reason)
		if err != nil {
			logger.Error("failed to deleted recent created room", zap.Error(err))
			return err
		}
		logger.Sugar().Debugf("deleted room \"%s\" successfully", room.ID)
	}
	logger.Info("deleting created rooms since switching active version had error - end successfully")
	return nil
}

func (ex *Executor) startReplaceRoomsLoop(ctx context.Context, logger *zap.Logger, maxSurgeNum int, scheduler entities.Scheduler, op *operation.Operation) error {
	logger.Info("replacing rooms loop - start")
	roomsChan := make(chan *game_room.GameRoom)
	errs, ctx := errgroup.WithContext(ctx)

	var totalRoomsAmount int
	var err error
	err = retry.Do(func() error {
		totalRoomsAmount, err = ex.roomStorage.GetRoomCount(ctx, op.SchedulerName)
		return err
	}, retry.Attempts(10))
	if err != nil {
		return err
	}

	for i := 0; i < maxSurgeNum; i++ {
		errs.Go(func() error {
			return ex.replaceRoom(logger, roomsChan, ex.roomManager, scheduler)
		})
	}

roomsListLoop:
	for {
		var rooms []*game_room.GameRoom
		err = retry.Do(func() error {
			rooms, err = ex.roomManager.ListRoomsWithDeletionPriority(ctx, scheduler.Name, scheduler.Spec.Version, maxSurgeNum, ex.roomsBeingReplaced)
			return err
		}, retry.Attempts(10))

		if err != nil {
			return fmt.Errorf("failed to list rooms for deletion")
		}
		for _, room := range rooms {
			ex.roomsBeingReplaced.Store(room.ID, true)

			select {
			case roomsChan <- room:
			case <-ctx.Done():
				break roomsListLoop
			}
		}

		ex.reportOperationProgress(ctx, logger, totalRoomsAmount, op)

		if len(rooms) == 0 {
			break
		}
	}

	// close the rooms change and ensure all replace goroutines are gone
	close(roomsChan)

	// Wait for possible errors from goroutines
	if err := errs.Wait(); err != nil {
		return err
	}
	ex.reportOperationProgress(ctx, logger, totalRoomsAmount, op)
	logger.Info("replacing rooms loop - finish")
	return nil
}

func (ex *Executor) replaceRoom(logger *zap.Logger, roomsChan chan *game_room.GameRoom, roomManager ports.RoomManager, scheduler entities.Scheduler) error {

	// we're going to use a separated context for each replaceRoom since we
	// don't want to cancel the replacement in the middle (like creating a room and
	// then left the old one (without deleting it).
	ctx := context.Background()

	for {
		room, ok := <-roomsChan
		if !ok {
			return nil
		}

		gameRoom, _, err := roomManager.CreateRoom(ctx, scheduler, false)
		if err != nil {
			logger.Error("error creating room", zap.Error(err))
		}

		err = roomManager.DeleteRoom(ctx, room, remove.SwitchVersionReplace)
		if err != nil {
			logger.Warn("failed to delete room", zap.Error(err))
			ex.roomsBeingReplaced.Delete(room.ID)
			return err
		}

		ex.roomsBeingReplaced.Delete(room.ID)
		if gameRoom == nil {
			return nil
		}

		logger.Sugar().Debugf("replaced room \"%s\" with \"%s\"", room.ID, gameRoom.ID)
		ex.appendToNewCreatedRooms(scheduler.Name, gameRoom)
	}
}

func (ex *Executor) appendToNewCreatedRooms(schedulerName string, gameRoom *game_room.GameRoom) {
	ex.newCreatedRoomsLock.Lock()
	defer ex.newCreatedRoomsLock.Unlock()
	ex.newCreatedRooms[schedulerName] = append(ex.newCreatedRooms[schedulerName], gameRoom)
}

func (ex *Executor) clearNewCreatedRooms(schedulerName string) {
	delete(ex.newCreatedRooms, schedulerName)
}

func (ex *Executor) shouldReplacePods(ctx context.Context, newScheduler *entities.Scheduler) (bool, error) {
	actualActiveScheduler, err := ex.schedulerManager.GetActiveScheduler(ctx, newScheduler.Name)
	if err != nil {
		return false, err
	}
	return actualActiveScheduler.IsMajorVersion(newScheduler), nil
}

func (ex *Executor) reportOperationProgress(ctx context.Context, logger *zap.Logger, totalAmount int, op *operation.Operation) {
	if totalAmount == 0 {
		return
	}

	amountReplaced := ex.amountReplaced(op.SchedulerName)
	currentPercentageRate := 100 * amountReplaced / totalAmount

	msg := fmt.Sprintf("Conclusion: %v%%. Amount of rooms replaced: %v", currentPercentageRate, amountReplaced)
	logger.Debug(msg)
	ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, msg)
}

func (ex *Executor) amountReplaced(schedulerName string) int {
	amountReplaced := len(ex.newCreatedRooms[schedulerName])
	return amountReplaced
}
