// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

// RoomStatusPayload is the struct that defines the payload for the status route
type RoomStatusPayload struct {
	Status     string `json:"status" valid:"matches(ready|occupied|terminating|terminated),required"`
	LastStatus string `json:"lastStatus" valid:"matches(creating|ready|occupied|terminating|terminated)"`
	Timestamp  int64  `json:"timestamp" valid:"int64,required"`
}
