package ports

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities"
)

type SchedulerStorage interface {
	GetScheduler(ctx context.Context, id string) (*entities.Scheduler, error)
	GetSchedulers(ctx context.Context, ids []string) ([]*entities.Scheduler, error)
	GetAllSchedulers(ctx context.Context) ([]*entities.Scheduler, error)
	CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) error
	UpdateScheduler(ctx context.Context, scheduler *entities.Scheduler) error
	DeleteScheduler(ctx context.Context, scheduler *entities.Scheduler) error
}
