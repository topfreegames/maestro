package runtime

import (
	"context"

	"github.com/topfreegames/maestro/internal/entities"
)

// Runtime defines an interface implemented by the services that manages
// containers (game rooms).
type Runtime interface {
	// CreateScheduler Creates a scheduler on the runtime.
	CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) error
	// CreateScheduler Deletes a scheduler on the runtime.
	DeleteScheduler(ctx context.Context, scheduler *entities.Scheduler) error
	// CreateGameRoom Creates a game room on the runtime using the specification
	// inside the GameRoom.
	CreateGameRoom(ctx context.Context, gameRoom *entities.GameRoom, spec entities.GameRoomSpec) error
	// DeleteGameRoom Deletes a game room on the runtime.
	DeleteGameRoom(ctx context.Context, gameRoom *entities.GameRoom) error
	// WathGameRooms Watches for changes of a scheduler game rooms.
	WatchGameRooms(ctx context.Context, scheduler *entities.Scheduler) (RuntimeWatcher, error)
}

// RuntimeWatcher defines a process of watcher, it will have a chan with the
// changes on the Runtime, and also a way to stop watching.
type RuntimeWatcher interface {
	// ResultChan returns the channel where the changes will be forwarded.
	ResultChan() chan RuntimeEvent
	// Stop stops the watcher.
	Stop()
}
