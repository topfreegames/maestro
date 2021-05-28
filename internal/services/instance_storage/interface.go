package instance_storage

import (
	"context"

	"github.com/topfreegames/maestro/internal/entities"
)

type RoomInstanceStorage interface {
	GetInstance(ctx context.Context, scheduler string, roomId string) (*entities.GameRoomInstance, error)
	AddInstance(ctx context.Context, instance *entities.GameRoomInstance) error
	RemoveInstance(ctx context.Context, scheduler string, roomId string) error
	GetAllInstances(ctx context.Context, scheduler string) ([]*entities.GameRoomInstance, error)
	GetInstanceCount(ctx context.Context, scheduler string) (int, error)
}
