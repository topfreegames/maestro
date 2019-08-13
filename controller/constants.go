package controller

import "fmt"

// SchedulerVersions rolling update statuses
const (
	timedoutStatus   = "timed out"
	erroredStatus    = "error"
	canceledStatus   = "canceled"
	inProgressStatus = "in progress"
	deployedStatus   = "deployed"
)

var (
	rollbackStatus = func(version string) string {
		return fmt.Sprintf("rolled back to %s", version)
	}
)
