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

package update_scheduler

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

type UpdateSchedulerExecutor struct {
	roomManager        *room_manager.RoomManager
	schedulerManager   interfaces.SchedulerManager
	roomsBeingReplaced sync.Map
}

func NewExecutor(roomManager *room_manager.RoomManager, schedulerManager interfaces.SchedulerManager) *UpdateSchedulerExecutor {
	return &UpdateSchedulerExecutor{
		roomManager:      roomManager,
		schedulerManager: schedulerManager,
	}
}

// Execute the process of updating a scheduler consists of the following:
// 1. Update the scheduler configuration using the one present on the operation
//    definition;
// 2. If this update creates a minor version, the operation is done since there
//    is no necessity of replacing game rooms;
// 3. For a major version, Fetch the MaxSurge for the scheduler;
// 4. Creates "replace" goroutines (same number as MaxSurge);
// 5. Each goroutine will listen to a channel and create a new room using the
//    new configuration. After the room is ready, it will then delete the room
//    being replaced;
// 6. List all game rooms that need to be replaced and produce them into the
//    replace goroutines channel;
func (e *UpdateSchedulerExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	logger := zap.L().With(
		zap.String("scheduler_name", op.SchedulerName),
		zap.String("operation_definition", definition.Name()),
		zap.String("operation_id", op.ID),
	)
	logger.Debug("start updating scheduler")

	updateDefinition := definition.(*UpdateSchedulerDefinition)
	scheduler := &updateDefinition.NewScheduler
	isMajor, err := e.schedulerManager.UpdateSchedulerConfig(ctx, scheduler)
	if err != nil {
		return fmt.Errorf("failed to update scheduler configuration: %w", err)
	}

	// no major change, the operation is done.
	if !isMajor {
		return nil
	}

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
		rooms, err := e.roomManager.ListRoomsWithDeletionPriority(ctx, scheduler.Name, scheduler.Spec.Version, maxSurgeNum)
		if err != nil {
			return fmt.Errorf("failed to list rooms for deletion")
		}

		for _, room := range rooms {
			isRoomBeingReplaced, exists := e.roomsBeingReplaced.Load(room.ID)

			if !exists {
				e.roomsBeingReplaced.Store(room.ID, true)
			} else if isRoomBeingReplaced.(bool) {
				continue
			}

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

	logger.Debug("scheduler update finishes with success")
	return nil
}

func (e *UpdateSchedulerExecutor) OnError(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	return nil
}

func (e *UpdateSchedulerExecutor) Name() string {
	return OperationName
}

func (e *UpdateSchedulerExecutor) replaceRoom(logger *zap.Logger, wg *sync.WaitGroup, roomsChan chan *game_room.GameRoom, roomManager *room_manager.RoomManager, scheduler entities.Scheduler) {
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
			e.roomsBeingReplaced.Store(room.ID, false)
			continue
		}

		err = roomManager.DeleteRoom(ctx, room)
		if err != nil {
			// TODO(gabrielcorado): failing to delete the room can cause some
			// issues since this room could be replaced again. should we perform
			// a "force delete" or fail the update here?
			logger.Warn("failed to delete room", zap.Error(err))
			e.roomsBeingReplaced.Store(room.ID, false)
			continue
		}

		logger.Sugar().Debugf("replaced room \"%s\" with \"%s\"", room.ID, gameRoom.ID)
		e.roomsBeingReplaced.Store(room.ID, false)
	}
}
