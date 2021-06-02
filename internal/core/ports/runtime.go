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
	// CreateScheduler Deletes a scheduler on the runtime.
	DeleteScheduler(ctx context.Context, scheduler *entities.Scheduler) error
	// CreateGameRoomInstance Creates a game room instance on the runtime using
	// the specification provided.
	CreateGameRoomInstance(ctx context.Context, schedulerId string, spec game_room.Spec) (*game_room.Instance, error)
	// DeleteGameRoomInstance Deletes a game room instance on the runtime.
	DeleteGameRoomInstance(ctx context.Context, gameRoomInstance *game_room.Instance) error
	// WathGameRooms Watches for changes of a scheduler game room instances.
	WatchGameRoomInstances(ctx context.Context, scheduler *entities.Scheduler) (RuntimeWatcher, error)
}

// RuntimeWatcher defines a process of watcher, it will have a chan with the
// changes on the Runtime, and also a way to stop watching.
type RuntimeWatcher interface {
	// ResultChan returns the channel where the changes will be forwarded.
	ResultChan() chan game_room.InstanceEvent
	// Stop stops the watcher.
	Stop()
}
