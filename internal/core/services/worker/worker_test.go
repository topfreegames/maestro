//+build unit

package worker

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	opflow "github.com/topfreegames/maestro/internal/adapters/operation_flow/mock"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	mockoperation "github.com/topfreegames/maestro/internal/core/operations/mock"
	"github.com/topfreegames/maestro/internal/core/services/operations_registry"
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
		registry := operations_registry.NewRegistry()
		registry.Register(operationName, defFunc)

		workerService := NewWorker(&WorkerOptions{operationStorage, operationFlow, []operations.Executor{operationExecutor}, &registry})

		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  "random-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		operationDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil)
		operationDefinition.EXPECT().ShouldExecute(context.Background(), []*operation.Operation{}).Return(true)
		operationExecutor.EXPECT().Execute(context.Background(), expectedOperation, operationDefinition).Return(nil)

		operationFlow.EXPECT().NextOperationID(context.Background(), expectedOperation.SchedulerName).Return(expectedOperation.ID, nil)
		operationStorage.EXPECT().GetOperation(context.Background(), expectedOperation.SchedulerName, expectedOperation.ID).Return(expectedOperation, []byte{}, nil)
		operationStorage.EXPECT().UpdateOperationStatus(context.Background(), expectedOperation.SchedulerName, expectedOperation.ID, operation.StatusInProgress).Return(nil)
		operationStorage.EXPECT().UpdateOperationStatus(context.Background(), expectedOperation.SchedulerName, expectedOperation.ID, operation.StatusFinished).Return(nil)
		operationFlow.EXPECT().NextOperationID(context.Background(), expectedOperation.SchedulerName).Return("", context.Canceled)

		err := workerService.SchedulerOperationsExecutionLoop(context.Background(), expectedOperation.SchedulerName)
		require.NoError(t, err)
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
		registry := operations_registry.NewRegistry()
		registry.Register(operationName, defFunc)

		workerService := NewWorker(&WorkerOptions{operationStorage, operationFlow, []operations.Executor{operationExecutor}, &registry})

		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  "random-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		operationDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil)
		operationDefinition.EXPECT().ShouldExecute(context.Background(), []*operation.Operation{}).Return(true)
		executionErr := fmt.Errorf("failed to execute operation")
		operationExecutor.EXPECT().Execute(context.Background(), expectedOperation, operationDefinition).Return(executionErr)
		operationExecutor.EXPECT().OnError(context.Background(), expectedOperation, operationDefinition, executionErr).Return(nil)

		operationFlow.EXPECT().NextOperationID(context.Background(), expectedOperation.SchedulerName).Return(expectedOperation.ID, nil)
		operationStorage.EXPECT().GetOperation(context.Background(), expectedOperation.SchedulerName, expectedOperation.ID).Return(expectedOperation, []byte{}, nil)
		operationStorage.EXPECT().UpdateOperationStatus(context.Background(), expectedOperation.SchedulerName, expectedOperation.ID, operation.StatusInProgress).Return(nil)
		operationStorage.EXPECT().UpdateOperationStatus(context.Background(), expectedOperation.SchedulerName, expectedOperation.ID, operation.StatusError).Return(nil)
		operationFlow.EXPECT().NextOperationID(context.Background(), expectedOperation.SchedulerName).Return("", context.Canceled)

		err := workerService.SchedulerOperationsExecutionLoop(context.Background(), expectedOperation.SchedulerName)
		require.NoError(t, err)
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
		registry := operations_registry.NewRegistry()
		registry.Register(operationName, defFunc)

		workerService := NewWorker(&WorkerOptions{operationStorage, operationFlow, []operations.Executor{}, &registry})

		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  "random-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		operationDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil)

		operationFlow.EXPECT().NextOperationID(context.Background(), expectedOperation.SchedulerName).Return(expectedOperation.ID, nil)
		operationStorage.EXPECT().GetOperation(context.Background(), expectedOperation.SchedulerName, expectedOperation.ID).Return(expectedOperation, []byte{}, nil)
		operationStorage.EXPECT().UpdateOperationStatus(context.Background(), expectedOperation.SchedulerName, expectedOperation.ID, operation.StatusEvicted).Return(nil)
		operationFlow.EXPECT().NextOperationID(context.Background(), expectedOperation.SchedulerName).Return("", context.Canceled)

		err := workerService.SchedulerOperationsExecutionLoop(context.Background(), expectedOperation.SchedulerName)
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
		registry := operations_registry.NewRegistry()
		registry.Register(operationName, defFunc)

		workerService := NewWorker(&WorkerOptions{operationStorage, operationFlow, []operations.Executor{operationExecutor}, &registry})

		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  "random-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		operationDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil)
		operationDefinition.EXPECT().ShouldExecute(context.Background(), []*operation.Operation{}).Return(false)

		operationFlow.EXPECT().NextOperationID(context.Background(), expectedOperation.SchedulerName).Return(expectedOperation.ID, nil)
		operationStorage.EXPECT().GetOperation(context.Background(), expectedOperation.SchedulerName, expectedOperation.ID).Return(expectedOperation, []byte{}, nil)
		operationStorage.EXPECT().UpdateOperationStatus(context.Background(), expectedOperation.SchedulerName, expectedOperation.ID, operation.StatusEvicted).Return(nil)
		operationFlow.EXPECT().NextOperationID(context.Background(), expectedOperation.SchedulerName).Return("", context.Canceled)

		err := workerService.SchedulerOperationsExecutionLoop(context.Background(), expectedOperation.SchedulerName)
		require.NoError(t, err)
	})
}
