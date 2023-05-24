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

package ports

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

// Runtime defines an interface implemented by the services that manages
// containers (game rooms).
type Runtime interface {
	// CreateScheduler Creates a scheduler on the runtime.
	CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) error
	// DeleteScheduler Deletes a scheduler on the runtime.
	DeleteScheduler(ctx context.Context, scheduler *entities.Scheduler) error
	// CreateGameRoomInstance Creates a game room instance on the runtime using
	// the specification provided.
	CreateGameRoomInstance(ctx context.Context, scheduler *entities.Scheduler, gameRoomName string, spec game_room.Spec) (*game_room.Instance, error)
	// DeleteGameRoomInstance Deletes a game room instance on the runtime.
	DeleteGameRoomInstance(ctx context.Context, gameRoomInstance *game_room.Instance) error
	// WatchGameRoomInstances Watches for changes of a scheduler game room instances.
	WatchGameRoomInstances(ctx context.Context, scheduler *entities.Scheduler) (RuntimeWatcher, error)
	// Create a name to the room.
	CreateGameRoomName(ctx context.Context, scheduler entities.Scheduler) (string, error)
}

// RuntimeWatcher defines a process of watcher, it will have a chan with the
// changes on the Runtime, and also a way to stop watching.
type RuntimeWatcher interface {
	// ResultChan returns the channel where the changes will be forwarded.
	ResultChan() chan game_room.InstanceEvent
	// Stop stops the watcher.
	Stop()
	// Err returns the error of the watcher. It is possible that the watcher
	// doesn't have any error (returning nil).
	Err() error
}
