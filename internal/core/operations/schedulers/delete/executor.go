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

package delete

import (
	"context"
	"errors"
	"fmt"
	"time"

	portsErrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	v1 "k8s.io/api/core/v1"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/operations"
	"go.uber.org/zap"
)

type Executor struct {
	schedulerStorage ports.SchedulerStorage
	schedulerCache   ports.SchedulerCache
	instanceStorage  ports.GameRoomInstanceStorage
	operationStorage ports.OperationStorage
	operationManager ports.OperationManager
	runtime          ports.Runtime
}

var _ operations.Executor = (*Executor)(nil)

func NewExecutor(
	schedulerStorage ports.SchedulerStorage,
	schedulerCache ports.SchedulerCache,
	instanceStorage ports.GameRoomInstanceStorage,
	operationStorage ports.OperationStorage,
	operationManager ports.OperationManager,
	runtime ports.Runtime,
) *Executor {
	return &Executor{
		schedulerStorage: schedulerStorage,
		schedulerCache:   schedulerCache,
		instanceStorage:  instanceStorage,
		operationStorage: operationStorage,
		operationManager: operationManager,
		runtime:          runtime,
	}
}

// Execute deletes the scheduler, cleaning all bounded resources in the runtime and on storages.
func (e *Executor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	logger := zap.L().With(
		zap.String(logs.LogFieldSchedulerName, op.SchedulerName),
		zap.String(logs.LogFieldOperationDefinition, op.DefinitionName),
		zap.String(logs.LogFieldOperationPhase, "Execute"),
		zap.String(logs.LogFieldOperationID, op.ID),
	)
	schedulerName := op.SchedulerName

	scheduler, err := e.getScheduler(ctx, schedulerName)

	if err != nil {
		logger.Error("error fetching scheduler for deletion", zap.Error(err))
		getSchedulerErr := fmt.Errorf("error fetching scheduler for deletion: %w", err)
		e.operationManager.AppendOperationEventToExecutionHistory(ctx, op, getSchedulerErr.Error())
		return getSchedulerErr
	}

	err = e.schedulerStorage.RunWithTransaction(ctx, func(transactionId ports.TransactionID) error {
		err = e.schedulerStorage.DeleteScheduler(ctx, transactionId, scheduler)
		if err != nil {
			if !errors.Is(err, portsErrors.ErrNotFound) {
				logger.Error("failed to delete scheduler from storage", zap.Error(err))
				return err
			}
			logger.Warn("scheduler not found on storage, will try to complete operation anyway", zap.Error(err))
		}

		err = e.runtime.DeleteScheduler(ctx, scheduler)
		if err != nil {
			if !errors.Is(err, portsErrors.ErrNotFound) {
				logger.Error("failed to delete scheduler from runtime", zap.Error(err))
				return err
			}
			logger.Warn("scheduler not found on runtime, will try to complete operation anyway", zap.Error(err))
		}

		err = e.waitForAllInstancesToBeDeleted(ctx, op, scheduler, logger)
		if err != nil {
			logger.Warn("failed to wait for instances to be deleted", zap.Error(err))
		}

		err = e.schedulerCache.DeleteScheduler(ctx, schedulerName)
		if err != nil {
			logger.Warn("failed to delete scheduler from cache", zap.Error(err))
		}

		err = e.operationStorage.CleanOperationsHistory(ctx, schedulerName)
		if err != nil {
			logger.Warn("failed to clean operations history", zap.Error(err))
		}

		return nil
	})

	if err != nil {
		logger.Error("error deleting scheduler in transaction", zap.Error(err))
		transactionErr := fmt.Errorf("error deleting scheduler: %w", err)
		e.operationManager.AppendOperationEventToExecutionHistory(ctx, op, transactionErr.Error())
		return transactionErr
	}

	return nil
}

// Rollback does nothing.
func (e *Executor) Rollback(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	return nil
}

// Name returns the name of the Operation.
func (e *Executor) Name() string {
	return OperationName
}

func (e *Executor) waitForAllInstancesToBeDeleted(ctx context.Context, op *operation.Operation, scheduler *entities.Scheduler, logger *zap.Logger) error {
	instancesCount, err := e.instanceStorage.GetInstanceCount(ctx, scheduler.Name)

	if err != nil {
		logger.Error("failed to get instance count", zap.Error(err))
		return err
	}
	if instancesCount == 0 {
		return nil
	}

	// TODO: the TerminationGracePeriod field should have validation enforcing it to be a positive number, or have
	// the default value we are using here.
	terminationGracePeriodSeconds := int(scheduler.Spec.TerminationGracePeriod.Seconds())
	if terminationGracePeriodSeconds == 0 {
		terminationGracePeriodSeconds = v1.DefaultTerminationGracePeriodSeconds
	}
	schedulerDeletionTimeout := time.Duration(terminationGracePeriodSeconds*instancesCount) * time.Second

	ticker := time.NewTicker(time.Duration(terminationGracePeriodSeconds) * time.Second)
	defer ticker.Stop()

	timeoutContext, cancelFunc := context.WithTimeout(ctx, schedulerDeletionTimeout)
	defer cancelFunc()

	for instancesCount != 0 {
		select {
		case <-timeoutContext.Done():
			logger.Error("timeout waiting for instances to be deleted", zap.Error(timeoutContext.Err()))
			return err
		case <-ticker.C:
			instancesCount, err = e.instanceStorage.GetInstanceCount(ctx, scheduler.Name)
			if err != nil {
				logger.Error("failed to get instance count", zap.Error(err))
				return err
			}
			e.operationManager.AppendOperationEventToExecutionHistory(ctx, op, fmt.Sprintf("Waiting for instances to be deleted: %d", instancesCount))
		}
	}
	return nil
}

func (e *Executor) getScheduler(ctx context.Context, schedulerName string) (*entities.Scheduler, error) {
	scheduler, err := e.schedulerCache.GetScheduler(ctx, schedulerName)
	if err != nil || scheduler == nil {
		scheduler, err = e.schedulerStorage.GetScheduler(ctx, schedulerName)
		if err != nil {
			return nil, err
		}
	}
	return scheduler, nil

}
