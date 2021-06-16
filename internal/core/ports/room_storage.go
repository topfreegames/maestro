package ports

import (
	"context"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

// RoomStorage is an interface for retrieving and updating room status and ping information
type RoomStorage interface {
	// GetRoom retrieves a specific room from a scheduler name and roomID
	// returns an error when the room does not exists
	GetRoom(ctx context.Context, scheduler string, roomID string) (*game_room.GameRoom, error)
	// CreateRoom creates a room and returns an error if the room already exists
	CreateRoom(ctx context.Context, room *game_room.GameRoom) error
	// UpdateRoom updates a room and returns an error if the room does not exists
	UpdateRoom(ctx context.Context, room *game_room.GameRoom) error
	// DeleteRoom deletes a room and returns an error if the room does not exists
	DeleteRoom(ctx context.Context, scheduler string, roomID string) error
	// SetRoomStatus sets only the status of a specific room
	SetRoomStatus(ctx context.Context, scheduler string, roomID string, status game_room.GameRoomStatus) error

	// GetAllRoomIDs gets all room ids in a scheduler
	GetAllRoomIDs(ctx context.Context, scheduler string) ([]string, error)
	// GetRoomIDsByLastPing gets all room ids in a scheduler where ping is less than threshold
	GetRoomIDsByLastPing(ctx context.Context, scheduler string, threshold time.Time) ([]string, error)
	// GetRoomCount gets the total count of rooms in a scheduler
	GetRoomCount(ctx context.Context, scheduler string) (int, error)
	// GetRoomCountByStatus gets the count of rooms with a specific status in a scheduler
	GetRoomCountByStatus(ctx context.Context, scheduler string, status game_room.GameRoomStatus) (int, error)

	// WatchRoomStatus watche for status changes on the storage.
	WatchRoomStatus(ctx context.Context, room *game_room.GameRoom) (RoomStorageStatusWatcher, error)
}

// RoomStorageStatusWatcher  defines a process of watcher, it will have a chan
// with the game rooms status changes.
type RoomStorageStatusWatcher interface {
	// ResultChan returns the channel where the changes will be forwarded.
	ResultChan() chan game_room.StatusEvent
	// Stop stops the watcher.
	Stop()
}
