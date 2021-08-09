package ports

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

type OperationStorage interface {
	CreateOperation(ctx context.Context, operation *operation.Operation, definitionContent []byte) error
	// GetOperation returns the operation and the definition contents.
	GetOperation(ctx context.Context, schedulerName, operationID string) (*operation.Operation, []byte, error)
	// ListSchedulerActiveOperations list scheduler active operations.
	ListSchedulerActiveOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error)
	// ListSchedulerFinishedOperations list scheduler finished operations.
	ListSchedulerFinishedOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error)
	// UpdateOperationStatus only updates the operation status for the given
	// operation ID.
	UpdateOperationStatus(ctx context.Context, schedulerName, operationID string, status operation.Status) error
}
