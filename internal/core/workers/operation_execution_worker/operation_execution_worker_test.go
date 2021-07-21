//+build unit

package operation_execution_worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	opflow "github.com/topfreegames/maestro/internal/adapters/operation_flow/mock"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/monitoring"
	"github.com/topfreegames/maestro/internal/core/operations"
	mockoperation "github.com/topfreegames/maestro/internal/core/operations/mock"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/services/operations_registry"
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
		registry := operations_registry.NewRegistry()
		registry.Register(operationName, defFunc)

		operationManager := operation_manager.NewWithRegistry(operationFlow, operationStorage, registry)
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  "random-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		workerService := NewOperationExecutionWorker(expectedOperation.SchedulerName, &workers.WorkerOptions{operationManager, []operations.Executor{operationExecutor}})

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
		require.False(t, workerService.IsRunning(context.Background()))

		metrics, _ := prometheus.DefaultGatherer.Gather()
		latency := monitoring.FilterMetric(metrics, "maestro_worker_operation_execution_latency").GetMetric()[0]
		require.Equal(t, uint64(1), latency.GetHistogram().GetSampleCount())
		require.GreaterOrEqual(t, latency.GetHistogram().GetSampleSum(), float64(1000))
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

		operationManager := operation_manager.NewWithRegistry(operationFlow, operationStorage, registry)
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  "random-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		workerService := NewOperationExecutionWorker(expectedOperation.SchedulerName, &workers.WorkerOptions{operationManager, []operations.Executor{operationExecutor}})

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
		require.False(t, workerService.IsRunning(context.Background()))

		metrics, _ := prometheus.DefaultGatherer.Gather()
		latency := monitoring.FilterMetric(metrics, "maestro_worker_operation_on_error_latency").GetMetric()[0]
		require.Equal(t, uint64(1), latency.GetHistogram().GetSampleCount())
		require.GreaterOrEqual(t, latency.GetHistogram().GetSampleSum(), float64(1000))
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

		operationManager := operation_manager.NewWithRegistry(operationFlow, operationStorage, registry)
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  "random-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		workerService := NewOperationExecutionWorker(expectedOperation.SchedulerName, &workers.WorkerOptions{operationManager, []operations.Executor{}})

		operationDefinition.EXPECT().Unmarshal(gomock.Any()).Return(nil)

		operationFlow.EXPECT().NextOperationID(gomock.Any(), expectedOperation.SchedulerName).Return(expectedOperation.ID, nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), expectedOperation.SchedulerName, expectedOperation.ID).Return(expectedOperation, []byte{}, nil)
		operationStorage.EXPECT().UpdateOperationStatus(gomock.Any(), expectedOperation.SchedulerName, expectedOperation.ID, operation.StatusEvicted).Return(nil)
		// ends the worker by cancelling it
		operationFlow.EXPECT().NextOperationID(gomock.Any(), expectedOperation.SchedulerName).Return("", context.Canceled)

		err := workerService.Start(context.Background())
		require.NoError(t, err)

		metrics, _ := prometheus.DefaultGatherer.Gather()
		counter := monitoring.FilterMetric(metrics, "maestro_worker_operation_evicted_counter").GetMetric()[0]
		require.Equal(t, float64(1), counter.GetCounter().GetValue())

		operationLabel := counter.GetLabel()[0]
		require.Equal(t, "operation", operationLabel.GetName())
		require.Equal(t, "test_operation", operationLabel.GetValue())

		reasonLabel := counter.GetLabel()[1]
		require.Equal(t, "reason", reasonLabel.GetName())
		require.Equal(t, "no_operation_executor_found", reasonLabel.GetValue())

		schedulerLabel := counter.GetLabel()[2]
		require.Equal(t, "scheduler", schedulerLabel.GetName())
		require.Equal(t, "random-scheduler", schedulerLabel.GetValue())
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

		operationManager := operation_manager.NewWithRegistry(operationFlow, operationStorage, registry)
		expectedOperation := &operation.Operation{
			ID:             "random-operation-id",
			SchedulerName:  "random-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: operationName,
		}

		workerService := NewOperationExecutionWorker(expectedOperation.SchedulerName, &workers.WorkerOptions{operationManager, []operations.Executor{operationExecutor}})

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
		require.False(t, workerService.IsRunning(context.Background()))

		metrics, _ := prometheus.DefaultGatherer.Gather()
		counter := monitoring.FilterMetric(metrics, "maestro_worker_operation_evicted_counter").GetMetric()[1]
		require.Equal(t, float64(1), counter.GetCounter().GetValue())

		operationLabel := counter.GetLabel()[0]
		require.Equal(t, "operation", operationLabel.GetName())
		require.Equal(t, "test_operation", operationLabel.GetValue())

		reasonLabel := counter.GetLabel()[1]
		require.Equal(t, "reason", reasonLabel.GetName())
		require.Equal(t, "should_not_execute", reasonLabel.GetValue())

		schedulerLabel := counter.GetLabel()[2]
		require.Equal(t, "scheduler", schedulerLabel.GetName())
		require.Equal(t, "random-scheduler", schedulerLabel.GetValue())
	})
}
