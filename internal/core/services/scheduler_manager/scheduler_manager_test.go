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

	"github.com/Masterminds/semver/v3"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	opflow "github.com/topfreegames/maestro/internal/adapters/operation_flow/mock"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	schedulerStorageMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/filters"
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

func TestUpdateSchedulerConfig(t *testing.T) {
	mockSchedulerManager := func(ctrl *gomock.Controller) (*SchedulerManager, *schedulerStorageMock.MockSchedulerStorage) {
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(ctrl)
		return NewSchedulerManager(schedulerStorage, nil), schedulerStorage
	}

	t.Run("major update with a valid scheduler and update succeeds should return no error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		schedulerManager, schedulerStorage := mockSchedulerManager(mockCtrl)
		ctx := context.Background()

		// ensure port range has a specific value
		currentScheduler := newValidScheduler()
		currentScheduler.PortRange = &entities.PortRange{Start: 1, End: 2}

		// update scheduler port range
		newScheduler := newValidScheduler()
		newScheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		schedulerStorage.EXPECT().GetScheduler(ctx, newScheduler.Name).Return(currentScheduler, nil)
		schedulerStorage.EXPECT().UpdateScheduler(ctx, gomock.Any()).Return(nil)

		isMajor, err := schedulerManager.UpdateSchedulerConfig(ctx, newScheduler)
		require.NoError(t, err)
		require.True(t, isMajor)

		prevVersion := semver.MustParse(currentScheduler.Spec.Version)
		newVersion := semver.MustParse(newScheduler.Spec.Version)
		require.Greater(t, newVersion.Major(), prevVersion.Major())
		require.Equal(t, currentScheduler.Spec.Version, newScheduler.RollbackVersion)
	})

	t.Run("minor update with a valid scheduler and update succeeds should return no error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		schedulerManager, schedulerStorage := mockSchedulerManager(mockCtrl)
		ctx := context.Background()

		// ensure max surge has a specific value
		currentScheduler := newValidScheduler()
		currentScheduler.MaxSurge = "10%"

		// update scheduler max surge
		newScheduler := newValidScheduler()
		newScheduler.MaxSurge = "10%"

		schedulerStorage.EXPECT().GetScheduler(ctx, newScheduler.Name).Return(currentScheduler, nil)
		schedulerStorage.EXPECT().UpdateScheduler(ctx, gomock.Any()).Return(nil)

		isMajor, err := schedulerManager.UpdateSchedulerConfig(ctx, newScheduler)
		require.NoError(t, err)
		require.False(t, isMajor)

		prevVersion := semver.MustParse(currentScheduler.Spec.Version)
		newVersion := semver.MustParse(newScheduler.Spec.Version)
		require.Equal(t, prevVersion.Major(), newVersion.Major())
		require.Greater(t, newVersion.Minor(), prevVersion.Minor())
		require.Equal(t, currentScheduler.Spec.Version, newScheduler.RollbackVersion)
	})

	t.Run("major update with a valid scheduler and update fails should return error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		schedulerManager, schedulerStorage := mockSchedulerManager(mockCtrl)
		ctx := context.Background()

		// ensure port range has a specific value
		currentScheduler := newValidScheduler()
		currentScheduler.PortRange = &entities.PortRange{Start: 1, End: 2}

		// update scheduler port range
		newScheduler := newValidScheduler()
		newScheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		schedulerStorage.EXPECT().GetScheduler(ctx, newScheduler.Name).Return(currentScheduler, nil)
		schedulerStorage.EXPECT().UpdateScheduler(ctx, gomock.Any()).Return(errors.ErrUnexpected)

		_, err := schedulerManager.UpdateSchedulerConfig(ctx, newScheduler)
		require.Error(t, err)
	})

	t.Run("valid scheduler but not found should return error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		schedulerManager, schedulerStorage := mockSchedulerManager(mockCtrl)
		ctx := context.Background()

		newScheduler := newValidScheduler()
		schedulerStorage.EXPECT().GetScheduler(ctx, newScheduler.Name).Return(nil, errors.ErrNotFound)

		_, err := schedulerManager.UpdateSchedulerConfig(ctx, newScheduler)
		require.Error(t, err)
	})

	t.Run("invalid scheduler should return error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		schedulerManager, _ := mockSchedulerManager(mockCtrl)
		ctx := context.Background()

		// update scheduler port range
		newScheduler := &entities.Scheduler{}

		_, err := schedulerManager.UpdateSchedulerConfig(ctx, newScheduler)
		require.Error(t, err)
	})
}

func TestIsMajorVersionUpdate(t *testing.T) {
	tests := map[string]struct {
		currentScheduler *entities.Scheduler
		newScheduler     *entities.Scheduler
		expected         bool
	}{
		"port range should be a major update": {
			currentScheduler: &entities.Scheduler{PortRange: &entities.PortRange{Start: 1000, End: 2000}},
			newScheduler:     &entities.Scheduler{PortRange: &entities.PortRange{Start: 1001, End: 2000}},
			expected:         true,
		},
		"container resources should be a major update": {
			currentScheduler: &entities.Scheduler{Spec: game_room.Spec{
				Containers: []game_room.Container{
					{Requests: game_room.ContainerResources{Memory: "100mi"}},
				},
			}},
			newScheduler: &entities.Scheduler{Spec: game_room.Spec{
				Containers: []game_room.Container{
					{Requests: game_room.ContainerResources{Memory: "200mi"}},
				},
			}},
			expected: true,
		},
		"no changes shouldn't be a major": {
			currentScheduler: &entities.Scheduler{PortRange: &entities.PortRange{Start: 1000, End: 2000}},
			newScheduler:     &entities.Scheduler{PortRange: &entities.PortRange{Start: 1000, End: 2000}},
			expected:         false,
		},
		"max surge shouldn't be a major": {
			currentScheduler: &entities.Scheduler{MaxSurge: "10"},
			newScheduler:     &entities.Scheduler{MaxSurge: "100"},
			expected:         false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			isMajor := isMajorVersionUpdate(test.currentScheduler, test.newScheduler)
			require.Equal(t, test.expected, isMajor)
		})
	}
}

func TestCreateSchedulerOperation(t *testing.T) {

	t.Run("with success", func(t *testing.T) {

		currentScheduler := newValidScheduler()
		currentScheduler.PortRange = &entities.PortRange{Start: 1, End: 2}

		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors())
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager)

		operationStorage.EXPECT().CreateOperation(ctx, gomock.Any(), gomock.Any()).Return(nil)
		operationFlow.EXPECT().InsertOperationID(ctx, scheduler.Name, gomock.Any()).Return(nil)
		schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(currentScheduler, nil)

		op, err := schedulerManager.CreateUpdateSchedulerOperation(ctx, scheduler)
		require.NoError(t, err)
		require.NotNil(t, op)
		require.NotNil(t, op.ID)

	})

	t.Run("fails when scheduler does not exists", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(nil, errors.NewErrNotFound("err"))

		op, err := schedulerManager.CreateUpdateSchedulerOperation(ctx, scheduler)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrNotFound)
		require.Contains(t, err.Error(), "no scheduler found to be updated: err")
	})

	t.Run("with failure", func(t *testing.T) {
		currentScheduler := newValidScheduler()
		currentScheduler.PortRange = &entities.PortRange{Start: 1, End: 2}

		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(nil, operationStorage, operations.NewDefinitionConstructors())
		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager)

		schedulerStorage.EXPECT().GetScheduler(ctx, scheduler.Name).Return(currentScheduler, nil)
		operationStorage.EXPECT().CreateOperation(ctx, gomock.Any(), gomock.Any()).Return(errors.NewErrUnexpected("storage offline"))

		op, err := schedulerManager.CreateUpdateSchedulerOperation(ctx, scheduler)
		require.Nil(t, op)
		require.ErrorIs(t, err, errors.ErrUnexpected)
		require.Contains(t, err.Error(), "failed to schedule 'update scheduler' operation")
	})
}

func TestGetSchedulerVersions(t *testing.T) {

	t.Run("with success", func(t *testing.T) {
		scheduler := newValidScheduler()

		schedulerVersionList := newValidSchedulerVersionList()

		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors())

		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager)

		schedulerStorage.EXPECT().GetSchedulerVersions(ctx, scheduler.Name).Return(schedulerVersionList, nil)

		versions, err := schedulerManager.GetSchedulerVersions(ctx, scheduler.Name)
		require.NoError(t, err)
		require.NotNil(t, versions)
		require.Equal(t, versions, schedulerVersionList)
	})

	t.Run("error", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors())

		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager)

		schedulerStorage.EXPECT().GetSchedulerVersions(ctx, scheduler.Name).Return(nil, errors.NewErrNotFound("scheduler not found"))

		versions, err := schedulerManager.GetSchedulerVersions(ctx, scheduler.Name)
		require.Error(t, err)
		require.Nil(t, versions)
	})
}

func TestGetScheduler(t *testing.T) {

	t.Run("with success", func(t *testing.T) {
		scheduler := newValidScheduler()

		ctx := context.Background()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors())

		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager)

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
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors())

		schedulerManager := NewSchedulerManager(schedulerStorage, operationManager)

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

// newValidSchedulerVersionList generates a valid list with SchedulerVersions.
func newValidSchedulerVersionList() []*entities.SchedulerVersion {
	listSchedulerVersions := make([]*entities.SchedulerVersion, 2)
	listSchedulerVersions[0] = &entities.SchedulerVersion{
		Version:   "v2.0",
		CreatedAt: time.Now(),
	}
	listSchedulerVersions[1] = &entities.SchedulerVersion{
		Version:   "v1.0",
		CreatedAt: time.Now(),
	}
	return listSchedulerVersions
}
