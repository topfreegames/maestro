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
	"fmt"
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

	config := Config{
		AmountLimit: 10,
	}

	op := operation.Operation{
		ID:             "some-op-id",
		SchedulerName:  "zooba_blue:1.0.0",
		Status:         operation.StatusPending,
		DefinitionName: "zooba_blue:1.0.0",
	}

	t.Run("should succeed - all room creations succeed => return nil, no error", func(t *testing.T) {
		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, config)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(&scheduler, nil)
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(10)
		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, fmt.Sprintf("added %d rooms", definition.Amount))

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, config)
		err := executor.Execute(context.Background(), &op, &definition)

		require.Nil(t, err)
	})

	t.Run("should fail - some room creation fail, others succeed => returns unexpected error", func(t *testing.T) {
		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, config)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), op.SchedulerName).Return(&scheduler, nil)

		gameRoomReady := gameRoom
		gameRoomReady.Status = game_room.GameStatusReady

		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(9)
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(nil, nil, porterrors.NewErrUnexpected("error"))
		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, "error while creating room: error")

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, config)
		err := executor.Execute(context.Background(), &op, &definition)

		require.NotNil(t, err)
		require.ErrorContains(t, err, "error while creating room: error")
	})

	t.Run("should fail - no scheduler found => returns error", func(t *testing.T) {
		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, config)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(nil, porterrors.NewErrNotFound("scheduler not found"))
		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, "error fetching scheduler from storage: scheduler not found")

		err := NewExecutor(roomsManager, schedulerStorage, operationsManager, config).Execute(context.Background(), &op, &definition)
		require.NotNil(t, err)
		require.ErrorContains(t, err, "error fetching scheduler from storage: scheduler not found")
	})

	t.Run("use default amount limit if AmountLimit not set", func(t *testing.T) {
		executor, _, _, _ := testSetup(t, Config{})
		require.NotNil(t, executor)
		require.Equal(t, executor.config.AmountLimit, int32(DefaultAmountLimit))
	})

	t.Run("use default amount limit if invalid AmountLimit value", func(t *testing.T) {
		executor, _, _, _ := testSetup(t, Config{AmountLimit: -1})
		require.NotNil(t, executor)
		require.Equal(t, executor.config.AmountLimit, int32(DefaultAmountLimit))
	})

	t.Run("should succeed - cap to the default amount if asked to create more than whats configured", func(t *testing.T) {
		bigAmountDefinition := Definition{Amount: 10000}
		smallDefaultConfig := Config{AmountLimit: 10}

		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, smallDefaultConfig)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(&scheduler, nil)
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(int(smallDefaultConfig.AmountLimit))
		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, fmt.Sprintf("added %d rooms", smallDefaultConfig.AmountLimit))

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, smallDefaultConfig)
		err := executor.Execute(context.Background(), &op, &bigAmountDefinition)

		require.Nil(t, err)
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

	config := Config{
		AmountLimit: 10,
	}

	t.Run("does nothing and return nil", func(t *testing.T) {
		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, config)

		rollbackErr := NewExecutor(roomsManager, schedulerStorage, operationsManager, config).Rollback(context.Background(), &op, &definition, nil)
		require.NoError(t, rollbackErr)
	})

}

func testSetup(t *testing.T, cfg Config) (*Executor, *mockports.MockRoomManager, *mockports.MockSchedulerStorage, *mockports.MockOperationManager) {
	mockCtrl := gomock.NewController(t)

	roomsManager := mockports.NewMockRoomManager(mockCtrl)
	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
	operationsManager := mockports.NewMockOperationManager(mockCtrl)
	executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, cfg)
	return executor, roomsManager, schedulerStorage, operationsManager
}
