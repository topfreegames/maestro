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

package switchversion_test

import (
	"context"
	"errors"
	"testing"

	"github.com/topfreegames/maestro/internal/core/operations/rooms/add"
	"github.com/topfreegames/maestro/internal/core/operations/schedulers/switchversion"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/validations"
)

// mockRoomAndSchedulerManager struct that holds all the mocks necessary for the
// operation executor.
type mockRoomAndSchedulerAndOperationManager struct {
	schedulerManager *mockports.MockSchedulerManager
	operationManager *mockports.MockOperationManager
	portAllocator    *mockports.MockPortAllocator
	instanceStorage  *mockports.MockGameRoomInstanceStorage
	runtime          *mockports.MockRuntime
	eventsService    ports.EventsService
	schedulerStorage *mockports.MockSchedulerStorage
	schedulerCache   *mockports.MockSchedulerCache
}

func TestExecutor_Execute(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	newMajorScheduler := newValidSchedulerV2()

	t.Run("should succeed - Execute switch active version operation", func(t *testing.T) {
		definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(context.Background(), newMajorScheduler.Name, definition.NewActiveVersion).Return(newMajorScheduler, nil)
		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), newMajorScheduler).Return(nil)

		executor := switchversion.NewExecutor(mocks.schedulerManager, mocks.operationManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)

		require.Nil(t, execErr)
	})

	t.Run("should fail - Invalid definition received", func(t *testing.T) {
		invalidDef := &add.Definition{Amount: 2}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		executor := switchversion.NewExecutor(mocks.schedulerManager, mocks.operationManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, invalidDef)

		require.NotNil(t, execErr)
	})

	t.Run("should fail - Can not get scheduler by version", func(t *testing.T) {
		getSchedErr := errors.New("foobar")
		definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(context.Background(), newMajorScheduler.Name, definition.NewActiveVersion).Return(nil, getSchedErr)
		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

		executor := switchversion.NewExecutor(mocks.schedulerManager, mocks.operationManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)

		require.NotNil(t, execErr)
		require.ErrorIs(t, execErr, getSchedErr)
	})

	t.Run("should fail - Can not update scheduler", func(t *testing.T) {
		updateSchedErr := errors.New("foobar")
		definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)

		mocks.schedulerManager.EXPECT().GetSchedulerByVersion(context.Background(), newMajorScheduler.Name, definition.NewActiveVersion).Return(newMajorScheduler, nil)
		mocks.schedulerManager.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Return(updateSchedErr)
		mocks.operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

		executor := switchversion.NewExecutor(mocks.schedulerManager, mocks.operationManager)
		execErr := executor.Execute(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition)

		require.NotNil(t, execErr)
		require.ErrorIs(t, execErr, updateSchedErr)
	})
}

func TestExecutor_Rollback(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	newMajorScheduler := newValidSchedulerV2()

	t.Run("empty rollback", func(t *testing.T) {
		execErr := errors.New("foobar")
		definition := &switchversion.Definition{NewActiveVersion: newMajorScheduler.Spec.Version}
		mocks := newMockRoomAndSchedulerManager(mockCtrl)
		executor := switchversion.NewExecutor(mocks.schedulerManager, mocks.operationManager)
		rollbackErr := executor.Rollback(context.Background(), &operation.Operation{SchedulerName: newMajorScheduler.Name}, definition, execErr)

		require.Nil(t, rollbackErr)
	})
}

func TestExecutor_Name(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("returns operation name", func(t *testing.T) {
		mocks := newMockRoomAndSchedulerManager(mockCtrl)
		executor := switchversion.NewExecutor(mocks.schedulerManager, mocks.operationManager)
		exName := executor.Name()

		require.Equal(t, "switch_active_version", exName)
	})
}

func newMockRoomAndSchedulerManager(mockCtrl *gomock.Controller) *mockRoomAndSchedulerAndOperationManager {
	portAllocator := mockports.NewMockPortAllocator(mockCtrl)
	instanceStorage := mockports.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := mockports.NewMockRuntime(mockCtrl)
	eventsForwarderService := mockports.NewMockEventsService(mockCtrl)
	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
	schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)

	schedulerManager := mockports.NewMockSchedulerManager(mockCtrl)
	operationManager := mockports.NewMockOperationManager(mockCtrl)

	return &mockRoomAndSchedulerAndOperationManager{
		schedulerManager,
		operationManager,
		portAllocator,
		instanceStorage,
		runtime,
		eventsForwarderService,
		schedulerStorage,
		schedulerCache,
	}
}

func newValidSchedulerV2() *entities.Scheduler {
	return &entities.Scheduler{
		Name:            "scheduler",
		Game:            "game",
		State:           entities.StateInSync,
		MaxSurge:        "5",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v2",
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
