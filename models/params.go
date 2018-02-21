// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

// RoomParams is the struct that defines the params for room routes
type RoomParams struct {
	Name      string `json:"roomName" valid:"required"`
	Scheduler string `json:"schedulerName" valid:"required"`
}

// SchedulerParams is the struct that defines the params for scheduler routes
type SchedulerParams struct {
	SchedulerName string `json:"schedulerName" valid:"required"`
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
