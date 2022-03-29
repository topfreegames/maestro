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

//go:build unit
// +build unit

package operation_execution_worker

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/ports/mock"
	"gotest.tools/assert"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	mockoperation "github.com/topfreegames/maestro/internal/core/operations/mock"
	"github.com/topfreegames/maestro/internal/core/workers"
)

func TestSchedulerOperationsExecutionLoop(t *testing.T) {
	t.Run("successfully runs a single operation", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationName := "test_operation"
		operationDefinition := mockoperation.NewMockDefinition(mockCtrl)
		operationExecutor := mockoperation.NewMockExecutor(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		operationExecutor.EXPECT().Name().Return(operationName).AnyTimes()
		operationDefinition.EXPECT().Name().Return(operationName).AnyTimes()

		defFunc := func() operations.Definition { return operationDefinition }
		definitionConstructors := operations.NewDefinitionConstructors()
		definitionConstructors[operationName] = defFunc

		scheduler := &entities.Scheduler{Name: "random-scheduler"}
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  scheduler.Name,
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		executors := map[string]operations.Executor{}
		executors[operationName] = operationExecutor
		workerService := NewOperationExecutionWorker(scheduler, workers.ProvideWorkerOptions(operationManager, executors, nil, nil))

		operationManager.EXPECT().NextSchedulerOperation(gomock.Any(), expectedOperation.SchedulerName).Return(expectedOperation, operationDefinition, nil)
		operationDefinition.EXPECT().ShouldExecute(gomock.Any(), []*operation.Operation{}).Return(true)
		operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), expectedOperation, "Starting operation")
		operationManager.EXPECT().GrantLease(gomock.Any(), expectedOperation)
		operationManager.EXPECT().StartOperation(gomock.Any(), expectedOperation, gomock.Any())
		operationManager.EXPECT().StartLeaseRenewGoRoutine(gomock.Any(), expectedOperation)
		operationManager.EXPECT().FinishOperation(gomock.Any(), expectedOperation)
		operationManager.EXPECT().RevokeLease(gomock.Any(), expectedOperation)
		operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), expectedOperation, "Finishing operation")
		// Ends the worker by cancelling it
		operationManager.EXPECT().NextSchedulerOperation(gomock.Any(), expectedOperation.SchedulerName).Return(nil, nil, context.Canceled)

		operationExecutor.EXPECT().Execute(gomock.Any(), expectedOperation, operationDefinition).
			Do(func(ctx, operation, definition interface{}) {
				time.Sleep(time.Second * 1)
			}).
			Return(nil)

		err := workerService.Start(context.Background())
		require.NoError(t, err)

		workerService.Stop(context.Background())
		require.False(t, workerService.IsRunning())
	})

	t.Run("execute Rollback when a Execute fails", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationName := "test_operation"
		operationDefinition := mockoperation.NewMockDefinition(mockCtrl)
		operationExecutor := mockoperation.NewMockExecutor(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		operationExecutor.EXPECT().Name().Return(operationName).AnyTimes()
		operationDefinition.EXPECT().Name().Return(operationName).AnyTimes()

		defFunc := func() operations.Definition { return operationDefinition }
		definitionConstructors := operations.NewDefinitionConstructors()
		definitionConstructors[operationName] = defFunc

		scheduler := &entities.Scheduler{Name: "random-scheduler"}
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  scheduler.Name,
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		executors := map[string]operations.Executor{}
		executors[operationName] = operationExecutor
		workerService := NewOperationExecutionWorker(scheduler, workers.ProvideWorkerOptions(operationManager, executors, nil, nil))

		operationManager.EXPECT().NextSchedulerOperation(gomock.Any(), expectedOperation.SchedulerName).Return(expectedOperation, operationDefinition, nil)
		operationDefinition.EXPECT().ShouldExecute(gomock.Any(), []*operation.Operation{}).Return(true)
		operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), expectedOperation, "Starting operation")
		operationManager.EXPECT().GrantLease(gomock.Any(), expectedOperation)
		operationManager.EXPECT().StartOperation(gomock.Any(), expectedOperation, gomock.Any())
		operationManager.EXPECT().StartLeaseRenewGoRoutine(gomock.Any(), expectedOperation)

		executionErr := fmt.Errorf("some execution error")
		operationExecutor.EXPECT().Execute(gomock.Any(), expectedOperation, operationDefinition).Return(executionErr)
		operationExecutor.EXPECT().Rollback(gomock.Any(), expectedOperation, operationDefinition, executionErr).Do(
			func(ctx, operation, definition, executeErr interface{}) {
				time.Sleep(time.Second * 1)
			},
		).Return(nil)
		operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), expectedOperation, "Operation execution failed, reason: some execution error")
		operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), expectedOperation, "Operation Rollback flow execution finished with success")

		operationManager.EXPECT().FinishOperation(gomock.Any(), expectedOperation)
		operationManager.EXPECT().RevokeLease(gomock.Any(), expectedOperation)
		operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), expectedOperation, "Finishing operation")
		// Ends the worker by cancelling it
		operationManager.EXPECT().NextSchedulerOperation(gomock.Any(), expectedOperation.SchedulerName).Return(nil, nil, context.Canceled)

		err := workerService.Start(context.Background())
		require.NoError(t, err)

		workerService.Stop(context.Background())
		require.False(t, workerService.IsRunning())
	})

	t.Run("evict operation if there is no executor", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationName := "test_operation"
		operationDefinition := mockoperation.NewMockDefinition(mockCtrl)
		operationDefinition.EXPECT().Name().Return(operationName).AnyTimes()

		defFunc := func() operations.Definition { return operationDefinition }
		definitionConstructors := operations.NewDefinitionConstructors()
		definitionConstructors[operationName] = defFunc

		scheduler := &entities.Scheduler{Name: "random-scheduler"}
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  scheduler.Name,
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		executors := map[string]operations.Executor{}
		workerService := NewOperationExecutionWorker(scheduler, workers.ProvideWorkerOptions(operationManager, executors, nil, nil))

		operationManager.EXPECT().NextSchedulerOperation(gomock.Any(), expectedOperation.SchedulerName).Return(expectedOperation, operationDefinition, nil)
		operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), expectedOperation, "Operation evicted")
		operationManager.EXPECT().FinishOperation(gomock.Any(), expectedOperation)
		// Ends the worker by cancelling it
		operationManager.EXPECT().NextSchedulerOperation(gomock.Any(), expectedOperation.SchedulerName).Return(nil, nil, context.Canceled)

		err := workerService.Start(context.Background())
		require.NoError(t, err)
	})

	t.Run("evict operation if ShouldExecute returns false", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationName := "test_operation"
		operationDefinition := mockoperation.NewMockDefinition(mockCtrl)
		operationExecutor := mockoperation.NewMockExecutor(mockCtrl)
		operationExecutor.EXPECT().Name().Return(operationName).AnyTimes()
		operationDefinition.EXPECT().Name().Return(operationName).AnyTimes()

		defFunc := func() operations.Definition { return operationDefinition }
		definitionConstructors := operations.NewDefinitionConstructors()
		definitionConstructors[operationName] = defFunc

		scheduler := &entities.Scheduler{Name: "random-scheduler"}
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  scheduler.Name,
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		executors := map[string]operations.Executor{}
		executors[operationName] = operationExecutor
		workerService := NewOperationExecutionWorker(scheduler, workers.ProvideWorkerOptions(operationManager, executors, nil, nil))

		operationDefinition.EXPECT().ShouldExecute(gomock.Any(), []*operation.Operation{}).Return(false)

		operationManager.EXPECT().NextSchedulerOperation(gomock.Any(), expectedOperation.SchedulerName).Return(expectedOperation, operationDefinition, nil)
		operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), expectedOperation, "Operation evicted")
		operationManager.EXPECT().FinishOperation(gomock.Any(), expectedOperation)
		// Ends the worker by cancelling it
		operationManager.EXPECT().NextSchedulerOperation(gomock.Any(), expectedOperation.SchedulerName).Return(nil, nil, context.Canceled)

		err := workerService.Start(context.Background())
		require.NoError(t, err)

		workerService.Stop(context.Background())
		require.False(t, workerService.IsRunning())
	})

	t.Run("error starting operation should stop execution of operation and set status as error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationName := "test_operation"
		operationDefinition := mockoperation.NewMockDefinition(mockCtrl)
		operationExecutor := mockoperation.NewMockExecutor(mockCtrl)
		operationExecutor.EXPECT().Name().Return(operationName).AnyTimes()
		operationDefinition.EXPECT().Name().Return(operationName).AnyTimes()

		defFunc := func() operations.Definition { return operationDefinition }
		definitionConstructors := operations.NewDefinitionConstructors()
		definitionConstructors[operationName] = defFunc

		scheduler := &entities.Scheduler{Name: "random-scheduler"}
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  scheduler.Name,
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		executors := map[string]operations.Executor{}
		executors[operationName] = operationExecutor
		workerService := NewOperationExecutionWorker(scheduler, workers.ProvideWorkerOptions(operationManager, executors, nil, nil))

		operationDefinition.EXPECT().ShouldExecute(gomock.Any(), []*operation.Operation{}).Return(true)

		operationManager.EXPECT().NextSchedulerOperation(gomock.Any(), expectedOperation.SchedulerName).Return(expectedOperation, operationDefinition, nil)
		operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), expectedOperation, "Starting operation")
		operationManager.EXPECT().GrantLease(gomock.Any(), expectedOperation).Return(nil)
		operationManager.EXPECT().StartOperation(gomock.Any(), expectedOperation, gomock.Any()).Return(errors.New("some error starting operation"))
		operationManager.EXPECT().FinishOperation(gomock.Any(), expectedOperation)
		operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), expectedOperation, "Failed to start operation, reason: some error starting operation")
		// Ends the worker by cancelling it

		err := workerService.Start(context.Background())
		require.Error(t, err)

		assert.Equal(t, expectedOperation.Status, operation.StatusError)

		workerService.Stop(context.Background())
		require.False(t, workerService.IsRunning())
	})

	t.Run("error granting lease should stop execution of operation and set status as error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationName := "test_operation"
		operationDefinition := mockoperation.NewMockDefinition(mockCtrl)
		operationExecutor := mockoperation.NewMockExecutor(mockCtrl)
		operationExecutor.EXPECT().Name().Return(operationName).AnyTimes()
		operationDefinition.EXPECT().Name().Return(operationName).AnyTimes()

		defFunc := func() operations.Definition { return operationDefinition }
		definitionConstructors := operations.NewDefinitionConstructors()
		definitionConstructors[operationName] = defFunc

		scheduler := &entities.Scheduler{Name: "random-scheduler"}
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  scheduler.Name,
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		executors := map[string]operations.Executor{}
		executors[operationName] = operationExecutor
		workerService := NewOperationExecutionWorker(scheduler, workers.ProvideWorkerOptions(operationManager, executors, nil, nil))

		operationManager.EXPECT().NextSchedulerOperation(gomock.Any(), expectedOperation.SchedulerName).Return(expectedOperation, operationDefinition, nil)
		operationDefinition.EXPECT().ShouldExecute(gomock.Any(), []*operation.Operation{}).Return(true)
		operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), expectedOperation, "Starting operation")
		operationManager.EXPECT().GrantLease(gomock.Any(), expectedOperation).Return(errors.New("some error granting lease"))
		operationManager.EXPECT().FinishOperation(gomock.Any(), expectedOperation)
		operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), expectedOperation, "Failed to grant lease to operation, reason: some error granting lease")
		// Ends the worker by cancelling it

		err := workerService.Start(context.Background())
		require.Error(t, err)

		assert.Equal(t, expectedOperation.Status, operation.StatusError)

		workerService.Stop(context.Background())
		require.False(t, workerService.IsRunning())
	})

	t.Run("error getting next operation should stop execution of operation", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationName := "test_operation"
		operationDefinition := mockoperation.NewMockDefinition(mockCtrl)
		operationExecutor := mockoperation.NewMockExecutor(mockCtrl)
		operationExecutor.EXPECT().Name().Return(operationName).AnyTimes()
		operationDefinition.EXPECT().Name().Return(operationName).AnyTimes()

		defFunc := func() operations.Definition { return operationDefinition }
		definitionConstructors := operations.NewDefinitionConstructors()
		definitionConstructors[operationName] = defFunc

		scheduler := &entities.Scheduler{Name: "random-scheduler"}
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  scheduler.Name,
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		executors := map[string]operations.Executor{}
		executors[operationName] = operationExecutor

		workerService := NewOperationExecutionWorker(scheduler, workers.ProvideWorkerOptions(operationManager, executors, nil, nil))

		operationManager.EXPECT().NextSchedulerOperation(gomock.Any(), expectedOperation.SchedulerName).Return(nil, nil, errors.New("error"))

		err := workerService.Start(context.Background())
		assert.Error(t, err, "failed to get next operation: error")

		workerService.Stop(context.Background())
		require.False(t, workerService.IsRunning())
	})
}
