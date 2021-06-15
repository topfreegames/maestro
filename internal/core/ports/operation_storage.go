package ports

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

type OperationStorage interface {
	CreateOperation(ctx context.Context, operation *operation.Operation, definitionContent []byte) error
	// GetOperation returns the operation and the definition contents.
	GetOperation(ctx context.Context, schedulerName, operationID string) (*operation.Operation, []byte, error)
}
