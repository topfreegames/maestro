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

//+build integration

package kubernetes

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGameRoomCreation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := getKubernetesClientset(t)
	kubernetesRuntime := New(client)

	t.Run("successfully create a room", func(t *testing.T) {
		t.Parallel()
		// first, create the scheduler
		scheduler := &entities.Scheduler{Name: "game-room-test"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		gameRoomSpec := game_room.Spec{
			Containers: []game_room.Container{
				{
					Name:  "nginx",
					Image: "nginx:stable-alpine",
				},
			},
		}
		instance, err := kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler.Name, gameRoomSpec)
		require.NoError(t, err)

		pods, err := client.CoreV1().Pods(scheduler.Name).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, pods.Items, 1)
		require.Len(t, pods.Items[0].Spec.Containers, 1)
		require.Equal(t, gameRoomSpec.Containers[0].Name, pods.Items[0].Spec.Containers[0].Name)
		require.Equal(t, gameRoomSpec.Containers[0].Image, pods.Items[0].Spec.Containers[0].Image)
		require.Equal(t, instance.ID, pods.Items[0].ObjectMeta.Name)
	})

	t.Run("fail with wrong game room spec", func(t *testing.T) {
		t.Parallel()
		// first, create the scheduler
		scheduler := &entities.Scheduler{Name: "game-room-invalid-spec"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		// no containers, meaning it will fail (bacause it can be a pod
		// without containers).
		gameRoomSpec := game_room.Spec{}
		_, err = kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler.Name, gameRoomSpec)
		require.Error(t, err)
		require.ErrorIs(t, err, errors.ErrInvalidArgument)

		pods, err := client.CoreV1().Pods(scheduler.Name).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, pods.Items, 0)
	})
}

func TestGameRoomDeletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := getKubernetesClientset(t)
	kubernetesRuntime := New(client)

	t.Run("successfully delete a room", func(t *testing.T) {
		t.Parallel()
		// first, create the scheduler
		scheduler := &entities.Scheduler{Name: "game-room-delete-test"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		gameRoomSpec := game_room.Spec{
			Containers: []game_room.Container{
				{
					Name:  "nginx",
					Image: "nginx:stable-alpine",
				},
			},
		}
		instance, err := kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler.Name, gameRoomSpec)
		require.NoError(t, err)

		pods, err := client.CoreV1().Pods(scheduler.Name).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, pods.Items, 1)

		err = kubernetesRuntime.DeleteGameRoomInstance(ctx, instance)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			pods, err := client.CoreV1().Pods(scheduler.Name).List(ctx, metav1.ListOptions{})
			require.NoError(t, err)
			return len(pods.Items) == 0
		}, 30*time.Second, time.Second)
	})

	t.Run("fail to delete inexistent game room", func(t *testing.T) {
		t.Parallel()
		// first, create the scheduler
		scheduler := &entities.Scheduler{Name: "game-room-inexistent-delete"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		gameRoomInstance := &game_room.Instance{
			// force a name, so that the delete can run.
			ID:          "game-room-inexistent-room-id",
			SchedulerID: scheduler.Name,
		}

		err = kubernetesRuntime.DeleteGameRoomInstance(ctx, gameRoomInstance)
		require.Error(t, err)
		require.ErrorIs(t, err, errors.ErrNotFound)
	})
}
