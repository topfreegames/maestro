// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

// RoomStatusPayload is the struct that defines the payload for the status route
type RoomStatusPayload struct {
	Metadata  map[string]interface{} `json:"metadata"`
	Status    string                 `json:"status" valid:"matches(ready|occupied|terminating|terminated),required"`
	Timestamp int64                  `json:"timestamp" valid:"int64,required"`
}

// PlayerEventPayload is the struct that defines the payload for the playerEvent route
type PlayerEventPayload struct {
	Metadata  map[string]interface{} `json:"metadata"`
	Event     string                 `json:"status" valid:"matches(playerLeft|playerJoin),required"`
	Timestamp int64                  `json:"timestamp" valid:"int64,required"`
}

// RoomEventPayload is the struct that defines the payload for the roomEvent route
type RoomEventPayload struct {
	Metadata  map[string]interface{} `json:"metadata"`
	Event     string                 `json:"status" valid:"required"`
	Timestamp int64                  `json:"timestamp" valid:"int64,required"`
}
