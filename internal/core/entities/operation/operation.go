package operation

import "fmt"

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

func (s Status) String() (string, error) {
	switch s {
	case StatusPending:
		return "pending", nil
	case StatusInProgress:
		return "in_progress", nil
	case StatusFinished:
		return "finished", nil
	case StatusEvicted:
		return "evicted", nil
	case StatusError:
		return "error", nil
	case StatusCanceled:
		return "canceled", nil
	}

	return "", fmt.Errorf("status could not be mapped to string: %d", s)
}
