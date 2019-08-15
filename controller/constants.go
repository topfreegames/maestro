package controller

import "fmt"

// SchedulerVersions rolling update statuses
const (
	timedoutStatus   = "timed out"
	canceledStatus   = "canceled"
	inProgressStatus = "in progress"
	deployedStatus   = "deployed"
)

var (
	rollbackStatus = func(version string) string {
		return fmt.Sprintf("rolled back to %s", version)
	}

	erroredStatus = func(err string) string {
		return fmt.Sprintf("error: %s", err)
	}
)
