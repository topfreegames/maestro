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

package runtimewatcher

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/core/worker"
)

func workerOptions(t *testing.T) (*gomock.Controller, *mockports.MockRuntime, *mockports.MockRoomManager, *worker.WorkerOptions) {
	mockCtrl := gomock.NewController(t)

	runtime := mockports.NewMockRuntime(mockCtrl)
	roomManager := mockports.NewMockRoomManager(mockCtrl)

	return mockCtrl, runtime, roomManager, &worker.WorkerOptions{
		Runtime:     runtime,
		RoomManager: roomManager,
	}
}

func TestRuntimeWatcher_Start(t *testing.T) {
	events := []game_room.InstanceEventType{
		game_room.InstanceEventTypeAdded,
		game_room.InstanceEventTypeUpdated,
	}

	for _, event := range events {
		t.Run(fmt.Sprintf("when %s happens, updates instance", event.String()), func(t *testing.T) {
			mockCtrl, runtime, roomManager, workerOptions := workerOptions(t)

			scheduler := &entities.Scheduler{Name: "test"}
			watcher := NewRuntimeWatcherWorker(scheduler, workerOptions)

			runtimeWatcher := mockports.NewMockRuntimeWatcher(mockCtrl)
			runtime.EXPECT().WatchGameRoomInstances(gomock.Any(), scheduler).Return(runtimeWatcher, nil)

			resultChan := make(chan game_room.InstanceEvent)
			runtimeWatcher.EXPECT().ResultChan().Return(resultChan)
			runtimeWatcher.EXPECT().Stop().MinTimes(0)

			// instance updates
			newInstance := &game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceReady}}
			roomManager.EXPECT().UpdateRoomInstance(gomock.Any(), gomock.Any()).Return(nil).MinTimes(0)

			watcherDone := make(chan error)
			go func() {
				err := watcher.Start(context.Background())
				watcherDone <- err
			}()

			resultChan <- game_room.InstanceEvent{
				Type:     event,
				Instance: newInstance,
			}

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

		t.Run(fmt.Sprintf("when %s happens, and update instance fails, does nothing", event.String()), func(t *testing.T) {
			mockCtrl, runtime, roomManager, workerOptions := workerOptions(t)

			scheduler := &entities.Scheduler{Name: "test"}
			watcher := NewRuntimeWatcherWorker(scheduler, workerOptions)

			runtimeWatcher := mockports.NewMockRuntimeWatcher(mockCtrl)
			runtime.EXPECT().WatchGameRoomInstances(gomock.Any(), scheduler).Return(runtimeWatcher, nil)

			resultChan := make(chan game_room.InstanceEvent)
			runtimeWatcher.EXPECT().ResultChan().Return(resultChan)
			runtimeWatcher.EXPECT().Stop().MinTimes(0)

			// instance updates
			newInstance := &game_room.Instance{Status: game_room.InstanceStatus{Type: game_room.InstanceReady}}
			roomManager.EXPECT().UpdateRoomInstance(gomock.Any(), gomock.Any()).Return(errors.New("error"))

			watcherDone := make(chan error)
			go func() {
				err := watcher.Start(context.Background())
				watcherDone <- err
			}()

			resultChan <- game_room.InstanceEvent{
				Type:     event,
				Instance: newInstance,
			}

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

	t.Run("fails to start watcher", func(t *testing.T) {
		_, runtime, _, workerOptions := workerOptions(t)

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

	t.Run("clean room state on delete event", func(t *testing.T) {
		mockCtrl, runtime, roomManager, workerOptions := workerOptions(t)

		scheduler := &entities.Scheduler{Name: "test"}
		watcher := NewRuntimeWatcherWorker(scheduler, workerOptions)

		runtimeWatcher := mockports.NewMockRuntimeWatcher(mockCtrl)
		runtime.EXPECT().WatchGameRoomInstances(gomock.Any(), scheduler).Return(runtimeWatcher, nil)
		roomManager.EXPECT().CleanRoomState(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		resultChan := make(chan game_room.InstanceEvent)
		runtimeWatcher.EXPECT().ResultChan().Return(resultChan)
		runtimeWatcher.EXPECT().Stop()

		// instance updates
		instance := &game_room.Instance{ID: "room-id", SchedulerID: "room-scheduler"}

		watcherDone := make(chan error)
		go func() {
			err := watcher.Start(context.Background())
			watcherDone <- err
		}()

		resultChan <- game_room.InstanceEvent{
			Type:     game_room.InstanceEventTypeDeleted,
			Instance: instance,
		}

		// stop the watcher
		watcher.Stop(context.Background())

		require.Eventually(t, func() bool {
			err := <-watcherDone
			require.NoError(t, err)

			return true
		}, time.Second, time.Millisecond)
	})

	t.Run("when clean room state fails, does nothing", func(t *testing.T) {
		mockCtrl, runtime, roomManager, workerOptions := workerOptions(t)

		scheduler := &entities.Scheduler{Name: "test"}
		watcher := NewRuntimeWatcherWorker(scheduler, workerOptions)

		runtimeWatcher := mockports.NewMockRuntimeWatcher(mockCtrl)
		runtime.EXPECT().WatchGameRoomInstances(gomock.Any(), scheduler).Return(runtimeWatcher, nil)
		roomManager.EXPECT().CleanRoomState(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("error"))

		resultChan := make(chan game_room.InstanceEvent)
		runtimeWatcher.EXPECT().ResultChan().Return(resultChan)
		runtimeWatcher.EXPECT().Stop()

		// instance updates
		instance := &game_room.Instance{ID: "room-id", SchedulerID: "room-scheduler"}

		watcherDone := make(chan error)
		go func() {
			err := watcher.Start(context.Background())
			watcherDone <- err
		}()

		resultChan <- game_room.InstanceEvent{
			Type:     game_room.InstanceEventTypeDeleted,
			Instance: instance,
		}

		// stop the watcher
		watcher.Stop(context.Background())

		require.Eventually(t, func() bool {
			err := <-watcherDone
			require.NoError(t, err)

			return true
		}, time.Second, time.Millisecond)
	})

	t.Run("when resultChan is closed, worker stops without error", func(t *testing.T) {
		mockCtrl, runtime, _, workerOptions := workerOptions(t)

		scheduler := &entities.Scheduler{Name: "test"}
		watcher := NewRuntimeWatcherWorker(scheduler, workerOptions)

		runtimeWatcher := mockports.NewMockRuntimeWatcher(mockCtrl)
		runtime.EXPECT().WatchGameRoomInstances(gomock.Any(), scheduler).Return(runtimeWatcher, nil)
		resultChan := make(chan game_room.InstanceEvent)
		runtimeWatcher.EXPECT().ResultChan().Return(resultChan)
		runtimeWatcher.EXPECT().Stop()

		ctx, cancelFunc := context.WithCancel(context.Background())

		go func() {
			time.Sleep(time.Millisecond * 100)
			close(resultChan)
			cancelFunc()
		}()

		err := watcher.Start(ctx)
		require.NoError(t, err)
	})
}
