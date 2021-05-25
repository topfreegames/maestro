//+build integration

package kubernetes

import (
	"context"
	"testing"
	"time"

	"github.com/orlangure/gnomock"
	"github.com/orlangure/gnomock/preset/k3s"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/entities"
	"github.com/topfreegames/maestro/internal/services/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube "k8s.io/client-go/kubernetes"
)

func TestGameRoomCreation(t *testing.T) {
	c, err := gnomock.Start(
		k3s.Preset(k3s.WithVersion("v1.16.15")),
	)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, gnomock.Stop(c))
	}()

	kubeconfig, err := k3s.Config(c)
	require.NoError(t, err)

	client, err := kube.NewForConfig(kubeconfig)
	require.NoError(t, err)

	kubernetesRuntime := New(client)
	ctx := context.Background()

	t.Run("successfully create a room", func(t *testing.T) {
		// first, create the scheduler
		scheduler := &entities.Scheduler{ID: "game-room-test"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
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

		pods, err := client.CoreV1().Pods(scheduler.ID).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, pods.Items, 1)
		require.Len(t, pods.Items[0].Spec.Containers, 1)
		require.Equal(t, gameRoomSpec.Containers[0].Name, pods.Items[0].Spec.Containers[0].Name)
		require.Equal(t, gameRoomSpec.Containers[0].Image, pods.Items[0].Spec.Containers[0].Image)
	})

	t.Run("fail with wrong game room spec", func(t *testing.T) {
		// first, create the scheduler
		scheduler := &entities.Scheduler{ID: "game-room-invalid-spec"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		gameRoom := &entities.GameRoom{Scheduler: *scheduler}
		// no containers, meaning it will fail (bacause it can be a pod
		// without containers).
		gameRoomSpec := entities.GameRoomSpec{}
		err = kubernetesRuntime.CreateGameRoom(ctx, gameRoom, gameRoomSpec)
		require.Error(t, err)
		require.ErrorIs(t, err, runtime.ErrInvalidGameRoomSpec)

		pods, err := client.CoreV1().Pods(scheduler.ID).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, pods.Items, 0)
	})
}

func TestGameRoomDeletion(t *testing.T) {
	c, err := gnomock.Start(
		k3s.Preset(k3s.WithVersion("v1.16.15")),
	)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, gnomock.Stop(c))
	}()

	kubeconfig, err := k3s.Config(c)
	require.NoError(t, err)

	client, err := kube.NewForConfig(kubeconfig)
	require.NoError(t, err)

	kubernetesRuntime := New(client)
	ctx := context.Background()

	t.Run("successfully delete a room", func(t *testing.T) {
		// first, create the scheduler
		scheduler := &entities.Scheduler{ID: "game-room-delete-test"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
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

		pods, err := client.CoreV1().Pods(scheduler.ID).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, pods.Items, 1)

		err = kubernetesRuntime.DeleteGameRoom(ctx, gameRoom)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			pods, err := client.CoreV1().Pods(scheduler.ID).List(ctx, metav1.ListOptions{})
			require.NoError(t, err)
			if len(pods.Items) == 0 {
				return true
			}

			return false
		}, 30*time.Second, time.Second)
	})

	t.Run("fail to delete inexistent game room", func(t *testing.T) {
		// first, create the scheduler
		scheduler := &entities.Scheduler{ID: "game-room-inexistent-delete"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		gameRoom := &entities.GameRoom{
			// force a name, so that the delete can run.
			ID:        "game-room-inexistent-room-id",
			Scheduler: *scheduler,
		}

		err = kubernetesRuntime.DeleteGameRoom(ctx, gameRoom)
		require.Error(t, err)
		require.ErrorIs(t, err, runtime.ErrGameRoomNotFound)
	})
}
