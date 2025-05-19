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

//go:build integration
// +build integration

package kubernetes

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGameRoomCreation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	gameRoomName := "some-game-room-name"
	kubernetesRuntime := New(clientset, KubernetesConfig{})

	t.Run("successfully create a room", func(t *testing.T) {
		t.Parallel()
		// first, create the scheduler
		schedulerName := fmt.Sprintf("game-room-test-%s", randomStr(6))
		scheduler := &entities.Scheduler{Name: schedulerName}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)
		t.Cleanup(func() {
			kubernetesRuntime.DeleteScheduler(ctx, scheduler) //nolint:errcheck
		})

		gameRoomSpec := game_room.Spec{
			Containers: []game_room.Container{
				{
					Name:  "nginx",
					Image: "nginx:stable-alpine",
				},
			},
		}
		instance, err := kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler, gameRoomName, gameRoomSpec)
		require.NoError(t, err)

		pods, err := clientset.CoreV1().Pods(scheduler.Name).List(ctx, metav1.ListOptions{})
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
		schedulerName := fmt.Sprintf("game-room-invalid-spec-%s", randomStr(6))
		scheduler := &entities.Scheduler{Name: schedulerName}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)
		t.Cleanup(func() {
			kubernetesRuntime.DeleteScheduler(ctx, scheduler) //nolint:errcheck
		})

		// no containers, meaning it will fail (because it can be a pod
		// without containers).
		gameRoomSpec := game_room.Spec{}
		_, err = kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler, gameRoomName, gameRoomSpec)
		require.Error(t, err)
		require.ErrorIs(t, err, errors.ErrInvalidArgument)

		pods, err := clientset.CoreV1().Pods(scheduler.Name).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, pods.Items, 0)
	})
}

func TestGameRoomDeletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	gameRoomName := "some-game-room"
	kubernetesRuntime := New(clientset, KubernetesConfig{})

	t.Run("successfully delete a room", func(t *testing.T) {
		t.Parallel()
		// first, create the scheduler
		schedulerName := fmt.Sprintf("game-room-delete-test-%s", randomStr(6))
		scheduler := &entities.Scheduler{Name: schedulerName}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)
		t.Cleanup(func() {
			kubernetesRuntime.DeleteScheduler(ctx, scheduler) //nolint:errcheck
		})

		gameRoomSpec := game_room.Spec{
			Containers: []game_room.Container{
				{
					Name:  "nginx",
					Image: "nginx:stable-alpine",
				},
			},
		}
		instance, err := kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler, gameRoomName, gameRoomSpec)
		require.NoError(t, err)

		pods, err := clientset.CoreV1().Pods(scheduler.Name).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, pods.Items, 1)

		err = kubernetesRuntime.DeleteGameRoomInstance(ctx, instance, "reason")
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			pods, err := clientset.CoreV1().Pods(scheduler.Name).List(ctx, metav1.ListOptions{})
			require.NoError(t, err)
			return len(pods.Items) == 0
		}, 30*time.Second, time.Second)
	})

	t.Run("fail to delete inexistent game room", func(t *testing.T) {
		t.Parallel()
		// first, create the scheduler
		schedulerName := fmt.Sprintf("game-room-inexistent-delete-%s", randomStr(6))
		scheduler := &entities.Scheduler{Name: schedulerName}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)
		t.Cleanup(func() {
			kubernetesRuntime.DeleteScheduler(ctx, scheduler) //nolint:errcheck
		})

		gameRoomInstance := &game_room.Instance{
			// force a name, so that to delete can run.
			ID:          "game-room-inexistent-room-id",
			SchedulerID: scheduler.Name,
		}

		err = kubernetesRuntime.DeleteGameRoomInstance(ctx, gameRoomInstance, "reason")
		require.Error(t, err)
		require.ErrorIs(t, err, errors.ErrNotFound)
	})
}

func TestCreateGameRoomName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kubernetesRuntime := New(clientset, KubernetesConfig{})

	t.Run("When scheduler name is greater than max name length minus randomLength", func(t *testing.T) {
		t.Run("return the name with randomLength", func(t *testing.T) {
			name, err := kubernetesRuntime.CreateGameRoomName(
				ctx,
				entities.Scheduler{
					Name: "lllllllllllllllllllllllllllllllllllllllllllllllllllllllllll",
				},
			)
			assert.NoError(t, err)
			assert.Len(t, name, 64)
		})
	})
	t.Run("To any other scheduler name size lower than max name length minus randomLength", func(t *testing.T) {
		t.Run("return the name plus 5 caracters random", func(t *testing.T) {
			schedulerName := "lllllllllllllllllllllllllllllll"
			name, err := kubernetesRuntime.CreateGameRoomName(
				ctx,
				entities.Scheduler{
					Name: schedulerName,
				},
			)
			assert.NoError(t, err)
			assert.Len(t, name, len(schedulerName)+6)
		})
	})
}
