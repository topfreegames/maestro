package core

import "fmt"

type Status int

const (
	Unknown Status = iota - 1
	Pending
	Unready
	Ready
	Occupied
	Terminating
	Error
	Terminated
	Active
)

func (s Status) String() string {
	if s == Unknown {
		return "unknown"
	}

	return []string{"pending", "unready", "ready", "occupied", "terminating", "error", "terminated", "active"}[s]
}

func StatusFromString(value string) (Status, error) {
	statuses := map[string]Status{
		"pending":     Pending,
		"unready":     Unready,
		"ready":       Ready,
		"occupied":    Occupied,
		"terminating": Terminating,
		"error":       Error,
		"terminated":  Terminated,
		"active":      Active,
	}

	if status, ok := statuses[value]; ok {
		return status, nil
	}

	return Unknown, fmt.Errorf("invalid value for Status: %s", value)
}

type Room struct {
	ID             string
	Scheduler      string
	Status         Status
	RunningMatches int
}
