package operation

import "context"

// Executor is where the actual operation logic is implemented, and it will
// receive as input its correlated definition.
type Executor interface {
	// This is where the operation logic will live; it will receive a context
	// that will be used for deadline and cancelation. This function has only
	// one return which is the operation error (if any);
	Execute(ctx context.Context, operation *Operation, definition Definition) error

	// This function is called if Execute returns an error. This will be used
	// for operations that need to do some cleanup or any process if it fails.
	OnError(ctx context.Context, executeErr error) error
}
