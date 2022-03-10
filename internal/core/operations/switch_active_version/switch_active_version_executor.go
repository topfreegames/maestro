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

package switch_active_version

import (
	"context"
	"fmt"
	"sync"

	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type SwitchActiveVersionExecutor struct {
	roomManager         ports.RoomManager
	schedulerManager    ports.SchedulerManager
	roomsBeingReplaced  *sync.Map
	newCreatedRooms     map[string][]*game_room.GameRoom
	newCreatedRoomsLock sync.Mutex
}

func NewExecutor(roomManager ports.RoomManager, schedulerManager ports.SchedulerManager) *SwitchActiveVersionExecutor {
	// TODO(caio.rodrigues): change map to store a list of ids (less memory used)
	newCreatedRoomsMap := make(map[string][]*game_room.GameRoom)

	return &SwitchActiveVersionExecutor{
		roomManager:         roomManager,
		schedulerManager:    schedulerManager,
		roomsBeingReplaced:  &sync.Map{},
		newCreatedRooms:     newCreatedRoomsMap,
		newCreatedRoomsLock: sync.Mutex{},
	}
}

// Execute the process of switching a scheduler active version consists of the following:
// 1. Creates "replace" goroutines (same number as MaxSurge);
// 2. Each goroutine will listen to a channel and create a new room using the
//    new configuration. After the room is ready, it will then delete the room
//    being replaced;
// 3. List all game rooms that need to be replaced and produce them into the
//    replace goroutines channel;
// 4. Switch the active version
func (ex *SwitchActiveVersionExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, definition.Name()),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	logger.Debug("start switching scheduler active version")

	updateDefinition, ok := definition.(*SwitchActiveVersionDefinition)
	if !ok {
		return fmt.Errorf("the definition is invalid. Should be type SwitchActiveVersionDefinition")
	}
	scheduler := &updateDefinition.NewActiveScheduler
	replacePods := updateDefinition.ReplacePods

	if replacePods {
		maxSurgeNum, err := ex.roomManager.SchedulerMaxSurge(ctx, scheduler)
		if err != nil {
			return fmt.Errorf("error fetching scheduler max surge: %w", err)
		}

		err = ex.startReplaceRoomsLoop(ctx, logger, maxSurgeNum, *scheduler)
		if err != nil {
			logger.Sugar().Errorf("replace rooms failed for scheduler \"%v\" with error \"%v\"", scheduler.Name, zap.Error(err))
			return err
		}
	}

	logger.Sugar().Debugf("switching version to %v", scheduler.Spec.Version)
	err := ex.schedulerManager.UpdateScheduler(ctx, scheduler)
	if err != nil {
		logger.Error("Error switching active scheduler version on scheduler manager")
		return err
	}

	ex.clearNewCreatedRooms(op.SchedulerName)
	logger.Debug("scheduler update finishes with success")
	return nil
}

func (ex *SwitchActiveVersionExecutor) OnError(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, definition.Name()),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	logger.Info("starting OnError routine")

	err := ex.deleteNewCreatedRooms(ctx, logger, op.SchedulerName)
	ex.clearNewCreatedRooms(op.SchedulerName)
	if err != nil {
		return err
	}

	logger.Debug("finished OnError routine")
	return nil
}

func (ex *SwitchActiveVersionExecutor) Name() string {
	return OperationName
}

func (ex *SwitchActiveVersionExecutor) deleteNewCreatedRooms(ctx context.Context, logger *zap.Logger, schedulerName string) error {
	logger.Debug("deleting created rooms since switching active version had error - start")
	for _, room := range ex.newCreatedRooms[schedulerName] {
		err := ex.roomManager.DeleteRoomAndWaitForRoomTerminated(ctx, room)
		if err != nil {
			logger.Error("failed to deleted recent created room", zap.Error(err))
			return err
		}
		logger.Sugar().Debugf("deleted room \"%s\" successfully", room.ID)
	}
	logger.Debug("deleting created rooms since switching active version had error - end successfully")
	return nil
}

func (ex *SwitchActiveVersionExecutor) startReplaceRoomsLoop(ctx context.Context, logger *zap.Logger, maxSurgeNum int, scheduler entities.Scheduler) error {
	logger.Debug("replacing rooms loop - start")
	roomsChan := make(chan *game_room.GameRoom)
	errs, ctx := errgroup.WithContext(ctx)

	logger.Sugar().Debugf("maxSurgeNum %d", maxSurgeNum)

	for i := 0; i < maxSurgeNum; i++ {
		errs.Go(func() error {
			return ex.replaceRoom(logger, roomsChan, ex.roomManager, scheduler)
		})
	}

roomsListLoop:
	for {
		logger.Sugar().Debug("listing schedulers to replace ", zap.String("version", scheduler.Spec.Version))
		rooms, err := ex.roomManager.ListRoomsWithDeletionPriority(ctx, scheduler.Name, scheduler.Spec.Version, maxSurgeNum, ex.roomsBeingReplaced)
		logger.Sugar().Debug("rooms to replace:", zap.Any("rooms", rooms))
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
	logger.Debug("replacing rooms loop - finish")
	return nil
}

func (ex *SwitchActiveVersionExecutor) replaceRoom(logger *zap.Logger, roomsChan chan *game_room.GameRoom, roomManager ports.RoomManager, scheduler entities.Scheduler) error {

	// we're going to use a separated context for each replaceRoom since we
	// don't want to cancel the replacement in the middle (like creating a room and
	// then left the old one (without deleting it).
	ctx := context.Background()

	for {
		room, ok := <-roomsChan
		if !ok {
			return nil
		}

		gameRoom, _, err := roomManager.CreateRoomAndWaitForReadiness(ctx, scheduler, false)
		if err != nil {
			logger.Error("error creating room", zap.Error(err))
			ex.roomsBeingReplaced.Delete(room.ID)
			return err
		}

		err = roomManager.DeleteRoomAndWaitForRoomTerminated(ctx, room)
		if err != nil {
			logger.Warn("failed to delete room", zap.Error(err))
			ex.roomsBeingReplaced.Delete(room.ID)
			return err
		}

		logger.Sugar().Debugf("replaced room \"%s\" with \"%s\"", room.ID, gameRoom.ID)
		ex.roomsBeingReplaced.Delete(room.ID)
		ex.appendToNewCreatedRooms(scheduler.Name, gameRoom)
	}
}

func (ex *SwitchActiveVersionExecutor) appendToNewCreatedRooms(schedulerName string, gameRoom *game_room.GameRoom) {
	ex.newCreatedRoomsLock.Lock()
	ex.newCreatedRooms[schedulerName] = append(ex.newCreatedRooms[schedulerName], gameRoom)
	ex.newCreatedRoomsLock.Unlock()
}

func (ex *SwitchActiveVersionExecutor) clearNewCreatedRooms(schedulerName string) {
	delete(ex.newCreatedRooms, schedulerName)
}
