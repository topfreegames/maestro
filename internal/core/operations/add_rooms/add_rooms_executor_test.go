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

package add_rooms

import (
	"time"

	"github.com/topfreegames/maestro/internal/core/operations"

	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	clock_mock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	serviceerrors "github.com/topfreegames/maestro/internal/core/services/errors"

	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
)

func TestAddRoomsExecutor_Execute(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	clockMock := clock_mock.NewFakeClock(time.Now())
	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)

	definition := AddRoomsDefinition{Amount: 10}

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
		roomsManager := mockports.NewMockRoomManager(mockCtrl)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(&scheduler, nil)
		roomsManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(10)

		executor := NewExecutor(roomsManager, schedulerStorage)
		err := executor.Execute(context.Background(), &op, &definition)

		require.Nil(t, err)
	})

	t.Run("should fail - some room creation fail, others succeed => returns unexpected error", func(t *testing.T) {
		roomsManager := mockports.NewMockRoomManager(mockCtrl)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), op.SchedulerName).Return(&scheduler, nil)

		gameRoomReady := gameRoom
		gameRoomReady.Status = game_room.GameStatusReady

		roomsManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(9)
		roomsManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), false).Return(nil, nil, porterrors.NewErrUnexpected("error"))

		executor := NewExecutor(roomsManager, schedulerStorage)
		err := executor.Execute(context.Background(), &op, &definition)

		require.NotNil(t, err)
		require.Equal(t, err.Kind(), operations.ErrKindUnexpected)
	})

	t.Run("should fail - some room creation fail with timeout => returns timeout error", func(t *testing.T) {
		roomsManager := mockports.NewMockRoomManager(mockCtrl)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), op.SchedulerName).Return(&scheduler, nil)

		gameRoomReady := gameRoom
		gameRoomReady.Status = game_room.GameStatusReady

		roomsManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(9)
		roomsManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), false).Return(nil, nil, serviceerrors.NewErrGameRoomStatusWaitingTimeout("context deadline exceeded"))

		executor := NewExecutor(roomsManager, schedulerStorage)
		err := executor.Execute(context.Background(), &op, &definition)

		require.NotNil(t, err)
		require.Equal(t, err.Kind(), operations.ErrKindReadyPingTimeout)
	})

	t.Run("should fail - no scheduler found => returns error", func(t *testing.T) {
		roomsManager := mockports.NewMockRoomManager(mockCtrl)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(nil, porterrors.NewErrNotFound("scheduler not found"))

		err := NewExecutor(roomsManager, schedulerStorage).Execute(context.Background(), &op, &definition)
		require.NotNil(t, err)
		require.Equal(t, err.Kind(), operations.ErrKindUnexpected)
	})
}

func TestAddRoomsExecutor_Rollback(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	clockMock := clock_mock.NewFakeClock(time.Now())
	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)

	definition := AddRoomsDefinition{Amount: 10}

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

	t.Run("when no error occurs it deletes previously created rooms and return without error", func(t *testing.T) {
		roomsManager := mockports.NewMockRoomManager(mockCtrl)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), op.SchedulerName).Return(&scheduler, nil)

		roomsManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(9)
		roomsManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), false).Return(nil, nil, porterrors.NewErrUnexpected("error"))

		executor := NewExecutor(roomsManager, schedulerStorage)
		err := executor.Execute(context.Background(), &op, &definition)

		require.NotNil(t, err)

		roomsManager.EXPECT().DeleteRoomAndWaitForRoomTerminated(gomock.Any(), gomock.Any()).Return(nil).Times(9)
		rollbackErr := executor.Rollback(context.Background(), &op, &definition, nil)
		require.NoError(t, rollbackErr)
	})

	t.Run("when some error occurs while deleting rooms it returns error", func(t *testing.T) {
		roomsManager := mockports.NewMockRoomManager(mockCtrl)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), op.SchedulerName).Return(&scheduler, nil)

		roomsManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(9)
		roomsManager.EXPECT().CreateRoomAndWaitForReadiness(gomock.Any(), gomock.Any(), false).Return(nil, nil, porterrors.NewErrUnexpected("error"))

		executor := NewExecutor(roomsManager, schedulerStorage)
		err := executor.Execute(context.Background(), &op, &definition)

		require.NotNil(t, err)

		roomsManager.EXPECT().DeleteRoomAndWaitForRoomTerminated(gomock.Any(), gomock.Any()).Return(porterrors.NewErrUnexpected("error")).Times(1)

		rollbackErr = executor.Rollback(context.Background(), &op, &definition, nil)
		require.Error(t, rollbackErr)
	})

}
