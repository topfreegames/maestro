package workers

import "context"

// This interface aims to map all required functions of a worker
type Worker interface {
	// Starts the worker with its own execution
	// configuration details
	Start(ctx context.Context) error
	// Stops the worker
	Stop(ctx context.Context)
	// Returns if the worker is running
	IsRunning(ctx context.Context) bool
	// Returns count of runs of the worker
	CountRuns(ctx context.Context) int
}
