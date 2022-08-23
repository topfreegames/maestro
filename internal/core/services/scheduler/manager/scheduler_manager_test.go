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

package manager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/operations/scheduler/delete"
	"github.com/topfreegames/maestro/internal/core/operations/scheduler/version/new"

	"github.com/topfreegames/maestro/internal/core/services/scheduler/manager/patch"

	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/mock"

	"github.com/stretchr/testify/assert"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/filters"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	portsErrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/validations"
)

func TestCreateScheduler(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
	operationManager := mock.NewMockOperationManager(mockCtrl)
	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
	schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

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

	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
	operationManager := mock.NewMockOperationManager(mockCtrl)
	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
	schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

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
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

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
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)

		_, err := schedulerManager.EnqueueNewSchedulerVersionOperation(ctx, scheduler)
		require.Error(t, err)

	})

	t.Run("return error when scheduler is not found", func(t *testing.T) {
		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(nil, errors.NewErrUnexpected("some_error"))

		_, err := schedulerManager.EnqueueNewSchedulerVersionOperation(ctx, scheduler)
		require.Error(t, err)
	})

	t.Run("with failure", func(t *testing.T) {
		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

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
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(ctx, scheduler.Name, gomock.Any()).Return(&operation.Operation{}, nil)

		op, err := schedulerManager.EnqueueSwitchActiveVersionOperation(ctx, scheduler.Name, scheduler.Spec.Version)
		require.NoError(t, err)
		require.NotNil(t, op)
		require.NotNil(t, op.ID)

	})

	t.Run("return error when some error occurs while creating operation", func(t *testing.T) {
		currentScheduler := newValidScheduler()
		currentScheduler.PortRange = &entities.PortRange{Start: 1, End: 2}

		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(ctx, scheduler.Name, gomock.Any()).Return(nil, errors.NewErrUnexpected("storage offline"))

		op, err := schedulerManager.EnqueueSwitchActiveVersionOperation(ctx, scheduler.Name, scheduler.Spec.Version)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrUnexpected)
		require.Contains(t, err.Error(), "failed to schedule switch_active_version operation:")
	})
}

func TestDeleteSchedulerOperation(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	t.Run("return the operation when no error occurs using cache", func(t *testing.T) {
		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}
		opDef := &delete.DeleteSchedulerDefinition{}

		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(ctx, scheduler.Name, opDef).Return(&operation.Operation{}, nil)
		schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)

		op, err := schedulerManager.EnqueueDeleteSchedulerOperation(ctx, scheduler.Name)
		require.NoError(t, err)
		require.NotNil(t, op)
		require.NotNil(t, op.ID)
	})

	t.Run("return the operation when no error occurs using storage", func(t *testing.T) {
		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}
		opDef := &delete.DeleteSchedulerDefinition{}

		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(ctx, scheduler.Name, opDef).Return(&operation.Operation{}, nil)
		schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(nil, errors.ErrNotFound)
		schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)

		op, err := schedulerManager.EnqueueDeleteSchedulerOperation(ctx, scheduler.Name)
		require.NoError(t, err)
		require.NotNil(t, op)
		require.NotNil(t, op.ID)
	})

	t.Run("return error when some error occurs while creating operation", func(t *testing.T) {
		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}
		opDef := &delete.DeleteSchedulerDefinition{}

		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(ctx, scheduler.Name, opDef).Return(nil, errors.NewErrUnexpected("storage offline"))
		schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(scheduler, nil)

		op, err := schedulerManager.EnqueueDeleteSchedulerOperation(ctx, scheduler.Name)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrUnexpected)
		require.Contains(t, err.Error(), "failed to schedule delete_scheduler operation:")
	})

	t.Run("return error when can't find scheduler to delete", func(t *testing.T) {
		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		schedulerCache.EXPECT().GetScheduler(ctx, scheduler.Name).Return(nil, errors.ErrNotFound)
		schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(nil, errors.ErrNotFound)

		_, err = schedulerManager.EnqueueDeleteSchedulerOperation(ctx, scheduler.Name)
		require.Error(t, err)
	})

}

func TestGetSchedulerVersions(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("with success", func(t *testing.T) {
		scheduler := newValidScheduler()

		schedulerVersionList := newValidSchedulerVersionList()

		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetSchedulerVersions(ctx, scheduler.Name).Return(schedulerVersionList, nil)

		versions, err := schedulerManager.GetSchedulerVersions(ctx, scheduler.Name)
		require.NoError(t, err)
		require.NotNil(t, versions)
		require.Equal(t, versions, schedulerVersionList)
	})

	t.Run("error", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetSchedulerVersions(ctx, scheduler.Name).Return(nil, errors.NewErrNotFound("scheduler not found"))

		versions, err := schedulerManager.GetSchedulerVersions(ctx, scheduler.Name)
		require.Error(t, err)
		require.Nil(t, versions)
	})
}

func TestGetSchedulerByVersion(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("with success", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetSchedulerWithFilter(ctx, &filters.SchedulerFilter{
			Name:    scheduler.Name,
			Version: scheduler.Spec.Version,
		}).Return(scheduler, nil)

		schedulerReturned, err := schedulerManager.GetSchedulerByVersion(ctx, scheduler.Name, scheduler.Spec.Version)
		require.NoError(t, err)
		require.NotNil(t, schedulerReturned)
	})

	t.Run("fail", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetSchedulerWithFilter(ctx, &filters.SchedulerFilter{
			Name:    scheduler.Name,
			Version: scheduler.Spec.Version,
		}).Return(nil, errors.NewErrUnexpected("error"))

		schedulerReturned, err := schedulerManager.GetSchedulerByVersion(ctx, scheduler.Name, scheduler.Spec.Version)
		require.Error(t, err)
		require.Nil(t, schedulerReturned)
	})
}

func TestGetScheduler(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("with success", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

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
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

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
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulers := []*entities.Scheduler{scheduler}
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetSchedulersWithFilter(ctx, gomock.Any()).Return(schedulers, nil)

		retScheduler, err := schedulerManager.GetSchedulersWithFilter(ctx, schedulerFilter)
		require.NoError(t, err)
		require.NotNil(t, retScheduler)
		require.Equal(t, retScheduler, schedulers)
	})

	t.Run("when some error occurs returns error", func(t *testing.T) {
		ctx := context.Background()
		schedulerFilter := &filters.SchedulerFilter{}
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		schedulerStorage.EXPECT().GetSchedulersWithFilter(ctx, gomock.Any()).Return(nil, errors.NewErrUnexpected("some error"))

		retScheduler, err := schedulerManager.GetSchedulersWithFilter(ctx, schedulerFilter)
		require.Error(t, err, "some error")
		require.Empty(t, retScheduler)
	})
}

func TestUpdateScheduler(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	t.Run("with success", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()

		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		schedulerStorage.EXPECT().UpdateScheduler(ctx, scheduler).Return(nil)
		schedulerCache.EXPECT().DeleteScheduler(ctx, scheduler.Name).Return(nil)

		err := schedulerManager.UpdateScheduler(ctx, scheduler)
		require.NoError(t, err)
	})

	t.Run("with success - deleting cache fails", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()

		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		schedulerStorage.EXPECT().UpdateScheduler(ctx, scheduler).Return(nil)
		schedulerCache.EXPECT().DeleteScheduler(ctx, scheduler.Name).Return(errors.NewErrUnexpected("error"))

		err := schedulerManager.UpdateScheduler(ctx, scheduler)
		require.NoError(t, err)
	})

	t.Run("with valid scheduler it returns with error if some error occurs when update scheduler on storage", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()

		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		schedulerStorage.EXPECT().UpdateScheduler(ctx, scheduler).Return(errors.NewErrUnexpected("error"))

		err := schedulerManager.UpdateScheduler(ctx, scheduler)
		require.Error(t, err)
	})

	t.Run("with invalid scheduler it return invalid scheduler error", func(t *testing.T) {
		scheduler := newInvalidScheduler()
		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		err := schedulerManager.CreateNewSchedulerVersion(ctx, scheduler)

		require.Error(t, err)
	})
}

func TestGetSchedulersInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Run("with valid request it returns a list of scheduler and game rooms information", func(t *testing.T) {
		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil, nil, roomStorage)
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
		require.Equal(t, 2, schedulersInfo[0].RoomsReplicas)
		require.Equal(t, 5, schedulersInfo[0].RoomsReady)
		require.Equal(t, 10, schedulersInfo[0].RoomsPending)
		require.Equal(t, 15, schedulersInfo[0].RoomsOccupied)
		require.Equal(t, 20, schedulersInfo[0].RoomsTerminating)
	})

	t.Run("it returns with error when no scheduler and game rooms info was founded", func(t *testing.T) {
		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil, nil, roomStorage)
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
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil, nil, roomStorage)
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
	t.Run("with valid request it returns a scheduler and game rooms information (no autoscaling)", func(t *testing.T) {
		ctx := context.Background()
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(nil, nil, nil, roomStorage)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(5, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(10, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(15, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(20, nil)
		scheduler := newValidScheduler()

		schedulersInfo, err := schedulerManager.newSchedulerInfo(ctx, scheduler)

		require.NoError(t, err)
		require.Equal(t, 5, schedulersInfo.RoomsReady)
		require.Equal(t, 10, schedulersInfo.RoomsPending)
		require.Equal(t, 15, schedulersInfo.RoomsOccupied)
		require.Equal(t, 20, schedulersInfo.RoomsTerminating)
		require.Nil(t, schedulersInfo.Autoscaling)
	})

	t.Run("with valid request it returns a scheduler and game rooms information and autoscaling", func(t *testing.T) {
		ctx := context.Background()
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(nil, nil, nil, roomStorage)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(5, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(10, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(15, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(20, nil)
		scheduler := newValidScheduler()

		scheduler.Autoscaling = &autoscaling.Autoscaling{
			Enabled: true,
			Min:     1,
			Max:     5,
		}

		schedulersInfo, err := schedulerManager.newSchedulerInfo(ctx, scheduler)

		require.NoError(t, err)
		require.Equal(t, 5, schedulersInfo.RoomsReady)
		require.Equal(t, 10, schedulersInfo.RoomsPending)
		require.Equal(t, 15, schedulersInfo.RoomsOccupied)
		require.Equal(t, 20, schedulersInfo.RoomsTerminating)
		require.NotNil(t, schedulersInfo.Autoscaling)
		require.Equal(t, 5, schedulersInfo.Autoscaling.Max)
		require.Equal(t, 1, schedulersInfo.Autoscaling.Min)
		require.Equal(t, true, schedulersInfo.Autoscaling.Enabled)
	})

	t.Run("it returns with error when couldn't get game rooms information in ready state", func(t *testing.T) {
		ctx := context.Background()
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(nil, nil, nil, roomStorage)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, errors.NewErrUnexpected("err"))
		scheduler := newValidScheduler()

		schedulersInfo, err := schedulerManager.newSchedulerInfo(ctx, scheduler)

		require.Error(t, err)
		require.Nil(t, schedulersInfo)
	})

	t.Run("it returns with error when couldn't get game rooms information in pending state", func(t *testing.T) {
		ctx := context.Background()
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(nil, nil, nil, roomStorage)
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
		schedulerManager := NewSchedulerManager(nil, nil, nil, roomStorage)
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
		schedulerManager := NewSchedulerManager(nil, nil, nil, roomStorage)
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

func TestDeleteScheduler(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Run("it returns with success when scheduler are found in database", func(t *testing.T) {
		schedulerName := "scheduler-name"
		scheduler := newValidScheduler()
		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil, nil, nil)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), schedulerName).Return(scheduler, nil)
		schedulerStorage.EXPECT().DeleteScheduler(gomock.Any(), ports.TransactionID(""), scheduler).Return(nil)

		err := schedulerManager.DeleteScheduler(ctx, schedulerName)

		assert.NoError(t, err)
	})

	t.Run("it returns with error when couldn't found scheduler in database", func(t *testing.T) {
		schedulerName := "scheduler-name"
		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil, nil, nil)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), schedulerName).Return(nil, errors.NewErrNotFound("err"))

		err := schedulerManager.DeleteScheduler(ctx, schedulerName)

		assert.Error(t, err)
		assert.ErrorIs(t, err, errors.ErrNotFound)
		assert.Contains(t, err.Error(), "no scheduler found to delete")
	})

	t.Run("it returns with error when couldn't delete scheduler from database", func(t *testing.T) {
		schedulerName := "scheduler-name"
		scheduler := newValidScheduler()
		ctx := context.Background()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil, nil, nil)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), schedulerName).Return(scheduler, nil)
		schedulerStorage.EXPECT().DeleteScheduler(gomock.Any(), ports.TransactionID(""), scheduler).Return(errors.NewErrUnexpected("err"))

		err := schedulerManager.DeleteScheduler(ctx, schedulerName)

		assert.Error(t, err)
		assert.ErrorIs(t, err, errors.ErrUnexpected)
		assert.Contains(t, err.Error(), "not able to delete scheduler")
	})
}

func TestPatchSchedulerAndSwitchActiveVersionOperation(t *testing.T) {
	type Input struct {
		PatchMap map[string]interface{}
	}
	type ExpectedMock struct {
		GetSchedulerError        error
		ChangedSchedulerFunction func() *entities.Scheduler
		CreateOperationReturn    *operation.Operation
		CreateOperationError     error
	}
	type Output struct {
		*operation.Operation
		Err error
	}

	scheduler := newValidScheduler()

	testCases := []struct {
		Title string
		Input
		ExpectedMock
		Output
	}{
		{
			Title: "There are parameters then send scheduler to CreateNewSchedulerVersionDefinition to changing it",
			Input: Input{
				PatchMap: map[string]interface{}{
					patch.LabelSchedulerMaxSurge: "12%",
				},
			},
			ExpectedMock: ExpectedMock{
				GetSchedulerError: nil,
				ChangedSchedulerFunction: func() *entities.Scheduler {
					scheduler.MaxSurge = "12%"
					return scheduler
				},
				CreateOperationReturn: &operation.Operation{ID: "some-id"},
				CreateOperationError:  nil,
			},
			Output: Output{
				Operation: &operation.Operation{ID: "some-id"},
				Err:       nil,
			},
		},
		{
			Title: "GetScheduler returns error not found then return not found error",
			Input: Input{
				PatchMap: map[string]interface{}{
					patch.LabelSchedulerMaxSurge: "12%",
				},
			},
			ExpectedMock: ExpectedMock{
				GetSchedulerError: portsErrors.NewErrNotFound("scheduler not found"),
				ChangedSchedulerFunction: func() *entities.Scheduler {
					scheduler.MaxSurge = "15%"
					return scheduler
				},
				CreateOperationReturn: nil,
				CreateOperationError:  nil,
			},
			Output: Output{
				Operation: nil,
				Err:       fmt.Errorf("no scheduler found, can not create new version for inexistent scheduler:"),
			},
		},
		{
			Title: "GetScheduler returns in error then return error",
			Input: Input{
				PatchMap: map[string]interface{}{
					patch.LabelSchedulerMaxSurge: "12%",
				},
			},
			ExpectedMock: ExpectedMock{
				GetSchedulerError: fmt.Errorf("error on get scheduler"),
				ChangedSchedulerFunction: func() *entities.Scheduler {
					scheduler.MaxSurge = "15%"
					return scheduler
				},
				CreateOperationReturn: nil,
				CreateOperationError:  nil,
			},
			Output: Output{
				Operation: nil,
				Err:       fmt.Errorf("unexpected error getting scheduler to patch:"),
			},
		},
		{
			Title: "Scheduler validation returns in error then return ErrInvalidArgument",
			Input: Input{
				PatchMap: map[string]interface{}{
					patch.LabelSchedulerMaxSurge: "potato",
				},
			},
			ExpectedMock: ExpectedMock{
				GetSchedulerError: nil,
				ChangedSchedulerFunction: func() *entities.Scheduler {
					scheduler.MaxSurge = "17%"
					return scheduler
				},
				CreateOperationReturn: nil,
				CreateOperationError:  nil,
			},
			Output: Output{
				Operation: nil,
				Err:       fmt.Errorf("invalid patched scheduler:"),
			},
		},
		{
			Title: "CreateOperation returns in error then return error",
			Input: Input{
				PatchMap: map[string]interface{}{
					patch.LabelSchedulerMaxSurge: "17%",
				},
			},
			ExpectedMock: ExpectedMock{
				GetSchedulerError: nil,
				ChangedSchedulerFunction: func() *entities.Scheduler {
					scheduler.MaxSurge = "17%"
					return scheduler
				},
				CreateOperationReturn: nil,
				CreateOperationError:  fmt.Errorf("error on create operation"),
			},
			Output: Output{
				Operation: nil,
				Err:       fmt.Errorf("failed to schedule create_new_scheduler_version operation:"),
			},
		},
		{
			Title: "PatchScheduler returns in error then return error",
			Input: Input{
				PatchMap: map[string]interface{}{
					patch.LabelSchedulerPortRange: "wrong-port-range-format",
				},
			},
			ExpectedMock: ExpectedMock{
				GetSchedulerError: nil,
				ChangedSchedulerFunction: func() *entities.Scheduler {
					scheduler.MaxSurge = "19%"
					return scheduler
				},
				CreateOperationReturn: nil,
				CreateOperationError:  nil,
			},
			Output: Output{
				Operation: nil,
				Err:       fmt.Errorf("error patching scheduler:"),
			},
		},
	}

	err := validations.RegisterValidations()
	require.NoError(t, err)

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			ctx := context.Background()
			mockCtrl := gomock.NewController(t)
			mockOperationManager := mock.NewMockOperationManager(mockCtrl)
			mockSchedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
			schedulerManager := NewSchedulerManager(mockSchedulerStorage, nil, mockOperationManager, nil)

			mockSchedulerStorage.EXPECT().GetScheduler(gomock.Any(), scheduler.Name).Return(scheduler, testCase.ExpectedMock.GetSchedulerError)
			mockOperationManager.EXPECT().
				CreateOperation(gomock.Any(), scheduler.Name, gomock.Any()).Do(func(_ context.Context, _ string, op operations.Definition) {
				newSchedulerVersion, ok := op.(*new.CreateNewSchedulerVersionDefinition)
				assert.True(t, ok)
				assert.EqualValues(t, testCase.ExpectedMock.ChangedSchedulerFunction(), newSchedulerVersion.NewScheduler)
			}).
				Return(testCase.ExpectedMock.CreateOperationReturn, testCase.ExpectedMock.CreateOperationError).
				AnyTimes()

			op, err := schedulerManager.PatchSchedulerAndCreateNewSchedulerVersionOperation(ctx, scheduler.Name, testCase.Input.PatchMap)
			if testCase.Output.Err != nil {
				assert.ErrorContains(t, err, testCase.Output.Err.Error())

				return
			}

			assert.NoError(t, err)
			assert.EqualValues(t, testCase.Output.Operation, op)
		})
	}
}

// newValidScheduler generates a valid scheduler with the required fields.
func newValidScheduler() *entities.Scheduler {
	return &entities.Scheduler{
		Name:            "scheduler",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "10%",
		RollbackVersion: "v1.0.0",
		RoomsReplicas:   2,
		Spec: game_room.Spec{
			Version:                "v1.1.0",
			TerminationGracePeriod: 60,
			Toleration:             "toleration",
			Affinity:               "affinity",
			Containers: []game_room.Container{
				{
					Name:            "default",
					Image:           "some-image",
					ImagePullPolicy: "IfNotPresent",
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

// newInvalidScheduler generate an invalid Scheduler,.
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
					ImagePullPolicy: "IfNotPresent",
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
