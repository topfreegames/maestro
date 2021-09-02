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

package add_rooms

import (
	"context"
	"fmt"
	"sync"

	"github.com/topfreegames/maestro/internal/core/entities"

	"go.uber.org/zap"

	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
)

type AddRoomsExecutor struct {
	roomManager *room_manager.RoomManager
	storage     ports.SchedulerStorage
}

func NewExecutor(roomManager *room_manager.RoomManager, storage ports.SchedulerStorage) *AddRoomsExecutor {
	return &AddRoomsExecutor{
		roomManager: roomManager,
		storage:     storage,
	}
}

func (ae *AddRoomsExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition, logger *zap.Logger) error {
	amount := definition.(*AddRoomsDefinition).Amount
	scheduler, err := ae.storage.GetScheduler(ctx, op.SchedulerName)
	if err != nil {
		logger.Error(fmt.Sprintf("Could not find scheduler with name %s, can not create rooms", op.SchedulerName), zap.Error(err))
		return err
	}
	var waitGroup sync.WaitGroup

	for i := int32(0); i < amount; i++ {
		waitGroup.Add(1)
		go ae.createRoom(ctx, i, scheduler, logger, &waitGroup)
	}
	// TODO: we can not block here, we should be listening ctx.Done() somehow
	waitGroup.Wait()
	return nil
}

func (ae *AddRoomsExecutor) OnError(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error, logger *zap.Logger) error {
	return nil
}

func (ae *AddRoomsExecutor) Name() string {
	return OperationName
}

func (ae *AddRoomsExecutor) createRoom(ctx context.Context, index int32, scheduler *entities.Scheduler, logger *zap.Logger, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	_, _, err := ae.roomManager.CreateRoom(ctx, *scheduler)
	if err != nil {
		logger.Error(fmt.Sprintf("Error while creating room number %d", index), zap.Error(err))
		reportAddRoomOperationExecutionFailed(scheduler.Name, ae.Name())
	}
}
