// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import "time"

// RoomParams is the struct that defines the params for room routes
type RoomParams struct {
	Name      string `json:"roomName" valid:"required"`
	Scheduler string `json:"schedulerName" valid:"required"`
}

// SchedulerParams is the struct that defines the params for scheduler routes
type SchedulerParams struct {
	SchedulerName string `json:"schedulerName" valid:"required"`
}

// SchedulerEventsParams is the struct that defines the params for scheduler events routes
type SchedulerEventsParams struct {
	SchedulerName string `json:"schedulerName" valid:"required"`
}

// SchedulerRoomsParams is the struct that defines the params for scheduler events routes
type SchedulerRoomsParams struct {
	SchedulerName string `json:"schedulerName" valid:"required"`
	Status        string `json:"status" valid:"required"`
}

// SchedulerRoomDetailsParams is the struct that defines the params for room details routes
type SchedulerRoomDetailsParams struct {
	SchedulerName string `json:"schedulerName" valid:"required"`
	RoomId        string `json:"roomId" valid:"required"`
}

// SchedulerLockParams is the struct that defines the params for scheduler locks routes
type SchedulerLockParams struct {
	SchedulerName string `json:"schedulerName" valid:"required"`
	LockKey       string `json:"lockKey" valid:"optional"`
}

// SchedulerImageParams holds the new image name to be updated
type SchedulerImageParams struct {
	Image     string `json:"image" yaml:"image" valid:"required"`
	Container string `json:"container" yaml:"container"`
}

// SchedulerMinParams holds the new image name to be updated
type SchedulerMinParams struct {
	Min int `json:"min" yaml:"min"`
}

// SchedulerScaleParams holds the new image name to be updated
type SchedulerScaleParams struct {
	ScaleDown uint `json:"scaledown" yaml:"scaledown"`
	ScaleUp   uint `json:"scaleup" yaml:"scaleup"`
	Replicas  uint `json:"replicas" yaml:"replicas"`
}

// SchedulerVersion holds the scheduler version
type SchedulerVersion struct {
	Version   string    `db:"version" json:"version"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

// SchedulersDiff has the diff between schedulers v1 and v2
type SchedulersDiff struct {
	Version1 string `json:"version1"`
	Version2 string `json:"version2"`
	Diff     string `json:"diff"`
}
