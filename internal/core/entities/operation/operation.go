package operation

type Status int

const (
	// Operation was created but no worker has started it yet.
	
	StatusPending Status = iota
	// Operation is being executed by a worker.
	StatusInProgress
	// Operation was canceled, usually by an external action or a timeout.
	StatusCanceled
	// Operation finished with success.
	StatusFinished
	// Operation could not execute because it didn’t meet the requirements.
	StatusEvicted
	// Operation finished with error.
	StatusError
)

type Operation struct {
	ID             string
	Status         Status
	DefinitionName string
	SchedulerName  string
}