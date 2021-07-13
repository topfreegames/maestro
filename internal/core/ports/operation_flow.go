package ports

import (
	"context"
)

type OperationFlow interface {
	InsertOperationID(ctx context.Context, schedulerName, operationID string) error
	// PopOperation fetches the next scheduler operation to be
	// processed and return its ID.
	NextOperationID(ctx context.Context, schedulerName string) (string, error)
}
