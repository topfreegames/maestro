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

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/services/interfaces"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
	"go.uber.org/zap"
)

var roomsSlice []*game_room.GameRoom

type SwitchActiveVersionExecutor struct {
	roomManager        *room_manager.RoomManager
	schedulerManager   interfaces.SchedulerManager
	roomsBeingReplaced *sync.Map
	newCreatedRooms    []*game_room.GameRoom
}

func NewExecutor(roomManager *room_manager.RoomManager, schedulerManager interfaces.SchedulerManager) *SwitchActiveVersionExecutor {
	return &SwitchActiveVersionExecutor{
		roomManager:        roomManager,
		schedulerManager:   schedulerManager,
		roomsBeingReplaced: &sync.Map{},
		newCreatedRooms:    roomsSlice,
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
func (e *SwitchActiveVersionExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	logger := zap.L().With(
		zap.String("scheduler_name", op.SchedulerName),
		zap.String("operation_definition", definition.Name()),
		zap.String("operation_id", op.ID),
	)
	logger.Debug("start switching scheduler active version")

	updateDefinition := definition.(*SwitchActiveVersionDefinition)
	scheduler := &updateDefinition.NewActiveScheduler

	maxSurgeNum, err := e.roomManager.SchedulerMaxSurge(ctx, scheduler)
	if err != nil {
		return fmt.Errorf("error fetching scheduler max surge: %w", err)
	}

	roomsChan := make(chan *game_room.GameRoom)
	var maxSurgeWg sync.WaitGroup

	maxSurgeWg.Add(maxSurgeNum)
	for i := 0; i < maxSurgeNum; i++ {
		go e.replaceRoom(logger, &maxSurgeWg, roomsChan, e.roomManager, *scheduler)
	}

roomsListLoop:
	for {
		rooms, err := e.roomManager.ListRoomsWithDeletionPriority(ctx, scheduler.Name, scheduler.Spec.Version, maxSurgeNum, e.roomsBeingReplaced)
		if err != nil {
			return fmt.Errorf("failed to list rooms for deletion")
		}

		for _, room := range rooms {
			e.roomsBeingReplaced.Store(room.ID, true)
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
	maxSurgeWg.Wait()

	err = e.schedulerManager.SwitchActiveScheduler(ctx, scheduler)
	if err != nil {
		logger.Error("Error switching active scheduler version on scheduler manager")
		return err
	}

	logger.Debug("scheduler update finishes with success")
	return nil
}

func (e *SwitchActiveVersionExecutor) OnError(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	logger := zap.L().With(
		zap.String("scheduler_name", op.SchedulerName),
		zap.String("operation_definition", definition.Name()),
		zap.String("operation_id", op.ID),
	)
	logger.Info("starting OnError routine")

	err := e.deleteNewCreatedRooms(ctx, logger)
	if err != nil {
		return err
	}

	logger.Debug("finished OnError routine")
	return nil
}

func (e *SwitchActiveVersionExecutor) Name() string {
	return OperationName
}

func (e *SwitchActiveVersionExecutor) replaceRoom(logger *zap.Logger, wg *sync.WaitGroup, roomsChan chan *game_room.GameRoom, roomManager *room_manager.RoomManager, scheduler entities.Scheduler) {
	defer wg.Done()

	// we're going to use a separated context for each replaceRoom since we
	// don't want to cancel the replace in the middle (like creating a room and
	// then left the old one (without deleting it).
	ctx := context.Background()

	for {
		room, ok := <-roomsChan
		if !ok {
			return
		}

		gameRoom, _, err := roomManager.CreateRoom(ctx, scheduler)
		if err != nil {
			// TODO(gabrielcorado): we need to know why the creation failed
			// to decide if we need to remove the newly created room or not.
			// This logic could be placed in a more "safe" RoomManager
			// CreateRoom function.
			e.roomsBeingReplaced.Delete(room.ID)
			continue
		}

		err = roomManager.DeleteRoom(ctx, room)
		if err != nil {
			// TODO(gabrielcorado): failing to delete the room can cause some
			// issues since this room could be replaced again. should we perform
			// a "force delete" or fail the update here?
			logger.Warn("failed to delete room", zap.Error(err))
			e.roomsBeingReplaced.Delete(room.ID)
			continue
		}

		logger.Sugar().Debugf("replaced room \"%s\" with \"%s\"", room.ID, gameRoom.ID)
		e.roomsBeingReplaced.Delete(room.ID)
		e.newCreatedRooms = append(e.newCreatedRooms, gameRoom)
	}
}

func (e *SwitchActiveVersionExecutor) deleteNewCreatedRooms(ctx context.Context, logger *zap.Logger) error {
	logger.Debug("deleting created rooms since switching active version had error - start")
	for _, room := range e.newCreatedRooms {
		err := e.roomManager.DeleteRoom(ctx, room)
		if err != nil {
			logger.Error("failed to deleted recent created room", zap.Error(err))
			return err
		}
		logger.Sugar().Debugf("deleted room \"%s\" successfully", room.ID)
	}
	logger.Debug("deleting created rooms since switching active version had error - end successfully")
	return nil
}
