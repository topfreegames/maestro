package operations

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

// Executor is where the actual operation logic is implemented, and it will
// receive as input its correlated definition.
type Executor interface {
	// This is where the operation logic will live; it will receive a context
	// that will be used for deadline and cancelation. This function has only
	// one return which is the operation error (if any);
	Execute(ctx context.Context, op *operation.Operation, definition Definition) error
	// This function is called if Execute returns an error. This will be used
	// for operations that need to do some cleanup or any process if it fails.
	OnError(ctx context.Context, op *operation.Operation, definition Definition, executeErr error) error
	// Name returns the executor name. This is used to identify a executor for a
	// definition.
	Name() string
}
