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

package deletescheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	instancemock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	runtimemock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/ports"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
)

func TestDeleteSchedulerExecutor_Execute(t *testing.T) {
	scheduler := &entities.Scheduler{
		Name: "schedulerTest",
		Spec: game_room.Spec{
			TerminationGracePeriod: 5 * time.Second,
		},
	}

	t.Run("returns no error", func(t *testing.T) {
		t.Run("when no internal error occurs with 0 running instances", func(t *testing.T) {
			executor, schedulerStorage, schedulerCache, instanceStorage, operationStorage, _, runtime := prepareMocks(t)
			ctx := context.Background()

			definition := &DeleteSchedulerDefinition{}
			op := &operation.Operation{SchedulerName: scheduler.Name}

			schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)
			schedulerStorage.EXPECT().RunWithTransaction(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(transactionId ports.TransactionID) error) error {
					return f("transactionID")
				})
			schedulerStorage.EXPECT().DeleteScheduler(ctx, ports.TransactionID("transactionID"), scheduler)
			runtime.EXPECT().DeleteScheduler(ctx, scheduler)

			instanceStorage.EXPECT().GetInstanceCount(ctx, scheduler.Name).Return(0, nil)
			schedulerCache.EXPECT().DeleteScheduler(ctx, scheduler.Name)
			operationStorage.EXPECT().CleanOperationsHistory(ctx, scheduler.Name)

			err := executor.Execute(ctx, op, definition)

			require.Nil(t, err)
		})

		t.Run("when no internal error occurs with 20 running instances", func(t *testing.T) {
			executor, schedulerStorage, schedulerCache, instanceStorage, operationStorage, operationManager, runtime := prepareMocks(t)
			ctx := context.Background()

			definition := &DeleteSchedulerDefinition{}
			op := &operation.Operation{SchedulerName: scheduler.Name}

			schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)
			schedulerStorage.EXPECT().RunWithTransaction(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(transactionId ports.TransactionID) error) error {
					return f("transactionID")
				})
			schedulerStorage.EXPECT().DeleteScheduler(ctx, ports.TransactionID("transactionID"), scheduler)
			runtime.EXPECT().DeleteScheduler(ctx, scheduler)

			instanceStorage.EXPECT().GetInstanceCount(ctx, scheduler.Name).Return(20, nil).Times(1)
			instanceStorage.EXPECT().GetInstanceCount(ctx, scheduler.Name).Return(10, nil).Times(1)
			instanceStorage.EXPECT().GetInstanceCount(ctx, scheduler.Name).Return(0, nil).Times(1)
			operationManager.EXPECT().AppendOperationEventToExecutionHistory(ctx, op, "Waiting for instances to be deleted: 10").Times(1)
			operationManager.EXPECT().AppendOperationEventToExecutionHistory(ctx, op, "Waiting for instances to be deleted: 0").Times(1)

			schedulerCache.EXPECT().DeleteScheduler(ctx, scheduler.Name)
			operationStorage.EXPECT().CleanOperationsHistory(ctx, scheduler.Name)

			err := executor.Execute(ctx, op, definition)

			require.Nil(t, err)
		})

		t.Run("when it fails to get scheduler from cache the first time", func(t *testing.T) {
			executor, schedulerStorage, schedulerCache, instanceStorage, operationStorage, _, runtime := prepareMocks(t)
			ctx := context.Background()

			definition := &DeleteSchedulerDefinition{}
			op := &operation.Operation{SchedulerName: scheduler.Name}

			schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, errors.New("error getting scheduler from cache"))
			schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)
			schedulerStorage.EXPECT().RunWithTransaction(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(transactionId ports.TransactionID) error) error {
					return f("transactionID")
				})
			schedulerStorage.EXPECT().DeleteScheduler(ctx, ports.TransactionID("transactionID"), scheduler)
			runtime.EXPECT().DeleteScheduler(ctx, scheduler)

			instanceStorage.EXPECT().GetInstanceCount(ctx, scheduler.Name).Return(0, nil)
			schedulerCache.EXPECT().DeleteScheduler(ctx, scheduler.Name)
			operationStorage.EXPECT().CleanOperationsHistory(ctx, scheduler.Name)

			err := executor.Execute(ctx, op, definition)

			require.Nil(t, err)
		})

		t.Run("when it fails to wait for all instances to be deleted error", func(t *testing.T) {
			executor, schedulerStorage, schedulerCache, instanceStorage, operationStorage, _, runtime := prepareMocks(t)
			ctx := context.Background()

			definition := &DeleteSchedulerDefinition{}
			op := &operation.Operation{SchedulerName: scheduler.Name}

			schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)
			schedulerStorage.EXPECT().RunWithTransaction(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(transactionId ports.TransactionID) error) error {
					return f("transactionID")
				})
			schedulerStorage.EXPECT().DeleteScheduler(ctx, ports.TransactionID("transactionID"), scheduler)
			runtime.EXPECT().DeleteScheduler(ctx, scheduler)

			instanceStorage.EXPECT().GetInstanceCount(ctx, scheduler.Name).Return(0, errors.New("some error instance storage"))
			schedulerCache.EXPECT().DeleteScheduler(ctx, scheduler.Name)
			operationStorage.EXPECT().CleanOperationsHistory(ctx, scheduler.Name)

			err := executor.Execute(ctx, op, definition)

			require.Nil(t, err)
		})

		t.Run("when it fails to delete scheduler from cache", func(t *testing.T) {
			executor, schedulerStorage, schedulerCache, instanceStorage, operationStorage, _, runtime := prepareMocks(t)
			ctx := context.Background()

			definition := &DeleteSchedulerDefinition{}
			op := &operation.Operation{SchedulerName: scheduler.Name}

			schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)
			schedulerStorage.EXPECT().RunWithTransaction(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(transactionId ports.TransactionID) error) error {
					return f("transactionID")
				})
			schedulerStorage.EXPECT().DeleteScheduler(ctx, ports.TransactionID("transactionID"), scheduler)
			runtime.EXPECT().DeleteScheduler(ctx, scheduler)

			instanceStorage.EXPECT().GetInstanceCount(ctx, scheduler.Name).Return(0, nil)
			schedulerCache.EXPECT().DeleteScheduler(ctx, scheduler.Name).Return(errors.New("failed to delete scheduler from cache"))
			operationStorage.EXPECT().CleanOperationsHistory(ctx, scheduler.Name)

			err := executor.Execute(ctx, op, definition)

			require.Nil(t, err)
		})

		t.Run("when it fails to clean operations history", func(t *testing.T) {
			executor, schedulerStorage, schedulerCache, instanceStorage, operationStorage, _, runtime := prepareMocks(t)
			ctx := context.Background()

			definition := &DeleteSchedulerDefinition{}
			op := &operation.Operation{SchedulerName: scheduler.Name}

			schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)
			schedulerStorage.EXPECT().RunWithTransaction(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(transactionId ports.TransactionID) error) error {
					return f("transactionID")
				})
			schedulerStorage.EXPECT().DeleteScheduler(ctx, ports.TransactionID("transactionID"), scheduler)
			runtime.EXPECT().DeleteScheduler(ctx, scheduler)

			instanceStorage.EXPECT().GetInstanceCount(ctx, scheduler.Name).Return(0, nil)
			schedulerCache.EXPECT().DeleteScheduler(ctx, scheduler.Name)
			operationStorage.EXPECT().CleanOperationsHistory(ctx, scheduler.Name).Return(errors.New("failed to clean operations history"))

			err := executor.Execute(ctx, op, definition)

			require.Nil(t, err)
		})

		t.Run("when some error occurs when waiting for instances to be deleted", func(t *testing.T) {
			executor, schedulerStorage, schedulerCache, instanceStorage, operationStorage, _, runtime := prepareMocks(t)
			ctx := context.Background()

			definition := &DeleteSchedulerDefinition{}
			op := &operation.Operation{SchedulerName: scheduler.Name}

			schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)
			schedulerStorage.EXPECT().RunWithTransaction(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(transactionId ports.TransactionID) error) error {
					return f("transactionID")
				})
			schedulerStorage.EXPECT().DeleteScheduler(ctx, ports.TransactionID("transactionID"), scheduler)
			runtime.EXPECT().DeleteScheduler(ctx, scheduler)

			instanceStorage.EXPECT().GetInstanceCount(ctx, scheduler.Name).Return(10, nil)
			instanceStorage.EXPECT().GetInstanceCount(ctx, scheduler.Name).Return(0, errors.New("some error"))

			schedulerCache.EXPECT().DeleteScheduler(ctx, scheduler.Name)
			operationStorage.EXPECT().CleanOperationsHistory(ctx, scheduler.Name)

			err := executor.Execute(ctx, op, definition)

			require.Nil(t, err)
		})

		t.Run("when timeout waiting for instances to be deleted", func(t *testing.T) {
			executor, schedulerStorage, schedulerCache, instanceStorage, operationStorage, operationManager, runtime := prepareMocks(t)
			ctx := context.Background()

			definition := &DeleteSchedulerDefinition{}
			op := &operation.Operation{SchedulerName: scheduler.Name}

			schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)
			schedulerStorage.EXPECT().RunWithTransaction(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(transactionId ports.TransactionID) error) error {
					return f("transactionID")
				})
			schedulerStorage.EXPECT().DeleteScheduler(ctx, ports.TransactionID("transactionID"), scheduler)
			runtime.EXPECT().DeleteScheduler(ctx, scheduler)

			instanceStorage.EXPECT().GetInstanceCount(ctx, scheduler.Name).Return(1, nil).AnyTimes()
			operationManager.EXPECT().AppendOperationEventToExecutionHistory(ctx, op, "Waiting for instances to be deleted: 1").AnyTimes()

			schedulerCache.EXPECT().DeleteScheduler(ctx, scheduler.Name)
			operationStorage.EXPECT().CleanOperationsHistory(ctx, scheduler.Name)

			err := executor.Execute(ctx, op, definition)

			require.Nil(t, err)
		})
	})

	t.Run("returns error", func(t *testing.T) {
		t.Run("when it fails to load the scheduler from storage the first time", func(t *testing.T) {
			executor, schedulerStorage, schedulerCache, _, _, _, _ := prepareMocks(t)
			ctx := context.Background()

			definition := &DeleteSchedulerDefinition{}
			op := &operation.Operation{SchedulerName: scheduler.Name}

			schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(nil, errors.New("some error on cache"))
			schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(nil, errors.New("some error on storage"))

			err := executor.Execute(ctx, op, definition)

			require.Equal(t, errors.New("some error on storage"), err)
		})

		t.Run("when it fails to delete scheduler in storage", func(t *testing.T) {
			executor, schedulerStorage, schedulerCache, _, _, _, _ := prepareMocks(t)
			ctx := context.Background()

			definition := &DeleteSchedulerDefinition{}
			op := &operation.Operation{SchedulerName: scheduler.Name}

			schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)
			schedulerStorage.EXPECT().RunWithTransaction(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(transactionId ports.TransactionID) error) error {
					return f("transactionID")
				})
			schedulerStorage.EXPECT().DeleteScheduler(ctx, ports.TransactionID("transactionID"), scheduler).
				Return(errors.New("some error on storage"))

			err := executor.Execute(ctx, op, definition)
			require.Equal(t, errors.New("some error on storage"), err)
		})

		t.Run("when it fails to delete scheduler in runtime", func(t *testing.T) {
			executor, schedulerStorage, schedulerCache, _, _, _, runtime := prepareMocks(t)

			ctx := context.Background()

			definition := &DeleteSchedulerDefinition{}
			op := &operation.Operation{SchedulerName: scheduler.Name}

			schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)
			schedulerStorage.EXPECT().RunWithTransaction(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(transactionId ports.TransactionID) error) error {
					return f("transactionID")
				})
			schedulerStorage.EXPECT().DeleteScheduler(ctx, ports.TransactionID("transactionID"), scheduler)
			runtime.EXPECT().DeleteScheduler(ctx, scheduler).Return(errors.New("some error on runtime"))

			err := executor.Execute(ctx, op, definition)
			require.Equal(t, errors.New("some error on runtime"), err)
		})
	})
}

func prepareMocks(t *testing.T) (
	*DeleteSchedulerExecutor,
	*mockports.MockSchedulerStorage,
	*mockports.MockSchedulerCache,
	*instancemock.MockGameRoomInstanceStorage,
	*mockports.MockOperationStorage,
	*mockports.MockOperationManager,
	*runtimemock.MockRuntime,
) {
	mockCtrl := gomock.NewController(t)
	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
	schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
	instanceStorage := instancemock.NewMockGameRoomInstanceStorage(mockCtrl)
	operationStorage := mockports.NewMockOperationStorage(mockCtrl)
	operationManager := mockports.NewMockOperationManager(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)

	op := NewExecutor(
		schedulerStorage,
		schedulerCache,
		instanceStorage,
		operationStorage,
		operationManager,
		runtime,
	)

	return op, schedulerStorage, schedulerCache, instanceStorage, operationStorage, operationManager, runtime
}
