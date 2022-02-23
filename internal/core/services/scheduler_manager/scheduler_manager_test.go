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

package scheduler_manager

import (
	"context"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/ports/mock"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
	schedulerStorageMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/filters"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/validations"
)

func TestCreateScheduler(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
	operationManager := mock.NewMockOperationManager(mockCtrl)
	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

	t.Run("with valid scheduler it returns no error when creating it", func(t *testing.T) {
		scheduler := newValidScheduler()

		schedulerStorage.EXPECT().CreateScheduler(ctx, scheduler).Return(nil)
		operationManager.EXPECT().CreateOperation(ctx, scheduler.Name, gomock.Any()).Return(&operation.Operation{}, nil)
		schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)

		result, err := schedulerManager.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, scheduler, result)
	})

	t.Run("with valid scheduler it returns with error if some error occurs when creating scheduler on storage", func(t *testing.T) {
		scheduler := newValidScheduler()

		schedulerStorage.EXPECT().CreateScheduler(ctx, scheduler).Return(errors.NewErrUnexpected("some error"))

		result, err := schedulerManager.CreateScheduler(ctx, scheduler)
		require.Error(t, err, "some error")
		require.Nil(t, result)
	})

	t.Run("with valid scheduler it returns with error if some error occurs when creating scheduler operation", func(t *testing.T) {
		scheduler := newValidScheduler()

		schedulerStorage.EXPECT().CreateScheduler(ctx, scheduler).Return(nil)
		operationManager.EXPECT().CreateOperation(ctx, scheduler.Name, gomock.Any()).Return(nil, errors.NewErrUnexpected("some error"))

		result, err := schedulerManager.CreateScheduler(ctx, scheduler)
		require.Error(t, err, "failing in creating the operation: create_scheduler: failed to create operation: some error")
		require.Nil(t, result)
	})

	t.Run("with invalid scheduler it return invalid scheduler error", func(t *testing.T) {
		scheduler := newInvalidScheduler()

		result, err := schedulerManager.CreateScheduler(ctx, scheduler)
		require.Error(t, err)
		require.Nil(t, result)
	})

}

func TestCreateNewSchedulerVersion(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
	operationManager := mock.NewMockOperationManager(mockCtrl)
	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

	t.Run("with valid scheduler it returns no error when creating it", func(t *testing.T) {
		scheduler := newValidScheduler()

		schedulerStorage.EXPECT().CreateSchedulerVersion(ctx, ports.TransactionID(""), scheduler).Return(nil)

		err := schedulerManager.CreateNewSchedulerVersion(ctx, scheduler)
		require.NoError(t, err)
	})

	t.Run("with valid scheduler it returns with error if some error occurs when creating new version on storage", func(t *testing.T) {
		scheduler := newValidScheduler()

		schedulerStorage.EXPECT().CreateSchedulerVersion(ctx, ports.TransactionID(""), scheduler).Return(errors.NewErrUnexpected("some error"))

		err := schedulerManager.CreateNewSchedulerVersion(ctx, scheduler)
		require.Error(t, err, "some error")
	})

	t.Run("with invalid scheduler it return invalid scheduler error", func(t *testing.T) {
		scheduler := newInvalidScheduler()

		err := schedulerManager.CreateNewSchedulerVersion(ctx, scheduler)
		require.Error(t, err)
	})

}

func TestAddRooms(t *testing.T) {
	schedulerName := "scheduler-name-1"
	mockCtrl := gomock.NewController(t)

	t.Run("with success", func(t *testing.T) {

		ctx := context.Background()

		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(ctx, schedulerName, gomock.Any()).Return(&operation.Operation{}, nil)

		schedulerStorage.EXPECT().GetScheduler(ctx, schedulerName).Return(nil, nil)

		op, err := schedulerManager.AddRooms(ctx, schedulerName, 10)
		require.NoError(t, err)
		require.NotNil(t, op)
		require.NotNil(t, op.ID)
	})

	t.Run("fails when scheduler does not exists", func(t *testing.T) {
		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil, nil)

		schedulerStorage.EXPECT().GetScheduler(ctx, schedulerName).Return(nil, errors.NewErrNotFound("err"))

		op, err := schedulerManager.AddRooms(ctx, schedulerName, 10)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrNotFound)
		require.Contains(t, err.Error(), "no scheduler found to add rooms on it: err")
	})

	t.Run("fails when operation enqueue fails", func(t *testing.T) {
		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetScheduler(ctx, schedulerName).Return(nil, nil)
		operationManager.EXPECT().CreateOperation(ctx, schedulerName, gomock.Any()).Return(nil, errors.NewErrUnexpected("storage offline"))

		op, err := schedulerManager.AddRooms(ctx, schedulerName, 10)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrUnexpected)
		require.Contains(t, err.Error(), "not able to schedule the 'add rooms' operation: storage offline")
	})
}

func TestRemoveRooms(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	schedulerName := "scheduler-name-1"

	t.Run("with success", func(t *testing.T) {

		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(ctx, schedulerName, gomock.Any()).Return(&operation.Operation{}, nil)
		schedulerStorage.EXPECT().GetScheduler(ctx, schedulerName).Return(nil, nil)

		op, err := schedulerManager.RemoveRooms(ctx, schedulerName, 10)
		require.NoError(t, err)
		require.NotNil(t, op)
		require.NotNil(t, op.ID)
	})

	t.Run("fails when scheduler does not exists", func(t *testing.T) {
		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil, roomStorage)

		schedulerStorage.EXPECT().GetScheduler(ctx, schedulerName).Return(nil, errors.NewErrNotFound("err"))

		op, err := schedulerManager.RemoveRooms(ctx, schedulerName, 10)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrNotFound)
		require.Contains(t, err.Error(), "no scheduler found for removing rooms: err")
	})

	t.Run("fails when operation enqueue fails", func(t *testing.T) {
		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetScheduler(ctx, schedulerName).Return(nil, nil)
		operationManager.EXPECT().CreateOperation(ctx, schedulerName, gomock.Any()).Return(nil, errors.NewErrUnexpected("storage offline"))

		op, err := schedulerManager.RemoveRooms(ctx, schedulerName, 10)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrUnexpected)
		require.Contains(t, err.Error(), "not able to schedule the 'remove rooms' operation: storage offline")
	})
}

func TestEnqueueNewSchedulerVersionOperation(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	t.Run("return the operation when no error occurs", func(t *testing.T) {
		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(ctx, scheduler.Name, gomock.Any()).Return(&operation.Operation{}, nil)
		schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)

		op, err := schedulerManager.EnqueueNewSchedulerVersionOperation(ctx, scheduler)
		require.NoError(t, err)
		require.NotNil(t, op)
		require.NotNil(t, op.ID)

	})

	t.Run("return error when the scheduler is invalid", func(t *testing.T) {
		scheduler := newInvalidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)

		_, err := schedulerManager.EnqueueNewSchedulerVersionOperation(ctx, scheduler)
		require.Error(t, err)

	})

	t.Run("return error when scheduler is not found", func(t *testing.T) {
		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(nil, errors.NewErrUnexpected("some_error"))

		_, err := schedulerManager.EnqueueNewSchedulerVersionOperation(ctx, scheduler)
		require.Error(t, err)
	})

	t.Run("with failure", func(t *testing.T) {
		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)
		operationManager.EXPECT().CreateOperation(ctx, scheduler.Name, gomock.Any()).Return(nil, errors.NewErrUnexpected("storage offline"))

		op, err := schedulerManager.EnqueueNewSchedulerVersionOperation(ctx, scheduler)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrUnexpected)
	})
}

func TestEnqueueSwitchActiveVersionOperation(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	t.Run("return the operation when no error occurs", func(t *testing.T) {

		currentScheduler := newValidScheduler()
		currentScheduler.PortRange = &entities.PortRange{Start: 1, End: 2}

		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(ctx, scheduler.Name, gomock.Any()).Return(&operation.Operation{}, nil)

		op, err := schedulerManager.EnqueueSwitchActiveVersionOperation(ctx, scheduler, true)
		require.NoError(t, err)
		require.NotNil(t, op)
		require.NotNil(t, op.ID)

	})

	t.Run("return error when the scheduler is invalid", func(t *testing.T) {

		currentScheduler := newValidScheduler()
		currentScheduler.PortRange = &entities.PortRange{Start: 1, End: 2}

		scheduler := newInvalidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		_, err := schedulerManager.EnqueueSwitchActiveVersionOperation(ctx, scheduler, true)
		require.Error(t, err)

	})

	t.Run("return error when some error occurs while creating operation", func(t *testing.T) {
		currentScheduler := newValidScheduler()
		currentScheduler.PortRange = &entities.PortRange{Start: 1, End: 2}

		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(ctx, scheduler.Name, gomock.Any()).Return(nil, errors.NewErrUnexpected("storage offline"))

		op, err := schedulerManager.EnqueueSwitchActiveVersionOperation(ctx, scheduler, true)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrUnexpected)
		require.Contains(t, err.Error(), "failed to schedule switch_active_version operation:")
	})
}

func TestGetSchedulerVersions(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("with success", func(t *testing.T) {
		scheduler := newValidScheduler()

		schedulerVersionList := newValidSchedulerVersionList()

		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetSchedulerVersions(ctx, scheduler.Name).Return(schedulerVersionList, nil)

		versions, err := schedulerManager.GetSchedulerVersions(ctx, scheduler.Name)
		require.NoError(t, err)
		require.NotNil(t, versions)
		require.Equal(t, versions, schedulerVersionList)
	})

	t.Run("error", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetSchedulerVersions(ctx, scheduler.Name).Return(nil, errors.NewErrNotFound("scheduler not found"))

		versions, err := schedulerManager.GetSchedulerVersions(ctx, scheduler.Name)
		require.Error(t, err)
		require.Nil(t, versions)
	})
}

func TestGetScheduler(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("with success", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerFilter := &filters.SchedulerFilter{
			Name:    scheduler.Name,
			Version: scheduler.Spec.Version,
		}
		schedulerStorage.EXPECT().GetSchedulerWithFilter(ctx, schedulerFilter).Return(scheduler, nil)

		retScheduler, err := schedulerManager.GetScheduler(ctx, schedulerFilter.Name, schedulerFilter.Version)
		require.NoError(t, err)
		require.NotNil(t, retScheduler)
		require.Equal(t, retScheduler, scheduler)
	})

	t.Run("error", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerFilter := &filters.SchedulerFilter{
			Name:    scheduler.Name,
			Version: scheduler.Spec.Version,
		}
		schedulerStorage.EXPECT().GetSchedulerWithFilter(ctx, schedulerFilter).Return(nil, errors.NewErrNotFound("scheduler not found"))

		retScheduler, err := schedulerManager.GetScheduler(ctx, schedulerFilter.Name, schedulerFilter.Version)
		require.Error(t, err)
		require.Nil(t, retScheduler)
	})
}

func TestGetSchedulersWithFilter(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("when no error occurs returns a list of schedulers", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()
		schedulerFilter := &filters.SchedulerFilter{}
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulers := []*entities.Scheduler{scheduler}
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetSchedulersWithFilter(ctx, gomock.Any()).Return(schedulers, nil)

		retScheduler, err := schedulerManager.GetSchedulersWithFilter(ctx, schedulerFilter)
		require.NoError(t, err)
		require.NotNil(t, retScheduler)
		require.Equal(t, retScheduler, schedulers)
	})

	t.Run("when some error occurs returns error", func(t *testing.T) {
		ctx := context.Background()
		schedulerFilter := &filters.SchedulerFilter{}
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetSchedulersWithFilter(ctx, gomock.Any()).Return(nil, errors.NewErrUnexpected("some error"))

		retScheduler, err := schedulerManager.GetSchedulersWithFilter(ctx, schedulerFilter)
		require.Error(t, err, "some error")
		require.Empty(t, retScheduler)
	})
}

func TestUpdateScheduler(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("with success", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()

		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerStorage.EXPECT().UpdateScheduler(ctx, scheduler).Return(nil)

		err := schedulerManager.UpdateScheduler(ctx, scheduler)
		require.NoError(t, err)
	})

	t.Run("with valid scheduler it returns with error if some error occurs when update scheduler on storage", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()

		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerStorage.EXPECT().UpdateScheduler(ctx, scheduler).Return(errors.NewErrUnexpected("error"))

		err := schedulerManager.UpdateScheduler(ctx, scheduler)
		require.Error(t, err)
	})

	t.Run("with invalid scheduler it return invalid scheduler error", func(t *testing.T) {
		scheduler := newInvalidScheduler()
		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		err := schedulerManager.CreateNewSchedulerVersion(ctx, scheduler)

		require.Error(t, err)
	})
}

func TestSwitchActiveVersion(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	schedulerName := "scheduler-name-1"
	targetVersion := "v2.0.0"

	t.Run("with success", func(t *testing.T) {
		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		scheduler := newValidScheduler()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(gomock.Any(), scheduler.Name, gomock.Any()).Return(&operation.Operation{}, nil)
		schedulerStorage.EXPECT().GetSchedulerWithFilter(gomock.Any(), gomock.Any()).Return(scheduler, nil)

		currentScheduler := newValidScheduler()
		currentScheduler.SetSchedulerVersion("v2.0.0")
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(currentScheduler, nil)

		op, err := schedulerManager.SwitchActiveVersion(ctx, schedulerName, targetVersion)
		require.NoError(t, err)
		require.NotNil(t, op)
		require.NotNil(t, op.ID)
	})

	t.Run("fails when scheduler does not exists", func(t *testing.T) {
		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil, roomStorage)

		schedulerStorage.EXPECT().GetSchedulerWithFilter(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrNotFound("err"))

		op, err := schedulerManager.SwitchActiveVersion(ctx, schedulerName, targetVersion)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrNotFound)
		require.Contains(t, err.Error(), "no scheduler versions found to switch")
	})

	t.Run("fails when fetching current active scheduler errors", func(t *testing.T) {
		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil, roomStorage)

		schedulerStorage.EXPECT().GetSchedulerWithFilter(gomock.Any(), gomock.Any()).Return(newValidScheduler(), nil)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrUnexpected("err"))

		op, err := schedulerManager.SwitchActiveVersion(ctx, schedulerName, targetVersion)
		require.Nil(t, op)
		require.Error(t, err)
	})

	t.Run("fails when operation enqueue fails", func(t *testing.T) {
		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		scheduler := newValidScheduler()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetSchedulerWithFilter(gomock.Any(), gomock.Any()).Return(scheduler, nil)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(scheduler, nil)
		operationManager.EXPECT().CreateOperation(gomock.Any(), scheduler.Name, gomock.Any()).Return(&operation.Operation{}, errors.NewErrUnexpected("storage offline"))

		op, err := schedulerManager.SwitchActiveVersion(ctx, schedulerName, targetVersion)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrUnexpected)
		require.Contains(t, err.Error(), "failed to schedule operation: failed to schedule switch_active_version operation")
	})
}

func TestGetSchedulersInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Run("with valid request it returns a list of scheduler and game rooms information", func(t *testing.T) {
		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil, roomStorage)
		schedulerFilter := filters.SchedulerFilter{Game: "Tennis-Clash"}
		scheduler := newValidScheduler()
		schedulers := []*entities.Scheduler{scheduler}
		schedulerStorage.EXPECT().GetSchedulersWithFilter(gomock.Any(), gomock.Any()).Return(schedulers, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(5, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(10, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(15, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(20, nil)

		schedulersInfo, err := schedulerManager.GetSchedulersInfo(ctx, &schedulerFilter)

		require.NoError(t, err)
		require.NotNil(t, schedulersInfo)
	})

	t.Run("it returns with error when no scheduler and game rooms info was founded", func(t *testing.T) {
		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil, roomStorage)
		schedulerFilter := filters.SchedulerFilter{Game: "Tennis-Clash"}
		schedulerStorage.EXPECT().GetSchedulersWithFilter(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrNotFound("err"))

		schedulersInfo, err := schedulerManager.GetSchedulersInfo(ctx, &schedulerFilter)

		require.Error(t, err)
		require.Nil(t, schedulersInfo)
		require.ErrorIs(t, err, errors.ErrNotFound)
		require.Contains(t, err.Error(), "no schedulers found: err")
	})

	t.Run("it returns with error when couldn't get game rooms information", func(t *testing.T) {
		ctx := context.Background()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil, roomStorage)
		schedulerFilter := filters.SchedulerFilter{Game: "Tennis-Clash"}
		scheduler := newValidScheduler()
		schedulers := []*entities.Scheduler{scheduler}
		schedulerStorage.EXPECT().GetSchedulersWithFilter(gomock.Any(), gomock.Any()).Return(schedulers, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, errors.NewErrUnexpected("err"))

		schedulersInfo, err := schedulerManager.GetSchedulersInfo(ctx, &schedulerFilter)

		require.Error(t, err)
		require.Nil(t, schedulersInfo)
	})
}

func TestNewSchedulerInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Run("with valid request it returns a scheduler and game rooms information", func(t *testing.T) {
		ctx := context.Background()
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(nil, nil, roomStorage)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(5, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(10, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(15, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(20, nil)
		scheduler := newValidScheduler()

		schedulersInfo, err := schedulerManager.newSchedulerInfo(ctx, scheduler)

		require.NoError(t, err)
		require.NotNil(t, schedulersInfo)
	})

	t.Run("it returns with error when couldn't get game rooms information in ready state", func(t *testing.T) {
		ctx := context.Background()
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(nil, nil, roomStorage)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, errors.NewErrUnexpected("err"))
		scheduler := newValidScheduler()

		schedulersInfo, err := schedulerManager.newSchedulerInfo(ctx, scheduler)

		require.Error(t, err)
		require.Nil(t, schedulersInfo)
	})

	t.Run("it returns with error when couldn't get game rooms information in pending state", func(t *testing.T) {
		ctx := context.Background()
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(nil, nil, roomStorage)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(5, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, errors.NewErrUnexpected("err"))
		scheduler := newValidScheduler()

		schedulersInfo, err := schedulerManager.newSchedulerInfo(ctx, scheduler)

		require.Error(t, err)
		require.Nil(t, schedulersInfo)
	})

	t.Run("it returns with error when couldn't get game rooms information in occupied state", func(t *testing.T) {
		ctx := context.Background()
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(nil, nil, roomStorage)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(5, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(5, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, errors.NewErrUnexpected("err"))
		scheduler := newValidScheduler()

		schedulersInfo, err := schedulerManager.newSchedulerInfo(ctx, scheduler)

		require.Error(t, err)
		require.Nil(t, schedulersInfo)
	})

	t.Run("it returns with error when couldn't get game rooms information in terminating state", func(t *testing.T) {
		ctx := context.Background()
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(nil, nil, roomStorage)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(5, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(5, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(5, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, errors.NewErrUnexpected("err"))
		scheduler := newValidScheduler()

		schedulersInfo, err := schedulerManager.newSchedulerInfo(ctx, scheduler)

		require.Error(t, err)
		require.Nil(t, schedulersInfo)
	})
}

// newValidScheduler generates a valid scheduler with the required fields.
func newValidScheduler() *entities.Scheduler {
	return &entities.Scheduler{
		Name:            "scheduler",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "10%",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Toleration:             "toleration",
			Affinity:               "affinity",
			Containers: []game_room.Container{
				{
					Name:            "default",
					Image:           "some-image",
					ImagePullPolicy: "Always",
					Command:         []string{"hello"},
					Ports: []game_room.ContainerPort{
						{Name: "tcp", Protocol: "tcp", Port: 80},
					},
					Requests: game_room.ContainerResources{
						CPU:    "10m",
						Memory: "100Mi",
					},
					Limits: game_room.ContainerResources{
						CPU:    "10m",
						Memory: "100Mi",
					},
				},
			},
		},
		PortRange: &entities.PortRange{
			Start: 40000,
			End:   60000,
		},
	}
}

// newValidScheduler generates an invalid scheduler
func newInvalidScheduler() *entities.Scheduler {
	return &entities.Scheduler{
		Name:            "",
		Game:            "",
		State:           entities.StateCreating,
		MaxSurge:        "12.0",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Toleration:             "toleration",
			Affinity:               "affinity",
			Containers: []game_room.Container{
				{
					Name:            "default",
					Image:           "some-image",
					ImagePullPolicy: "Always",
					Command:         []string{"hello"},
					Ports: []game_room.ContainerPort{
						{Name: "tcp", Protocol: "tcp", Port: 80},
					},
					Requests: game_room.ContainerResources{
						CPU:    "10m",
						Memory: "100Mi",
					},
					Limits: game_room.ContainerResources{
						CPU:    "10m",
						Memory: "100Mi",
					},
				},
			},
		},
		PortRange: &entities.PortRange{
			Start: -1,
			End:   -1000,
		},
	}
}

// newValidSchedulerVersionList generates a valid list with SchedulerVersions.
func newValidSchedulerVersionList() []*entities.SchedulerVersion {
	listSchedulerVersions := make([]*entities.SchedulerVersion, 2)
	listSchedulerVersions[0] = &entities.SchedulerVersion{
		Version:   "v2.0",
		IsActive:  false,
		CreatedAt: time.Now(),
	}
	listSchedulerVersions[1] = &entities.SchedulerVersion{
		Version:   "v1.0",
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	return listSchedulerVersions
}
