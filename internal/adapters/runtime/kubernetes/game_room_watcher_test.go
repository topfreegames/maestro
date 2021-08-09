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

	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
)

func TestGameRoomsWatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := getKubernetesClientset(t)
	kubernetesRuntime := New(client)

	t.Run("watch pod addition", func(t *testing.T) {
		t.Parallel()
		scheduler := &entities.Scheduler{Name: "watch-room-addition"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
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

		instance, err := kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler.Name, gameRoomSpec)
		require.NoError(t, err)

		event := <-watcher.ResultChan()
		require.Equal(t, game_room.InstanceEventTypeAdded, event.Type)
		require.Equal(t, instance.ID, event.Instance.ID)
		require.Equal(t, game_room.InstancePending, event.Instance.Status.Type)
	})

	t.Run("watch pod becoming ready", func(t *testing.T) {
		t.Parallel()
		scheduler := &entities.Scheduler{Name: "watch-room-ready"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
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

		instance, err := kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler.Name, gameRoomSpec)
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
		t.Parallel()
		scheduler := &entities.Scheduler{Name: "watch-room-error"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
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

		instance, err := kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler.Name, gameRoomSpec)
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
		}, time.Minute, time.Second)
	})

	t.Run("watch pod deletion", func(t *testing.T) {
		t.Parallel()
		scheduler := &entities.Scheduler{Name: "watch-room-delete"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
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

		instance, err := kubernetesRuntime.CreateGameRoomInstance(ctx, scheduler.Name, gameRoomSpec)
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
		}, time.Minute, time.Second)
	})
}
