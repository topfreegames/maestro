// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"time"
)


// SchedulerEvent is the struct that defines a maestro scheduler event
type SchedulerEvent struct {
	Name          string                 `json:"name"`
	SchedulerName string                 `json:"schedulerName"`
	CreatedAt     time.Time              `json:"createdAt"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// NewSchedulerEvent is the scheduler event constructor
func NewSchedulerEvent(eventName, schedulerName string , metadata map[string]interface{}) *SchedulerEvent {
	return &SchedulerEvent{
		Name:          eventName,
		SchedulerName: schedulerName,
		CreatedAt:     time.Now(),
		Metadata:      metadata,
	}
}
