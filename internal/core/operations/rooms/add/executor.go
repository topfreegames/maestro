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
	"sync"
	"time"

	"github.com/topfreegames/maestro/internal/core/logs"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities"

	"go.uber.org/zap"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
)

const (
	DefaultAmountLimit      = 1000
	DefaultBatchSize        = 40
	DefaultOperationTimeout = 5 * time.Second
)

type Config struct {
	AmountLimit      int32
	BatchSize        int32
	OperationTimeout time.Duration
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
	if config.BatchSize <= 0 {
		zap.L().Sugar().Infof("Add Executor - Batch size wrongly configured with %d, using default value %d", config.BatchSize, DefaultBatchSize)
		config.BatchSize = DefaultBatchSize
	}
	if config.OperationTimeout <= 0 {
		zap.L().Sugar().Infof("Add Executor - Operation timeout wrongly configured with %v, using default value %v", config.OperationTimeout, DefaultOperationTimeout)
		config.OperationTimeout = DefaultOperationTimeout
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
		executionLogger.Debug("operation called with amount greater than limit, capping it", zap.Int32("calledAmount", amount), zap.Int32("limit", ex.config.AmountLimit))
		amount = ex.config.AmountLimit
	}

	executionLogger.Debug("starting batched room creation",
		zap.Int32("totalAmount", amount),
		zap.Int32("batchSize", ex.config.BatchSize))

	var collectedErrors []error
	remainingAmount := amount

	for remainingAmount > 0 {
		batchSize := min(remainingAmount, ex.config.BatchSize)

		executionLogger.Debug("processing batch",
			zap.Int32("batchSize", batchSize),
			zap.Int32("remainingAmount", remainingAmount))

		var wg sync.WaitGroup
		errsCh := make(chan error, batchSize)

		for range batchSize {
			wg.Add(1)
			go func() {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(ctx, ex.config.OperationTimeout)
				defer cancel()

				errsCh <- ex.createRoom(ctx, scheduler, executionLogger)
			}()
		}

		wg.Wait()
		close(errsCh)

		for err := range errsCh {
			if err != nil {
				collectedErrors = append(collectedErrors, err)
			}
		}

		remainingAmount -= batchSize
		executionLogger.Debug("batch completed",
			zap.Int32("batchSize", batchSize),
			zap.Int32("remainingAmount", remainingAmount))
	}

	errCount := int32(len(collectedErrors))
	successCount := amount - errCount

	switch {
	case errCount > successCount:
		ErrMajorityRooms := fmt.Errorf("more rooms failed than succeeded, errors: %d and successes: %d of amount: %d", errCount, successCount, amount)

		executionLogger.Error(ErrMajorityRooms.Error(),
			zap.Error(ErrMajorityRooms),
			zap.Int32("failedRoomsCount", errCount),
			zap.Int32("successRoomsCount", successCount),
			zap.Any("allErrors", collectedErrors))

		ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, ErrMajorityRooms.Error())
		return ErrMajorityRooms
	default:
		executionLogger.Sugar().Infof("added rooms successfully with errors: %d and success: %d of amount: %d", errCount, successCount, amount)
		ex.operationManager.AppendOperationEventToExecutionHistory(ctx, op, fmt.Sprintf("added %d rooms", amount))
		return nil
	}
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
