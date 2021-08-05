package ports

import (
	"context"
)

type OperationFlow interface {
	InsertOperationID(ctx context.Context, schedulerName, operationID string) error
	// NextOperationID fetches the next scheduler operation to be
	// processed and return its ID.
	NextOperationID(ctx context.Context, schedulerName string) (string, error)
	// ListSchedulerPendingOperationIDs list scheduler pending operation IDs.
	ListSchedulerPendingOperationIDs(ctx context.Context, schedulerName string) ([]string, error)
}
