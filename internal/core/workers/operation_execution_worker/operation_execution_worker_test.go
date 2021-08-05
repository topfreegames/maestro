//+build unit

package operation_execution_worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	opflow "github.com/topfreegames/maestro/internal/adapters/operation_flow/mock"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	mockoperation "github.com/topfreegames/maestro/internal/core/operations/mock"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
)

func TestSchedulerOperationsExecutionLoop(t *testing.T) {
	t.Run("successfully runs a single operation", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)

		operationName := "test_operation"
		operationDefinition := mockoperation.NewMockDefinition(mockCtrl)
		operationExecutor := mockoperation.NewMockExecutor(mockCtrl)
		operationExecutor.EXPECT().Name().Return(operationName).AnyTimes()
		operationDefinition.EXPECT().Name().Return(operationName).AnyTimes()

		defFunc := func() operations.Definition { return operationDefinition }
		definitionConstructors := operations.NewDefinitionConstructors()
		definitionConstructors[operationName] = defFunc

		operationManager := operation_manager.New(operationFlow, operationStorage, definitionConstructors)
		scheduler := &entities.Scheduler{Name: "random-scheduler"}
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  scheduler.Name,
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		executors := map[string]operations.Executor{}
		executors[operationName] = operationExecutor
		workerService := NewOperationExecutionWorker(scheduler, workers.ProvideWorkerOptions(operationManager, executors))

		operationDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil)
		operationDefinition.EXPECT().ShouldExecute(gomock.Any(), []*operation.Operation{}).Return(true)
		operationExecutor.EXPECT().Execute(gomock.Any(), expectedOperation, operationDefinition).
			Do(func(ctx, operation, definition interface{}) {
				time.Sleep(time.Second * 1)
			}).
			Return(nil)

		operationFlow.EXPECT().NextOperationID(gomock.Any(), expectedOperation.SchedulerName).Return(expectedOperation.ID, nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), expectedOperation.SchedulerName, expectedOperation.ID).Return(expectedOperation, []byte{}, nil)
		operationStorage.EXPECT().UpdateOperationStatus(gomock.Any(), expectedOperation.SchedulerName, expectedOperation.ID, operation.StatusInProgress).Return(nil)
		operationStorage.EXPECT().UpdateOperationStatus(gomock.Any(), expectedOperation.SchedulerName, expectedOperation.ID, operation.StatusFinished).Return(nil)
		// ends the worker by cancelling it
		operationFlow.EXPECT().NextOperationID(gomock.Any(), expectedOperation.SchedulerName).Return("", context.Canceled)

		err := workerService.Start(context.Background())
		require.NoError(t, err)

		workerService.Stop(context.Background())
		require.False(t, workerService.IsRunning())
	})

	t.Run("execute OnError when a Execute fails", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)

		operationName := "test_operation"
		operationDefinition := mockoperation.NewMockDefinition(mockCtrl)
		operationExecutor := mockoperation.NewMockExecutor(mockCtrl)
		operationExecutor.EXPECT().Name().Return(operationName).AnyTimes()
		operationDefinition.EXPECT().Name().Return(operationName).AnyTimes()

		defFunc := func() operations.Definition { return operationDefinition }
		definitionConstructors := operations.NewDefinitionConstructors()
		definitionConstructors[operationName] = defFunc

		operationManager := operation_manager.New(operationFlow, operationStorage, definitionConstructors)
		scheduler := &entities.Scheduler{Name: "random-scheduler"}
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  scheduler.Name,
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		executors := map[string]operations.Executor{}
		executors[operationName] = operationExecutor
		workerService := NewOperationExecutionWorker(scheduler, workers.ProvideWorkerOptions(operationManager, executors))

		operationDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil)
		operationDefinition.EXPECT().ShouldExecute(gomock.Any(), []*operation.Operation{}).Return(true)
		executionErr := fmt.Errorf("failed to execute operation")
		operationExecutor.EXPECT().Execute(gomock.Any(), expectedOperation, operationDefinition).Return(executionErr)
		operationExecutor.EXPECT().OnError(gomock.Any(), expectedOperation, operationDefinition, executionErr).
			Do(func(ctx, operation, definition, executeErr interface{}) {
				time.Sleep(time.Second * 1)
			}).Return(nil)

		operationFlow.EXPECT().NextOperationID(gomock.Any(), expectedOperation.SchedulerName).Return(expectedOperation.ID, nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), expectedOperation.SchedulerName, expectedOperation.ID).Return(expectedOperation, []byte{}, nil)
		operationStorage.EXPECT().UpdateOperationStatus(gomock.Any(), expectedOperation.SchedulerName, expectedOperation.ID, operation.StatusInProgress).Return(nil)
		operationStorage.EXPECT().UpdateOperationStatus(gomock.Any(), expectedOperation.SchedulerName, expectedOperation.ID, operation.StatusError).Return(nil)
		// ends the worker by cancelling it
		operationFlow.EXPECT().NextOperationID(gomock.Any(), expectedOperation.SchedulerName).Return("", context.Canceled)

		err := workerService.Start(context.Background())
		require.NoError(t, err)

		workerService.Stop(context.Background())
		require.False(t, workerService.IsRunning())
	})

	t.Run("evict operation if there is no executor", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)

		operationName := "test_operation"
		operationDefinition := mockoperation.NewMockDefinition(mockCtrl)
		operationDefinition.EXPECT().Name().Return(operationName).AnyTimes()

		defFunc := func() operations.Definition { return operationDefinition }
		definitionConstructors := operations.NewDefinitionConstructors()
		definitionConstructors[operationName] = defFunc

		operationManager := operation_manager.New(operationFlow, operationStorage, definitionConstructors)
		scheduler := &entities.Scheduler{Name: "random-scheduler"}
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  scheduler.Name,
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		executors := map[string]operations.Executor{}
		workerService := NewOperationExecutionWorker(scheduler, workers.ProvideWorkerOptions(operationManager, executors))

		operationDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil)

		operationFlow.EXPECT().NextOperationID(gomock.Any(), expectedOperation.SchedulerName).Return(expectedOperation.ID, nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), expectedOperation.SchedulerName, expectedOperation.ID).Return(expectedOperation, []byte{}, nil)
		operationStorage.EXPECT().UpdateOperationStatus(gomock.Any(), expectedOperation.SchedulerName, expectedOperation.ID, operation.StatusEvicted).Return(nil)
		// ends the worker by cancelling it
		operationFlow.EXPECT().NextOperationID(gomock.Any(), expectedOperation.SchedulerName).Return("", context.Canceled)

		err := workerService.Start(context.Background())
		require.NoError(t, err)
	})

	t.Run("evict operation if ShouldExecute returns false", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)

		operationName := "test_operation"
		operationDefinition := mockoperation.NewMockDefinition(mockCtrl)
		operationExecutor := mockoperation.NewMockExecutor(mockCtrl)
		operationExecutor.EXPECT().Name().Return(operationName).AnyTimes()
		operationDefinition.EXPECT().Name().Return(operationName).AnyTimes()

		defFunc := func() operations.Definition { return operationDefinition }
		definitionConstructors := operations.NewDefinitionConstructors()
		definitionConstructors[operationName] = defFunc

		operationManager := operation_manager.New(operationFlow, operationStorage, definitionConstructors)
		scheduler := &entities.Scheduler{Name: "random-scheduler"}
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  scheduler.Name,
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		executors := map[string]operations.Executor{}
		executors[operationName] = operationExecutor
		workerService := NewOperationExecutionWorker(scheduler, workers.ProvideWorkerOptions(operationManager, executors))

		operationDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil)
		operationDefinition.EXPECT().ShouldExecute(gomock.Any(), []*operation.Operation{}).Return(false)

		operationFlow.EXPECT().NextOperationID(gomock.Any(), expectedOperation.SchedulerName).Return(expectedOperation.ID, nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), expectedOperation.SchedulerName, expectedOperation.ID).Return(expectedOperation, []byte{}, nil)
		operationStorage.EXPECT().UpdateOperationStatus(gomock.Any(), expectedOperation.SchedulerName, expectedOperation.ID, operation.StatusEvicted).Return(nil)
		// ends the worker by cancelling it
		operationFlow.EXPECT().NextOperationID(gomock.Any(), expectedOperation.SchedulerName).Return("", context.Canceled)

		err := workerService.Start(context.Background())
		require.NoError(t, err)

		workerService.Stop(context.Background())
		require.False(t, workerService.IsRunning())
	})
}
