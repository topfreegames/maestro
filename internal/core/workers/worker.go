package workers

import "context"

type Worker interface {
	// Starts the worker with its own execution
	// configuration details
	Start(ctx context.Context) error
	// Stops the worker
	Stop(ctx context.Context) error
}
