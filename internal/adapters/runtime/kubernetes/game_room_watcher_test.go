//+build integration

package kubernetes

import (
	"context"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	kube "k8s.io/client-go/kubernetes"

	"github.com/orlangure/gnomock"
	"github.com/orlangure/gnomock/preset/k3s"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
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

		watcher, err := kubernetesRuntime.WatchGameRoomInstances(ctx, scheduler)
		defer watcher.Stop()
		require.NoError(t, err)

		gameRoomSpec := game_room.Spec{
			Containers: []game_room.Container{
				{
					Name:  "nginx",
					Image: "nginx:stable-alpine",
				},
			},
		}

		instance, err := kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler.ID, gameRoomSpec)
		require.NoError(t, err)

		event := <-watcher.ResultChan()
		require.Equal(t, game_room.InstanceEventTypeAdded, event.Type)
		require.Equal(t, instance.ID, event.Instance.ID)
		require.Equal(t, game_room.InstancePending, event.Instance.Status.Type)
	})

	t.Run("watch pod becoming ready", func(t *testing.T) {
		scheduler := &entities.Scheduler{ID: "watch-room-ready"}
		err = kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		watcher, err := kubernetesRuntime.WatchGameRoomInstances(ctx, scheduler)
		defer watcher.Stop()
		require.NoError(t, err)

		gameRoomSpec := game_room.Spec{
			Containers: []game_room.Container{
				{
					Name:  "nginx",
					Image: "nginx:stable-alpine",
				},
			},
		}

		instance, err := kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler.ID, gameRoomSpec)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			select {
			case event := <-watcher.ResultChan():
				if event.Type == game_room.InstanceEventTypeUpdated &&
					event.Instance.ID == instance.ID &&
					event.Instance.Status.Type == game_room.InstanceReady {
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

		watcher, err := kubernetesRuntime.WatchGameRoomInstances(ctx, scheduler)
		defer watcher.Stop()
		require.NoError(t, err)

		gameRoomSpec := game_room.Spec{
			Containers: []game_room.Container{
				{
					Name:    "nginx",
					Image:   "nginx:stable-alpine",
					Command: []string{"some", "inexistend", "command"},
				},
			},
		}

		instance, err := kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler.ID, gameRoomSpec)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			select {
			case event := <-watcher.ResultChan():
				if event.Type == game_room.InstanceEventTypeUpdated &&
					event.Instance.ID == instance.ID &&
					event.Instance.Status.Type == game_room.InstanceError {
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

		watcher, err := kubernetesRuntime.WatchGameRoomInstances(ctx, scheduler)
		defer watcher.Stop()
		require.NoError(t, err)

		gameRoomSpec := game_room.Spec{
			Containers: []game_room.Container{
				{
					Name:  "nginx",
					Image: "nginx:stable-alpine",
				},
			},
		}

		instance, err := kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler.ID, gameRoomSpec)
		require.NoError(t, err)

		err = kubernetesRuntime.DeleteGameRoomInstance(ctx, instance)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			select {
			case event := <-watcher.ResultChan():
				if event.Type == game_room.InstanceEventTypeDeleted &&
					event.Instance.ID == instance.ID {
					return true
				}
			default:
			}

			return false
		}, 30*time.Second, time.Second)
	})
}
