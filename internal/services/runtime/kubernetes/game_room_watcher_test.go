//+build abc

package kubernetes

import (
	"context"
	"testing"
	"time"

	kube "k8s.io/client-go/kubernetes"

	"github.com/orlangure/gnomock"
	"github.com/orlangure/gnomock/preset/k3s"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/entities"
	"github.com/topfreegames/maestro/internal/services/runtime"
)

func TestGameRoomsWatch(t *testing.T) {
	c, err := gnomock.Start(
		k3s.Preset(k3s.WithVersion("v1.16.15")),
	)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, gnomock.Stop(c))
	}()

	kubeconfig, err := k3s.Config(c)
	require.NoError(t, err)

	ctx := context.Background()
	client, err := kube.NewForConfig(kubeconfig)
	require.NoError(t, err)

	kubernetesRuntime := New(client)
	t.Run("watch pod addition", func(t *testing.T) {
		scheduler := &entities.Scheduler{ID: "watch-room-addition"}
		err = kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		watcher, err := kubernetesRuntime.WatchGameRooms(ctx, scheduler)
		defer watcher.Stop()
		require.NoError(t, err)

		gameRoom := &entities.GameRoom{Scheduler: *scheduler}
		gameRoomSpec := entities.GameRoomSpec{
			Containers: []entities.GameRoomContainer{
				{
					Name:  "nginx",
					Image: "nginx:stable-alpine",
				},
			},
		}

		err = kubernetesRuntime.CreateGameRoom(ctx, gameRoom, gameRoomSpec)
		require.NoError(t, err)

		event := <-watcher.ResultChan()
		require.Equal(t, runtime.RuntimeEventAdded, event.Type)
		require.Equal(t, gameRoom.ID, event.GameRoomID)
		require.Equal(t, runtime.RuntimeGameRoomStatusTypePending, event.Status.Type)
	})

	t.Run("watch pod becoming ready", func(t *testing.T) {
		scheduler := &entities.Scheduler{ID: "watch-room-ready"}
		err = kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		watcher, err := kubernetesRuntime.WatchGameRooms(ctx, scheduler)
		defer watcher.Stop()
		require.NoError(t, err)

		gameRoom := &entities.GameRoom{Scheduler: *scheduler}
		gameRoomSpec := entities.GameRoomSpec{
			Containers: []entities.GameRoomContainer{
				{
					Name:  "nginx",
					Image: "nginx:stable-alpine",
				},
			},
		}

		err = kubernetesRuntime.CreateGameRoom(ctx, gameRoom, gameRoomSpec)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			select {
			case event := <-watcher.ResultChan():
				if event.Type == runtime.RuntimeEventUpdated &&
					event.GameRoomID == gameRoom.ID &&
					event.Status.Type == runtime.RuntimeGameRoomStatusTypeReady {
					return true
				}
			default:
			}

			return false
		}, time.Minute, time.Second)
	})

	t.Run("watch pod with error", func(t *testing.T) {
		scheduler := &entities.Scheduler{ID: "watch-room-error"}
		err = kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		watcher, err := kubernetesRuntime.WatchGameRooms(ctx, scheduler)
		defer watcher.Stop()
		require.NoError(t, err)

		gameRoom := &entities.GameRoom{Scheduler: *scheduler}
		gameRoomSpec := entities.GameRoomSpec{
			Containers: []entities.GameRoomContainer{
				{
					Name:    "nginx",
					Image:   "nginx:stable-alpine",
					Command: []string{"some", "inexistend", "command"},
				},
			},
		}

		err = kubernetesRuntime.CreateGameRoom(ctx, gameRoom, gameRoomSpec)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			select {
			case event := <-watcher.ResultChan():
				if event.Type == runtime.RuntimeEventUpdated &&
					event.GameRoomID == gameRoom.ID &&
					event.Status.Type == runtime.RuntimeGameRoomStatusTypeError {
					return true
				}
			default:
			}

			return false
		}, 30*time.Second, time.Second)
	})

	t.Run("watch pod deletion", func(t *testing.T) {
		scheduler := &entities.Scheduler{ID: "watch-room-delete"}
		err = kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		watcher, err := kubernetesRuntime.WatchGameRooms(ctx, scheduler)
		defer watcher.Stop()
		require.NoError(t, err)

		gameRoom := &entities.GameRoom{Scheduler: *scheduler}
		gameRoomSpec := entities.GameRoomSpec{
			Containers: []entities.GameRoomContainer{
				{
					Name:  "nginx",
					Image: "nginx:stable-alpine",
				},
			},
		}

		err = kubernetesRuntime.CreateGameRoom(ctx, gameRoom, gameRoomSpec)
		require.NoError(t, err)

		err = kubernetesRuntime.DeleteGameRoom(ctx, gameRoom)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			select {
			case event := <-watcher.ResultChan():
				if event.Type == runtime.RuntimeEventDeleted &&
					event.GameRoomID == gameRoom.ID {
					return true
				}
			default:
			}

			return false
		}, 30*time.Second, time.Second)
	})
}
