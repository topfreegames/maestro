package workers

import "context"

type Worker interface {
	// Starts the worker with its own execution
	// configuration details
	Start(ctx context.Context) error
	// Stops the worker
	Stop(ctx context.Context) error
	// Returns if the worker is running
	IsRunning(ctx context.Context) bool
	// Returns count of runs of the worker
	CountRuns(ctx context.Context) int
}
