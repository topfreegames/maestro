// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package service

import (
	"time"

	"github.com/topfreegames/maestro/internal/core/services/events_forwarder"

	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
)

func NewRoomManagerConfig(c config.Config) (room_manager.RoomManagerConfig, error) {
	pingTimeout := time.Duration(c.GetInt("services.roomManager.roomPingTimeoutMillis")) * time.Millisecond
	initializationTimeout := time.Duration(c.GetInt("services.roomManager.roomInitializationTimeoutMillis")) * time.Millisecond
	deletionTimeout := time.Duration(c.GetInt("services.roomManager.roomDeletionTimeoutMillis")) * time.Millisecond

	roomManagerConfig := room_manager.RoomManagerConfig{
		RoomPingTimeout:           pingTimeout,
		RoomInitializationTimeout: initializationTimeout,
		RoomDeletionTimeout:       deletionTimeout,
	}

	return roomManagerConfig, nil
}

func NewOperationManagerConfig(c config.Config) (operation_manager.OperationManagerConfig, error) {
	operationLeaseTtl := time.Duration(c.GetInt("services.operationManager.operationLeaseTtlMillis")) * time.Millisecond

	operationManagerConfig := operation_manager.OperationManagerConfig{
		OperationLeaseTtl: operationLeaseTtl,
	}

	return operationManagerConfig, nil
}

func NewEventsForwarderServiceConfig(c config.Config) (events_forwarder.EventsForwarderConfig, error) {
	var schedulerCacheTtl time.Duration
	defaultSchedulerCacheTtl := time.Hour * 24

	if configuredSchedulerCacheTtlInt := c.GetInt("services.eventsForwarder.schedulerCacheTtlMillis"); configuredSchedulerCacheTtlInt > 0 {
		schedulerCacheTtl = time.Duration(configuredSchedulerCacheTtlInt) * time.Millisecond
	} else {
		schedulerCacheTtl = defaultSchedulerCacheTtl
	}

	eventsForwarderConfig := events_forwarder.EventsForwarderConfig{
		SchedulerCacheTtl: schedulerCacheTtl,
	}

	return eventsForwarderConfig, nil
}
