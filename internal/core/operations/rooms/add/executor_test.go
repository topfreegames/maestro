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

package add

import (
	"time"

	clock_mock "github.com/topfreegames/maestro/internal/core/ports/clock_mock.go"

	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
)

func TestExecutor_Execute(t *testing.T) {
	clockMock := clock_mock.NewFakeClock(time.Now())

	definition := Definition{Amount: 10}

	container1 := game_room.Container{
		Name: "container1",
		Ports: []game_room.ContainerPort{
			{Protocol: "tcp"},
		},
	}

	container2 := game_room.Container{
		Name: "container2",
		Ports: []game_room.ContainerPort{
			{Protocol: "udp"},
		},
	}

	scheduler := entities.Scheduler{
		Name: "zooba_blue:1.0.0",
		Spec: game_room.Spec{
			Version:    "1.0.0",
			Containers: []game_room.Container{container1, container2},
		},
		PortRange: nil,
	}

	gameRoom := game_room.GameRoom{
		ID:          "game-1",
		SchedulerID: "zooba_blue:1.0.0",
		Version:     "1.0.0",
		Status:      game_room.GameStatusPending,
		LastPingAt:  clockMock.Now(),
	}

	gameRoomInstance := game_room.Instance{
		ID:          "game-1",
		SchedulerID: "game",
	}

	op := operation.Operation{
		ID:             "some-op-id",
		SchedulerName:  "zooba_blue:1.0.0",
		Status:         operation.StatusPending,
		DefinitionName: "zooba_blue:1.0.0",
	}

	t.Run("should succeed - all room creations succeed => return nil, no error", func(t *testing.T) {
		_, roomsManager, schedulerStorage, operationsManager := testSetup(t)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(&scheduler, nil)
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(10)

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager)
		err := executor.Execute(context.Background(), &op, &definition)

		require.Nil(t, err)
	})

	t.Run("should fail - some room creation fail, others succeed => returns unexpected error", func(t *testing.T) {
		_, roomsManager, schedulerStorage, operationsManager := testSetup(t)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), op.SchedulerName).Return(&scheduler, nil)

		gameRoomReady := gameRoom
		gameRoomReady.Status = game_room.GameStatusReady

		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(9)
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(nil, nil, porterrors.NewErrUnexpected("error"))
		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, "error while creating room: error")

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager)
		err := executor.Execute(context.Background(), &op, &definition)

		require.NotNil(t, err)
		require.ErrorContains(t, err, "error while creating room: error")
	})

	t.Run("should fail - no scheduler found => returns error", func(t *testing.T) {
		_, roomsManager, schedulerStorage, operationsManager := testSetup(t)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(nil, porterrors.NewErrNotFound("scheduler not found"))
		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, "error fetching scheduler from storage: scheduler not found")

		err := NewExecutor(roomsManager, schedulerStorage, operationsManager).Execute(context.Background(), &op, &definition)
		require.NotNil(t, err)
		require.ErrorContains(t, err, "error fetching scheduler from storage: scheduler not found")
	})
}

func TestExecutor_Rollback(t *testing.T) {

	definition := Definition{Amount: 10}

	op := operation.Operation{
		ID:             "some-op-id",
		SchedulerName:  "zooba_blue:1.0.0",
		Status:         operation.StatusPending,
		DefinitionName: "zooba_blue:1.0.0",
	}

	t.Run("does nothing and return nil", func(t *testing.T) {
		_, roomsManager, schedulerStorage, operationsManager := testSetup(t)

		rollbackErr := NewExecutor(roomsManager, schedulerStorage, operationsManager).Rollback(context.Background(), &op, &definition, nil)
		require.NoError(t, rollbackErr)
	})

}

func testSetup(t *testing.T) (*Executor, *mockports.MockRoomManager, *mockports.MockSchedulerStorage, *mockports.MockOperationManager) {
	mockCtrl := gomock.NewController(t)

	roomsManager := mockports.NewMockRoomManager(mockCtrl)
	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
	operationsManager := mockports.NewMockOperationManager(mockCtrl)
	executor := NewExecutor(roomsManager, schedulerStorage, operationsManager)
	return executor, roomsManager, schedulerStorage, operationsManager
}
