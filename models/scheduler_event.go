// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"fmt"
	"time"
)

const (
	// AutoScale events.
	StartAutoScaleEventName    = "AUTO_SCALE_START"
	FinishedAutoScaleEventName = "AUTO_SCALE_FINISHED"
	FailedAutoScaleEventName   = "AUTO_SCALE_FAILED"

	// Remove dead rooms.
	StartRemoveDeadRoomsEventName    = "REMOVE_DEAD_ROOMS_START"
	FinishedRemoveDeadRoomsEventName = "REMOVE_DEAD_ROOMS_FINISHED"

	// Worker update events.
	StartWorkerUpdateEventName    = "WORKER_UPDATE_STARTED"
	FailedWorkerUpdateEventName   = "WORKER_UPDATE_FAILED"
	FinishedWorkerUpdateEventName = "WORKER_UPDATE_FINISHED"

	// Worker update events.
	StartUpdateEventName    = "UPDATE_STARTED"
	FailedUpdateEventName   = "UPDATE_FAILED"
	FinishedUpdateEventName = "UPDATE_FINISHED"

	// Rollback events.
	TriggerRollbackEventName = "ROLLBACK_TRIGGERED"

	// Metadata attributes name.

	// ErrorMetadaName metadata containing an error.
	ErrorMetadataName = "error"
	// TypeMetadataName type of the operation. For example, AutoScale has "up"
	// and "down" types.
	TypeMetadataName = "type"
	// AmountMetadataName amount of rooms that are going to be manipulated.
	AmountMetadataName = "amount"
	// SucessMetadataName indicates if the remove dead rooms finished successfully.
	SuccessMetadataName = "success"
	// SchedulerVersionMetadataName current scheduler version.
	SchedulerVersionMetadataName = "scheduler_version"
	// InvalidVersionAmountMetadataName amount of rooms with invalid version.
	InvalidVersionAmountMetadataName = "invalid_version_amount"
	// UnregisteredAmountMetadataName amount of rooms that are not present on
	// Maestro's state.
	UnregisteredAmountMetadataName = "unregistered_amount"
)

// SchedulerEvent is the struct that defines a maestro scheduler event
type SchedulerEvent struct {
	Name          string                 `json:"name"`
	SchedulerName string                 `json:"schedulerName"`
	CreatedAt     time.Time              `json:"createdAt"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// NewSchedulerEvent is the scheduler event constructor
func NewSchedulerEvent(eventName, schedulerName string, metadata map[string]interface{}) *SchedulerEvent {
	metadata = stringfyMetadataError(metadata)

	return &SchedulerEvent{
		Name:          eventName,
		SchedulerName: schedulerName,
		CreatedAt:     time.Now(),
		Metadata:      metadata,
	}
}

func stringfyMetadataError(metadata map[string]interface{}) map[string]interface{} {
	if metadataErr, existsError := metadata[ErrorMetadataName]; existsError {
		if err, isError := metadataErr.(error); isError {
			metadata[ErrorMetadataName] = err.Error()
		} else {
			metadata[ErrorMetadataName] = fmt.Sprintf("%v", metadataErr)
		}
	}
	return metadata
}
