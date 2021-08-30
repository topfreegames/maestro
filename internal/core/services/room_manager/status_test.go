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

func TestRoomManager_ValidateRoomStatusTransition_SuccessTransitions(t *testing.T) {
	for fromStatus, transitions := range validStatusTransitions {
		for transition := range transitions {
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
				err := roomManager.validateRoomStatusTransition(fromStatus, transition)
				require.NoError(t, err)
			})
		}
	}
}

func TestRoomManager_ValidateRoomStatusTransition_InvalidTransition(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	roomManager := NewRoomManager(
		clockmock.NewFakeClock(time.Now()),
		pamock.NewMockPortAllocator(mockCtrl),
		rsmock.NewMockRoomStorage(mockCtrl),
		ismock.NewMockGameRoomInstanceStorage(mockCtrl),
		runtimemock.NewMockRuntime(mockCtrl),
	)

	err := roomManager.validateRoomStatusTransition(game_room.GameStatusTerminating, game_room.GameStatusReady)
	require.Error(t, err)
}

func TestRoomManager_WaitGameRoomStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	watcher := rsmock.NewMockRoomStorageStatusWatcher(mockCtrl)
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
	eventsChan := make(chan game_room.StatusEvent)
	group.Go(func() error {
		waitCalled <- struct{}{}

		roomStorage.EXPECT().GetRoom(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(gameRoom, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gameRoom).Return(watcher, nil)
		watcher.EXPECT().ResultChan().Return(eventsChan)
		watcher.EXPECT().Stop()

		return roomManager.WaitRoomStatus(context.Background(), gameRoom, transition)
	})

	<-waitCalled
	eventsChan <- game_room.StatusEvent{RoomID: gameRoom.ID, SchedulerName: gameRoom.SchedulerID, Status: transition}

	require.Eventually(t, func() bool {
		err := group.Wait()
		require.NoError(t, err)
		return err == nil
	}, 2*time.Second, time.Second)
}

func TestRoomManager_WaitGameRoomStatus_Deadline(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	watcher := rsmock.NewMockRoomStorageStatusWatcher(mockCtrl)
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
	eventsChan := make(chan game_room.StatusEvent)
	group.Go(func() error {
		waitCalled <- struct{}{}
		defer cancel()

		roomStorage.EXPECT().GetRoom(ctx, gameRoom.SchedulerID, gameRoom.ID).Return(gameRoom, nil)
		roomStorage.EXPECT().WatchRoomStatus(gomock.Any(), gameRoom).Return(watcher, nil)
		watcher.EXPECT().ResultChan().Return(eventsChan)
		watcher.EXPECT().Stop()

		return roomManager.WaitRoomStatus(ctx, gameRoom, game_room.GameStatusOccupied)
	})

	<-waitCalled
	require.Eventually(t, func() bool {
		err := group.Wait()
		require.Error(t, err)
		return err != nil
	}, 2*time.Second, time.Second)
}
