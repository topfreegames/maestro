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
	"golang.org/x/sync/errgroup"
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

func TestRoomManager_WaitGameRoomStatus(t *testing.T) {
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

	var group errgroup.Group
	waitCalled := make(chan struct{})
	group.Go(func() error {
		waitCalled <- struct{}{}
		roomStorage.EXPECT().GetRoom(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(gameRoom, nil)
		return roomManager.WaitRoomStatus(context.Background(), gameRoom, transition)
	})

	<-waitCalled

	roomStorage.EXPECT().SetRoomStatus(context.Background(), gameRoom.SchedulerID, gameRoom.ID, transition).Return(nil)
	err := roomManager.SetRoomStatus(context.Background(), gameRoom, transition)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		err := group.Wait()
		require.NoError(t, err)
		return err == nil
	}, 2*time.Second, time.Second)

	roomStorage.EXPECT().SetRoomStatus(context.Background(), gameRoom.SchedulerID, gameRoom.ID, game_room.GameStatusOccupied).Return(nil)
	err = roomManager.SetRoomStatus(context.Background(), gameRoom, game_room.GameStatusOccupied)
	require.NoError(t, err)
}

func TestRoomManager_WaitGameRoomStatus_Deadline(t *testing.T) {
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

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Millisecond))
	gameRoom := &game_room.GameRoom{ID: "transition-test", SchedulerID: "scheduler-test", Status: game_room.GameStatusReady}

	var group errgroup.Group
	waitCalled := make(chan struct{})
	group.Go(func() error {
		waitCalled <- struct{}{}
		defer cancel()

		roomStorage.EXPECT().GetRoom(ctx, gameRoom.SchedulerID, gameRoom.ID).Return(gameRoom, nil)
		return roomManager.WaitRoomStatus(ctx, gameRoom, game_room.GameStatusOccupied)
	})

	<-waitCalled
	require.Eventually(t, func() bool {
		err := group.Wait()
		require.Error(t, err)
		return err != nil
	}, 2*time.Second, time.Second)

	roomStorage.EXPECT().SetRoomStatus(context.Background(), gameRoom.SchedulerID, gameRoom.ID, game_room.GameStatusOccupied).Return(nil)
	err := roomManager.SetRoomStatus(context.Background(), gameRoom, game_room.GameStatusOccupied)
	require.NoError(t, err)
}

// In this test we're going to guarantee that we can be left with a case where
// we do have an dead lock.
func TestRoomManager_WaitGameRoomStatus_PreventDeadlock(t *testing.T) {
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

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Millisecond))
	gameRoom := &game_room.GameRoom{ID: "transition-test", SchedulerID: "scheduler-test", Status: game_room.GameStatusPending}

	// we're going to make multiple set status to populate our buffer channel
	// with messages. from ready -> occupied -> ready
	roomStorage.EXPECT().GetRoom(ctx, gameRoom.SchedulerID, gameRoom.ID).DoAndReturn(func(_ context.Context, _ string, _ string) (*game_room.GameRoom, error) {
		roomStorage.EXPECT().SetRoomStatus(context.Background(), gameRoom.SchedulerID, gameRoom.ID, game_room.GameStatusReady).Return(nil)
		err := roomManager.SetRoomStatus(context.Background(), gameRoom, game_room.GameStatusReady)
		require.NoError(t, err)

		roomStorage.EXPECT().SetRoomStatus(context.Background(), gameRoom.SchedulerID, gameRoom.ID, game_room.GameStatusOccupied).Return(nil)
		err = roomManager.SetRoomStatus(context.Background(), gameRoom, game_room.GameStatusOccupied)
		require.NoError(t, err)

		return gameRoom, nil
	})

	var group errgroup.Group
	waitCalled := make(chan struct{})
	group.Go(func() error {
		waitCalled <- struct{}{}
		defer cancel()
		return roomManager.WaitRoomStatus(ctx, gameRoom, game_room.GameStatusTerminating)
	})

	<-waitCalled
	require.Eventually(t, func() bool {
		err := group.Wait()
		require.Error(t, err)
		return err != nil
	}, 2*time.Second, time.Second)

	roomStorage.EXPECT().SetRoomStatus(context.Background(), gameRoom.SchedulerID, gameRoom.ID, game_room.GameStatusTerminating).Return(nil)
	err := roomManager.SetRoomStatus(context.Background(), gameRoom, game_room.GameStatusTerminating)
	require.NoError(t, err)
}
