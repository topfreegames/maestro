// +build unit

package runtime_watcher_worker

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	clockmock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	instancemock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	pamock "github.com/topfreegames/maestro/internal/adapters/port_allocator/mock"
	roomstoragemock "github.com/topfreegames/maestro/internal/adapters/room_storage/mock"
	runtimemock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
	"github.com/topfreegames/maestro/internal/core/workers"
)

func TestProcessRuntimeEvents(t *testing.T) {
	workerOptions := func(t *testing.T) (*gomock.Controller, *instancemock.MockGameRoomInstanceStorage, *runtimemock.MockRuntime, *workers.WorkerOptions) {
		mockCtrl := gomock.NewController(t)

		now := time.Now()
		portAllocator := pamock.NewMockPortAllocator(mockCtrl)
		roomStorage := roomstoragemock.NewMockRoomStorage(mockCtrl)
		runtime := runtimemock.NewMockRuntime(mockCtrl)
		instanceStorage := instancemock.NewMockGameRoomInstanceStorage(mockCtrl)
		fakeClock := clockmock.NewFakeClock(now)
		config := room_manager.RoomManagerConfig{RoomInitializationTimeoutMillis: time.Millisecond * 1000}
		roomManager := room_manager.NewRoomManager(fakeClock, portAllocator, roomStorage, instanceStorage, runtime, config)

		return mockCtrl, instanceStorage, runtime, &workers.WorkerOptions{
			Runtime:     runtime,
			RoomManager: roomManager,
		}
	}

	t.Run("fails to start watcher", func(t *testing.T) {
		mockCtrl, _, runtime, workerOptions := workerOptions(t)
		defer mockCtrl.Finish()

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

			return true
		}, time.Second, time.Millisecond)
	})

	t.Run("updates instance on added event", func(t *testing.T) {
		mockCtrl, instanceStorage, runtime, workerOptions := workerOptions(t)
		defer mockCtrl.Finish()

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
			return nil
		})

		watcherDone := make(chan error)
		go func() {
			err := watcher.Start(context.Background())
			watcherDone <- err
		}()

		require.Eventually(t, func() bool {
			resultChan <- game_room.InstanceEvent{
				Type:     game_room.InstanceEventTypeAdded,
				Instance: newInstance,
			}

			return true
		}, time.Second, time.Millisecond)

		// wait until the watcher process the event
		require.Eventually(t, func() bool {
			return updateCalled
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
