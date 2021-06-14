package ports

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

type OperationStorage interface {
	CreateOperation(ctx context.Context, operation *operation.Operation) error
}
