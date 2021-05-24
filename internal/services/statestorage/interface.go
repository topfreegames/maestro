package statestorage

import (
	"context"
	"github.com/topfreegames/maestro/internal/entities"
	"time"
)


type StateStorage interface {
	GetRoom(ctx context.Context, scheduler string, roomID string) (*entities.GameRoom, error)
	CreateRoom(ctx context.Context, room *entities.GameRoom) error
	UpdateRoom(ctx context.Context, room *entities.GameRoom) error
	RemoveRoom(ctx context.Context, scheduler string, roomID string) error
	SetRoomStatus(ctx context.Context, scheduler string, roomID string, status entities.GameRoomStatus) error

	GetAllRoomIDs(ctx context.Context, scheduler string) ([]string, error)
	GetRoomIDsByLastPing(ctx context.Context, scheduler string, threshold time.Time) ([]string, error)
	GetRoomCount(ctx context.Context, scheduler string) (int, error)
	GetRoomCountByStatus(ctx context.Context, scheduler string, status entities.GameRoomStatus) (int, error)
}
