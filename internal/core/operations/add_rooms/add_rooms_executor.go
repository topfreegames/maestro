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
	"errors"
	"fmt"
	"sync"

	"github.com/topfreegames/maestro/internal/core/logs"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"golang.org/x/sync/errgroup"

	"go.uber.org/zap"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	serviceerrors "github.com/topfreegames/maestro/internal/core/services/errors"
)

type AddRoomsExecutor struct {
	roomManager         ports.RoomManager
	storage             ports.SchedulerStorage
	logger              *zap.Logger
	newCreatedRooms     map[string][]*game_room.GameRoom
	newCreatedRoomsLock sync.Mutex
}

var _ operations.Executor = (*AddRoomsExecutor)(nil)

func NewExecutor(roomManager ports.RoomManager, storage ports.SchedulerStorage) *AddRoomsExecutor {
	return &AddRoomsExecutor{
		roomManager:         roomManager,
		storage:             storage,
		logger:              zap.L().With(zap.String(logs.LogFieldServiceName, "worker")),
		newCreatedRooms:     map[string][]*game_room.GameRoom{},
		newCreatedRoomsLock: sync.Mutex{},
	}
}

func (ex *AddRoomsExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) operations.ExecutionError {
	executionLogger := ex.logger.With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, definition.Name()),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	amount := definition.(*AddRoomsDefinition).Amount
	scheduler, err := ex.storage.GetScheduler(ctx, op.SchedulerName)
	if err != nil {
		executionLogger.Error(fmt.Sprintf("Could not find scheduler with name %s, can not create rooms", op.SchedulerName), zap.Error(err))
		return operations.NewErrUnexpected(err)
	}

	errGroup, errContext := errgroup.WithContext(ctx)
	executionLogger.Info("start adding rooms", zap.Int32("amount", amount))
	for i := int32(1); i <= amount; i++ {
		errGroup.Go(func() error {
			err := ex.createRoom(errContext, scheduler, executionLogger)
			return err
		})
	}

	if executionErr := errGroup.Wait(); executionErr != nil {
		executionLogger.Error("Error creating rooms", zap.Error(executionErr))
		if errors.Is(executionErr, serviceerrors.ErrGameRoomStatusWaitingTimeout) {
			return operations.NewErrReadyPingTimeout(executionErr)
		} else {
			return operations.NewErrUnexpected(executionErr)
		}
	}
	ex.clearNewCreatedRooms(op.SchedulerName)

	executionLogger.Info("finished adding rooms")
	return nil
}

func (ex *AddRoomsExecutor) Rollback(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr operations.ExecutionError) error {
	executionLogger := ex.logger.With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, definition.Name()),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	executionLogger.Info("starting rollback routine")

	err := ex.deleteNewCreatedRooms(ctx, executionLogger, op.SchedulerName)
	ex.clearNewCreatedRooms(op.SchedulerName)
	if err != nil {
		return err
	}
	executionLogger.Info("finished rollback routine")
	return nil
}

func (ex *AddRoomsExecutor) Name() string {
	return OperationName
}

func (ex *AddRoomsExecutor) createRoom(ctx context.Context, scheduler *entities.Scheduler, logger *zap.Logger) error {
	gameRoom, _, err := ex.roomManager.CreateRoomAndWaitForReadiness(ctx, *scheduler, false)
	if err != nil {
		logger.Error("Error while creating room", zap.Error(err))
		reportAddRoomOperationExecutionFailed(scheduler.Name, ex.Name())
		return fmt.Errorf("error while creating room: %w", err)
	}
	ex.appendToNewCreatedRooms(scheduler.Name, gameRoom)

	return nil
}

func (ex *AddRoomsExecutor) deleteNewCreatedRooms(ctx context.Context, logger *zap.Logger, schedulerName string) error {
	logger.Info("deleting created rooms since add rooms operation had error - start")
	for _, room := range ex.newCreatedRooms[schedulerName] {
		err := ex.roomManager.DeleteRoomAndWaitForRoomTerminated(ctx, room)
		if err != nil {
			logger.Error("failed to deleted recent created room", zap.Error(err))
			return err
		}
		logger.Sugar().Debugf("deleted room \"%s\" successfully", room.ID)
	}
	logger.Info("deleting created rooms since add rooms operation had error - end successfully")
	return nil
}

func (ex *AddRoomsExecutor) appendToNewCreatedRooms(schedulerName string, gameRoom *game_room.GameRoom) {
	ex.newCreatedRoomsLock.Lock()
	defer ex.newCreatedRoomsLock.Unlock()
	ex.newCreatedRooms[schedulerName] = append(ex.newCreatedRooms[schedulerName], gameRoom)
}

func (ex *AddRoomsExecutor) clearNewCreatedRooms(schedulerName string) {
	delete(ex.newCreatedRooms, schedulerName)
}
