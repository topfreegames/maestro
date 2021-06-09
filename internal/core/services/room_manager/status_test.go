//+build unit

package room_manager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	clockmock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	ismock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	pamock "github.com/topfreegames/maestro/internal/adapters/port_allocator/mock"
	rsmock "github.com/topfreegames/maestro/internal/adapters/room_storage/mock"
	runtimemock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

func TestRoomManager_SetRoomStatus_SuccessTransitions(t *testing.T) {
	for fromStatus, transitions := range validStatusTransitions {
		for transition, _ := range transitions {
			t.Run(fmt.Sprintf("transition from %s to %s", fromStatus.String(), transition.String()), func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()

				roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
				roomManager := NewRoomManager(
					clockmock.NewFakeClock(time.Now()),
					pamock.NewMockPortAllocator(mockCtrl),
					roomStorage,
					ismock.NewMockGameRoomInstanceStorage(mockCtrl),
					runtimemock.NewMockRuntime(mockCtrl),
				)

				gameRoom := &game_room.GameRoom{ID: "transition-test", SchedulerID: "scheduler-test", Status: fromStatus}
				roomStorage.EXPECT().SetRoomStatus(context.Background(), gameRoom.SchedulerID, gameRoom.ID, transition).Return(nil)
				err := roomManager.SetRoomStatus(context.Background(), gameRoom, transition)
				require.NoError(t, err)
			})
		}
	}
}

func TestRoomManager_SetRoomStatus_InvalidTransition(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	roomManager := NewRoomManager(
		clockmock.NewFakeClock(time.Now()),
		pamock.NewMockPortAllocator(mockCtrl),
		rsmock.NewMockRoomStorage(mockCtrl),
		ismock.NewMockGameRoomInstanceStorage(mockCtrl),
		runtimemock.NewMockRuntime(mockCtrl),
	)

	transition := game_room.GameStatusReady
	gameRoom := &game_room.GameRoom{ID: "transition-test", SchedulerID: "scheduler-test", Status: game_room.GameStatusTerminating}
	err := roomManager.SetRoomStatus(context.Background(), gameRoom, transition)
	require.Error(t, err)
}

func TestRoomManager_SetRoomStatus_StorageFailure(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	roomManager := NewRoomManager(
		clockmock.NewFakeClock(time.Now()),
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		ismock.NewMockGameRoomInstanceStorage(mockCtrl),
		runtimemock.NewMockRuntime(mockCtrl),
	)

	transition := game_room.GameStatusReady
	gameRoom := &game_room.GameRoom{ID: "transition-test", SchedulerID: "scheduler-test", Status: game_room.GameStatusPending}
	roomStorage.EXPECT().SetRoomStatus(context.Background(), gameRoom.SchedulerID, gameRoom.ID, transition).Return(fmt.Errorf("something bad happened"))
	err := roomManager.SetRoomStatus(context.Background(), gameRoom, transition)
	require.Error(t, err)
}
