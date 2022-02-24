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

package runtime_watcher_worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	mockeventsservice "github.com/topfreegames/maestro/internal/core/services/interfaces/mock/events_service"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	clockmock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	instancemock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	pamock "github.com/topfreegames/maestro/internal/adapters/port_allocator/mock"
	runtimemock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
)

func workerOptions(t *testing.T) (*gomock.Controller, *instancemock.MockGameRoomInstanceStorage, *mockports.MockRoomStorage, *runtimemock.MockRuntime, *workers.WorkerOptions) {
	mockCtrl := gomock.NewController(t)

	now := time.Now()
	portAllocator := pamock.NewMockPortAllocator(mockCtrl)
	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	instanceStorage := instancemock.NewMockGameRoomInstanceStorage(mockCtrl)
	fakeClock := clockmock.NewFakeClock(now)
	config := room_manager.RoomManagerConfig{RoomInitializationTimeout: time.Millisecond * 1000}
	eventsForwarderService := mockeventsservice.NewMockEventsService(mockCtrl)
	roomManager := room_manager.New(fakeClock, portAllocator, roomStorage, instanceStorage, runtime, eventsForwarderService, config)

	return mockCtrl, instanceStorage, roomStorage, runtime, &workers.WorkerOptions{
		Runtime:     runtime,
		RoomManager: roomManager,
	}
}

func TestRuntimeWatcher_Start(t *testing.T) {
	t.Run("fails to start watcher", func(t *testing.T) {
		_, _, _, runtime, workerOptions := workerOptions(t)

		scheduler := &entities.Scheduler{Name: "test"}
		watcher := NewRuntimeWatcherWorker(scheduler, workerOptions)

		runtime.EXPECT().WatchGameRoomInstances(gomock.Any(), scheduler).Return(nil, porterrors.ErrUnexpected)

		watcherDone := make(chan error)
		go func() {
			err := watcher.Start(context.Background())
			watcherDone <- err
		}()

		require.Eventually(t, func() bool {
			err := <-watcherDone
			require.Error(t, err)
			require.False(t, watcher.IsRunning())

			return true
		}, time.Second, time.Millisecond)
	})
}

func TestRuntimeWatcher_UpdateInstance(t *testing.T) {
	events := []game_room.InstanceEventType{
		game_room.InstanceEventTypeAdded,
		game_room.InstanceEventTypeUpdated,
	}

	for _, event := range events {
		t.Run(fmt.Sprintf("when %s happens, updates instance", event.String()), func(t *testing.T) {
			mockCtrl, instanceStorage, roomStorage, runtime, workerOptions := workerOptions(t)

			scheduler := &entities.Scheduler{Name: "test"}
			watcher := NewRuntimeWatcherWorker(scheduler, workerOptions)

			runtimeWatcher := runtimemock.NewMockRuntimeWatcher(mockCtrl)
			runtime.EXPECT().WatchGameRoomInstances(gomock.Any(), scheduler).Return(runtimeWatcher, nil)

			resultChan := make(chan game_room.InstanceEvent)
			runtimeWatcher.EXPECT().ResultChan().Return(resultChan)
			runtimeWatcher.EXPECT().Stop()

			// instance updates
			newInstance := &game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceReady}}
			gameRoom := &game_room.GameRoom{Status: game_room.GameStatusPending, PingStatus: game_room.GameRoomPingStatusReady}

			updateCalled := false
			instanceStorage.EXPECT().UpsertInstance(gomock.Any(), newInstance).DoAndReturn(func(_ context.Context, _ *game_room.Instance) error {
				updateCalled = true
				return nil
			})
			roomStorage.EXPECT().GetRoom(gomock.Any(), gomock.Any(), gomock.Any()).Return(gameRoom, nil)
			instanceStorage.EXPECT().GetInstance(gomock.Any(), gomock.Any(), gomock.Any()).Return(newInstance, nil)
			roomStorage.EXPECT().UpdateRoomStatus(gomock.Any(), gomock.Any(), gomock.Any(), game_room.GameStatusReady).Return(nil)

			watcherDone := make(chan error)
			go func() {
				err := watcher.Start(context.Background())
				watcherDone <- err
			}()

			require.Eventually(t, func() bool {
				resultChan <- game_room.InstanceEvent{
					Type:     event,
					Instance: newInstance,
				}

				return true
			}, time.Second, time.Millisecond)

			// wait until the watcher process the event
			require.Eventually(t, func() bool {
				return updateCalled
			}, time.Second, 100*time.Millisecond)

			// stop the watcher
			require.True(t, watcher.IsRunning())
			watcher.Stop(context.Background())

			require.Eventually(t, func() bool {
				err := <-watcherDone
				require.NoError(t, err)
				require.False(t, watcher.IsRunning())

				return true
			}, time.Second, time.Millisecond)
		})

		t.Run(fmt.Sprintf("when %s happens, and update instance fails, does nothgin", event.String()), func(t *testing.T) {
			mockCtrl, instanceStorage, _, runtime, workerOptions := workerOptions(t)

			scheduler := &entities.Scheduler{Name: "test"}
			watcher := NewRuntimeWatcherWorker(scheduler, workerOptions)

			runtimeWatcher := runtimemock.NewMockRuntimeWatcher(mockCtrl)
			runtime.EXPECT().WatchGameRoomInstances(gomock.Any(), scheduler).Return(runtimeWatcher, nil)

			resultChan := make(chan game_room.InstanceEvent)
			runtimeWatcher.EXPECT().ResultChan().Return(resultChan)
			runtimeWatcher.EXPECT().Stop()

			// instance updates
			newInstance := &game_room.Instance{}

			updateCalled := false
			instanceStorage.EXPECT().UpsertInstance(gomock.Any(), newInstance).DoAndReturn(func(_ context.Context, _ *game_room.Instance) error {
				updateCalled = true
				return porterrors.ErrUnexpected
			})

			watcherDone := make(chan error)
			go func() {
				err := watcher.Start(context.Background())
				watcherDone <- err
			}()

			require.Eventually(t, func() bool {
				resultChan <- game_room.InstanceEvent{
					Type:     event,
					Instance: newInstance,
				}

				return true
			}, time.Second, time.Millisecond)

			// wait until the watcher process the event
			require.Eventually(t, func() bool {
				return updateCalled
			}, time.Second, 100*time.Millisecond)

			// stop the watcher
			require.True(t, watcher.IsRunning())
			watcher.Stop(context.Background())

			require.Eventually(t, func() bool {
				err := <-watcherDone
				require.NoError(t, err)
				require.False(t, watcher.IsRunning())

				return true
			}, time.Second, time.Millisecond)
		})
	}
}

func TestRuntimeWatcher_CleanRoomState(t *testing.T) {
	t.Run("clean room state on delete event", func(t *testing.T) {
		mockCtrl, instanceStorage, roomStorage, runtime, workerOptions := workerOptions(t)

		scheduler := &entities.Scheduler{Name: "test"}
		watcher := NewRuntimeWatcherWorker(scheduler, workerOptions)

		runtimeWatcher := runtimemock.NewMockRuntimeWatcher(mockCtrl)
		runtime.EXPECT().WatchGameRoomInstances(gomock.Any(), scheduler).Return(runtimeWatcher, nil)

		resultChan := make(chan game_room.InstanceEvent)
		runtimeWatcher.EXPECT().ResultChan().Return(resultChan)
		runtimeWatcher.EXPECT().Stop()

		// instance updates
		instance := &game_room.Instance{ID: "room-id", SchedulerID: "room-scheduler"}

		removeRoom := false
		removeInstance := false
		roomStorage.EXPECT().DeleteRoom(gomock.Any(), instance.SchedulerID, instance.ID).DoAndReturn(func(_ context.Context, _ string, _ string) error {
			removeRoom = true
			return nil
		})
		instanceStorage.EXPECT().DeleteInstance(gomock.Any(), instance.SchedulerID, instance.ID).DoAndReturn(func(_ context.Context, _ string, _ string) error {
			removeInstance = true
			return nil
		})

		watcherDone := make(chan error)
		go func() {
			err := watcher.Start(context.Background())
			watcherDone <- err
		}()

		require.Eventually(t, func() bool {
			resultChan <- game_room.InstanceEvent{
				Type:     game_room.InstanceEventTypeDeleted,
				Instance: instance,
			}

			return true
		}, time.Second, time.Millisecond)

		// wait until the watcher process the event
		require.Eventually(t, func() bool {
			return removeRoom && removeInstance
		}, time.Second, 100*time.Millisecond)

		// stop the watcher
		watcher.Stop(context.Background())

		require.Eventually(t, func() bool {
			err := <-watcherDone
			require.NoError(t, err)

			return true
		}, time.Second, time.Millisecond)
	})

	t.Run("when clean room state fails, does nothing", func(t *testing.T) {
		mockCtrl, _, roomStorage, runtime, workerOptions := workerOptions(t)

		scheduler := &entities.Scheduler{Name: "test"}
		watcher := NewRuntimeWatcherWorker(scheduler, workerOptions)

		runtimeWatcher := runtimemock.NewMockRuntimeWatcher(mockCtrl)
		runtime.EXPECT().WatchGameRoomInstances(gomock.Any(), scheduler).Return(runtimeWatcher, nil)

		resultChan := make(chan game_room.InstanceEvent)
		runtimeWatcher.EXPECT().ResultChan().Return(resultChan)
		runtimeWatcher.EXPECT().Stop()

		// instance updates
		instance := &game_room.Instance{ID: "room-id", SchedulerID: "room-scheduler"}

		removeRoom := false
		roomStorage.EXPECT().DeleteRoom(gomock.Any(), instance.SchedulerID, instance.ID).DoAndReturn(func(_ context.Context, _ string, _ string) error {
			removeRoom = true
			return porterrors.ErrUnexpected
		})

		watcherDone := make(chan error)
		go func() {
			err := watcher.Start(context.Background())
			watcherDone <- err
		}()

		require.Eventually(t, func() bool {
			resultChan <- game_room.InstanceEvent{
				Type:     game_room.InstanceEventTypeDeleted,
				Instance: instance,
			}

			return true
		}, time.Second, time.Millisecond)

		// wait until the watcher process the event
		require.Eventually(t, func() bool {
			return removeRoom
		}, time.Second, 100*time.Millisecond)

		// stop the watcher
		watcher.Stop(context.Background())

		require.Eventually(t, func() bool {
			err := <-watcherDone
			require.NoError(t, err)

			return true
		}, time.Second, time.Millisecond)
	})
}
