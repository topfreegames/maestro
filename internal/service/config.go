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

	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"
	"github.com/topfreegames/maestro/internal/core/operations/newschedulerversion"
	"github.com/topfreegames/maestro/internal/core/services/events_forwarder"
	"github.com/topfreegames/maestro/internal/core/workers"

	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
)

const healthControllerExecutionIntervalConfigPath = "workers.operationExecution.healthControllerInterval"

// NewCreateSchedulerVersionConfig instantiate a new CreateSchedulerVersionConfig to be used by the NewSchedulerVersion operation to customize its configuration.
func NewCreateSchedulerVersionConfig(c config.Config) newschedulerversion.Config {
	initializationTimeout := time.Duration(c.GetInt("services.roomManager.roomInitializationTimeoutMillis")) * time.Millisecond

	createSchedulerVersionConfig := newschedulerversion.Config{
		RoomInitializationTimeout: initializationTimeout,
	}

	return createSchedulerVersionConfig
}

// NewHealthControllerConfig instantiate a new HealthControllerConfig to be used by the HealthController to customize its configuration.
func NewHealthControllerConfig(c config.Config) healthcontroller.Config {
	initializationTimeout := time.Duration(c.GetInt("services.roomManager.roomInitializationTimeoutMillis")) * time.Millisecond
	pingTimeout := time.Duration(c.GetInt("services.roomManager.roomPingTimeoutMillis")) * time.Millisecond

	config := healthcontroller.Config{
		RoomInitializationTimeout: initializationTimeout,
		RoomPingTimeout:           pingTimeout,
	}

	return config
}

// NewRoomManagerConfig instantiate a new RoomManagerConfig to be used by the RoomManager to customize its configuration.
func NewRoomManagerConfig(c config.Config) (room_manager.RoomManagerConfig, error) {
	pingTimeout := time.Duration(c.GetInt("services.roomManager.roomPingTimeoutMillis")) * time.Millisecond
	deletionTimeout := time.Duration(c.GetInt("services.roomManager.roomDeletionTimeoutMillis")) * time.Millisecond

	roomManagerConfig := room_manager.RoomManagerConfig{
		RoomPingTimeout:     pingTimeout,
		RoomDeletionTimeout: deletionTimeout,
	}

	return roomManagerConfig, nil
}

// NewWorkersConfig instantiate a new workers Config stucture to be used by the workers to customize them from the config package.
func NewWorkersConfig(c config.Config) (workers.Configuration, error) {
	healthControllerExecutionInterval := c.GetDuration(healthControllerExecutionIntervalConfigPath)
	config := workers.Configuration{
		HealthControllerExecutionInterval: healthControllerExecutionInterval,
	}

	return config, nil
}

// NewOperationManagerConfig instantiate a new OperationManagerConfig to be used by the OperationManager to customize its configuration.
func NewOperationManagerConfig(c config.Config) (operation_manager.OperationManagerConfig, error) {
	operationLeaseTTL := time.Duration(c.GetInt("services.operationManager.operationLeaseTtlMillis")) * time.Millisecond

	operationManagerConfig := operation_manager.OperationManagerConfig{
		OperationLeaseTtl: operationLeaseTTL,
	}

	return operationManagerConfig, nil
}

// NewEventsForwarderServiceConfig instantiate a new EventsForwarderConfig to be used by the EventsForwarder to customize its configuration.
func NewEventsForwarderServiceConfig(c config.Config) (events_forwarder.EventsForwarderConfig, error) {
	var schedulerCacheTTL time.Duration
	defaultSchedulerCacheTTL := time.Hour * 24

	if configuredSchedulerCacheTTLInt := c.GetInt("services.eventsForwarder.schedulerCacheTtlMillis"); configuredSchedulerCacheTTLInt > 0 {
		schedulerCacheTTL = time.Duration(configuredSchedulerCacheTTLInt) * time.Millisecond
	} else {
		schedulerCacheTTL = defaultSchedulerCacheTTL
	}

	eventsForwarderConfig := events_forwarder.EventsForwarderConfig{
		SchedulerCacheTtl: schedulerCacheTTL,
	}

	return eventsForwarderConfig, nil
}
