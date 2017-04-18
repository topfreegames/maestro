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
	ConfigName string `json:"schedulerName" valid:"required"`
}
