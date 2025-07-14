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

package add

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/logs"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities"
	"golang.org/x/sync/errgroup"

	"go.uber.org/zap"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
)

const DefaultAmountLimit = 1000

type Config struct {
	AmountLimit int32
}

type Executor struct {
	roomManager      ports.RoomManager
	storage          ports.SchedulerStorage
	operationManager ports.OperationManager
	config           Config
}

var _ operations.Executor = (*Executor)(nil)

func NewExecutor(roomManager ports.RoomManager, storage ports.SchedulerStorage, operationManager ports.OperationManager, config Config) *Executor {
	if config.AmountLimit <= 0 {
		zap.L().Sugar().Infof("Add Executor - Amount limit wrongly configured with %d, using default value %d", config.AmountLimit, DefaultAmountLimit)
		config.AmountLimit = DefaultAmountLimit
	}
	return &Executor{
		roomManager,
		storage,
		operationManager,
		config,
	}
}

func (ex *Executor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	executionLogger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, definition.Name()),
		zap.String(logs.LogFieldOperationPhase, "Execute"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	amount := definition.(*Definition).Amount
	scheduler, err := ex.storage.GetScheduler(ctx, op.SchedulerName)
	if err != nil {
		executionLogger.Error("error fetching scheduler from storage, can not create rooms", zap.Error(err))
		getSchedulerStorageErr := fmt.Errorf("error fetching scheduler from storage: %w", err)
		ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, getSchedulerStorageErr.Error())
		return getSchedulerStorageErr
	}

	if amount > ex.config.AmountLimit {
		executionLogger.Info("operation called with amount greater than limit, capping it", zap.Int32("calledAmount", amount), zap.Int32("limit", ex.config.AmountLimit))
		amount = ex.config.AmountLimit
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
		ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, executionErr.Error())
		return executionErr
	}

	ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, fmt.Sprintf("added %d rooms", amount))
	executionLogger.Info("finished adding rooms")
	return nil
}

func (ex *Executor) Rollback(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	return nil
}

func (ex *Executor) Name() string {
	return OperationName
}

func (ex *Executor) createRoom(ctx context.Context, scheduler *entities.Scheduler, logger *zap.Logger) error {
	_, _, err := ex.roomManager.CreateRoom(ctx, *scheduler, false)
	if err != nil {
		logger.Error("Error while creating room", zap.Error(err))
		reportAddRoomOperationExecutionFailed(scheduler.Name, ex.Name())
		return fmt.Errorf("error while creating room: %w", err)
	}

	return nil
}
