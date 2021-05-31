package room_storage

import (
	"context"
	"time"

	"github.com/topfreegames/maestro/internal/entities"
)

// RoomStorage is an interface for retrieving and updating room status and ping information
type RoomStorage interface {
	// GetRoom retrieves a specific room from a scheduler name and roomID
	// returns an error when the room does not exists
	GetRoom(ctx context.Context, scheduler string, roomID string) (*entities.GameRoom, error)
	// CreateRoom creates a room and returns an error if the room already exists
	CreateRoom(ctx context.Context, room *entities.GameRoom) error
	// UpdateRoom updates a room and returns an error if the room does not exists
	UpdateRoom(ctx context.Context, room *entities.GameRoom) error
	// RemoveRoom deletes a room and returns an error if the room does not exists
	RemoveRoom(ctx context.Context, scheduler string, roomID string) error
	// SetRoomStatus sets only the status of a specific room
	SetRoomStatus(ctx context.Context, scheduler string, roomID string, status entities.GameRoomStatus) error

	// GetAllRoomIDs gets all room ids in a scheduler
	GetAllRoomIDs(ctx context.Context, scheduler string) ([]string, error)
	// GetRoomIDsByLastPing gets all room ids in a scheduler where ping is less than threshold
	GetRoomIDsByLastPing(ctx context.Context, scheduler string, threshold time.Time) ([]string, error)
	// GetRoomCount gets the total count of rooms in a scheduler
	GetRoomCount(ctx context.Context, scheduler string) (int, error)
	// GetRoomCountByStatus gets the count of rooms with a specific status in a scheduler
	GetRoomCountByStatus(ctx context.Context, scheduler string, status entities.GameRoomStatus) (int, error)
}
