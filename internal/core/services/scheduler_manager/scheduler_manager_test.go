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

// +build unit

package scheduler_manager

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	opflow "github.com/topfreegames/maestro/internal/adapters/operation_flow/mock"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	schedulerStorageMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
)

func TestAddRooms(t *testing.T) {
	schedulerName := "scheduler-name-1"

	t.Run("with success", func(t *testing.T) {

		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors())
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager)

		operationStorage.EXPECT().CreateOperation(ctx, gomock.Any(), gomock.Any()).Return(nil)
		operationFlow.EXPECT().InsertOperationID(ctx, schedulerName, gomock.Any()).Return(nil)
		schedulerStorage.EXPECT().GetScheduler(ctx, schedulerName).Return(nil, nil)

		op, err := schedulerManager.AddRooms(ctx, schedulerName, 10)
		require.NoError(t, err)
		require.NotNil(t, op)
		require.NotNil(t, op.ID)
	})

	t.Run("fails when scheduler does not exists", func(t *testing.T) {
		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetScheduler(ctx, schedulerName).Return(nil, errors.NewErrNotFound("err"))

		op, err := schedulerManager.AddRooms(ctx, schedulerName, 10)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrNotFound)
		require.Contains(t, err.Error(), "no scheduler found to add rooms on it: err")
	})

	t.Run("fails when operation enqueue fails", func(t *testing.T) {
		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(nil, operationStorage, operations.NewDefinitionConstructors())
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager)

		schedulerStorage.EXPECT().GetScheduler(ctx, schedulerName).Return(nil, nil)
		operationStorage.EXPECT().CreateOperation(ctx, gomock.Any(), gomock.Any()).Return(errors.NewErrUnexpected("storage offline"))

		op, err := schedulerManager.AddRooms(ctx, schedulerName, 10)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrUnexpected)
		require.Contains(t, err.Error(), "not able to schedule the 'add rooms' operation: failed to create operation: storage offline")
	})
}

func TestRemoveRooms(t *testing.T) {
	schedulerName := "scheduler-name-1"

	t.Run("with success", func(t *testing.T) {

		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors())
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager)
		//operationDefinition := remove_rooms.RemoveRoomsDefinition{Amount: 10}

		operationStorage.EXPECT().CreateOperation(ctx, gomock.Any(), gomock.Any()).Return(nil)
		operationFlow.EXPECT().InsertOperationID(ctx, schedulerName, gomock.Any()).Return(nil)
		schedulerStorage.EXPECT().GetScheduler(ctx, schedulerName).Return(nil, nil)

		op, err := schedulerManager.RemoveRooms(ctx, schedulerName, 10)
		require.NoError(t, err)
		require.NotNil(t, op)
		require.NotNil(t, op.ID)
	})

	t.Run("fails when scheduler does not exists", func(t *testing.T) {
		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetScheduler(ctx, schedulerName).Return(nil, errors.NewErrNotFound("err"))

		op, err := schedulerManager.RemoveRooms(ctx, schedulerName, 10)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrNotFound)
		require.Contains(t, err.Error(), "no scheduler found for removing rooms: err")
	})

	t.Run("fails when operation enqueue fails", func(t *testing.T) {
		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(nil, operationStorage, operations.NewDefinitionConstructors())
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager)

		schedulerStorage.EXPECT().GetScheduler(ctx, schedulerName).Return(nil, nil)
		operationStorage.EXPECT().CreateOperation(ctx, gomock.Any(), gomock.Any()).Return(errors.NewErrUnexpected("storage offline"))

		op, err := schedulerManager.RemoveRooms(ctx, schedulerName, 10)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrUnexpected)
		require.Contains(t, err.Error(), "not able to schedule the 'remove rooms' operation: failed to create operation: storage offline")
	})
}
