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

	"github.com/topfreegames/maestro/internal/core/operations/rooms/add"
	"github.com/topfreegames/maestro/internal/core/operations/rooms/remove"
	"github.com/topfreegames/maestro/internal/core/operations/schedulers/newversion"
	"github.com/topfreegames/maestro/internal/core/services/events"
	operationmanager "github.com/topfreegames/maestro/internal/core/services/operations"
	roommanager "github.com/topfreegames/maestro/internal/core/services/rooms"
	"github.com/topfreegames/maestro/internal/core/services/workers"

	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"
	"github.com/topfreegames/maestro/internal/core/worker"

	"github.com/topfreegames/maestro/internal/config"
)

const (
	healthControllerExecutionIntervalConfigPath  = "workers.healthControllerInterval"
	storagecleanupExecutionIntervalConfigPath    = "workers.storageClenupInterval"
	roomInitializationTimeoutMillisConfigPath    = "services.roomManager.roomInitializationTimeoutMillis"
	roomRoomValidationAttemptsConfigPath         = "services.roomManager.roomValidationAttempts"
	roomPingTimeoutMillisConfigPath              = "services.roomManager.roomPingTimeoutMillis"
	roomDeletionTimeoutMillisConfigPath          = "services.roomManager.roomDeletionTimeoutMillis"
	operationLeaseTTLMillisConfigPath            = "services.operationManager.operationLeaseTTLMillis"
	schedulerCacheTTLMillisConfigPath            = "services.eventsForwarder.schedulerCacheTTLMillis"
	operationsRoomsAddLimitConfigPath            = "operations.rooms.add.limit"
	operationsRoomsAddBatchSizePath              = "operations.rooms.add.batchSize"
	operationsRoomsAddOperationTimeoutConfigPath = "operations.rooms.add.operationTimeout"
	operationsRoomsRemoveLimitConfigPath         = "operations.rooms.remove.limit"
)

// NewCreateSchedulerVersionConfig instantiate a new CreateSchedulerVersionConfig to be used by the NewSchedulerVersion operation to customize its configuration.
func NewCreateSchedulerVersionConfig(c config.Config) newversion.Config {
	initializationTimeout := time.Duration(c.GetInt(roomInitializationTimeoutMillisConfigPath)) * time.Millisecond
	roomValidationAttempts := c.GetInt(roomRoomValidationAttemptsConfigPath)
	if roomValidationAttempts < 1 {
		roomValidationAttempts = 1
	}

	createSchedulerVersionConfig := newversion.Config{
		RoomInitializationTimeout: initializationTimeout,
		RoomValidationAttempts:    roomValidationAttempts,
	}

	return createSchedulerVersionConfig
}

// NewHealthControllerConfig instantiate a new HealthControllerConfig to be used by the HealthController to customize its configuration.
func NewHealthControllerConfig(c config.Config) healthcontroller.Config {
	initializationTimeout := time.Duration(c.GetInt(roomInitializationTimeoutMillisConfigPath)) * time.Millisecond
	pingTimeout := time.Duration(c.GetInt(roomPingTimeoutMillisConfigPath)) * time.Millisecond
	deletionTimeout := time.Duration(c.GetInt(roomDeletionTimeoutMillisConfigPath)) * time.Millisecond

	config := healthcontroller.Config{
		RoomInitializationTimeout: initializationTimeout,
		RoomPingTimeout:           pingTimeout,
		RoomDeletionTimeout:       deletionTimeout,
	}

	return config
}

// NewOperationRoomsAddConfig instantiate a new add.Config to be used by the rooms add operation.
func NewOperationRoomsAddConfig(c config.Config) add.Config {
	operationsRoomsAddLimit := int32(c.GetInt(operationsRoomsAddLimitConfigPath))
	operationsRoomsAddBatchSize := int32(c.GetInt(operationsRoomsAddBatchSizePath))
	operationsRoomsAddOperationTimeout := c.GetDuration(operationsRoomsAddOperationTimeoutConfigPath)

	config := add.Config{
		AmountLimit:      operationsRoomsAddLimit,
		BatchSize:        operationsRoomsAddBatchSize,
		OperationTimeout: operationsRoomsAddOperationTimeout,
	}

	return config
}

// NewOperationRoomsRemoveConfig instantiate a new remove.Config to be used by the rooms remove operation.
func NewOperationRoomsRemoveConfig(c config.Config) remove.Config {
	operationsRoomsRemoveLimit := c.GetInt(operationsRoomsRemoveLimitConfigPath)

	config := remove.Config{
		AmountLimit: operationsRoomsRemoveLimit,
	}

	return config
}

// NewRoomManagerConfig instantiate a new RoomManagerConfig to be used by the RoomManager to customize its configuration.
func NewRoomManagerConfig(c config.Config) (roommanager.RoomManagerConfig, error) {
	pingTimeout := time.Duration(c.GetInt(roomPingTimeoutMillisConfigPath)) * time.Millisecond
	deletionTimeout := time.Duration(c.GetInt(roomDeletionTimeoutMillisConfigPath)) * time.Millisecond
	schedulerCacheTTL := getSchedulerCacheTTL(c)

	roomManagerConfig := roommanager.RoomManagerConfig{
		RoomPingTimeout:     pingTimeout,
		RoomDeletionTimeout: deletionTimeout,
		SchedulerCacheTtl:   schedulerCacheTTL,
	}

	return roomManagerConfig, nil
}

// NewWorkersConfig instantiate a new workers Config stucture to be used by the workers to customize them from the config package.
func NewWorkersConfig(c config.Config) (worker.Configuration, error) {
	healthControllerExecutionInterval := c.GetDuration(healthControllerExecutionIntervalConfigPath)
	storagecleanupExecutionInterval := c.GetDuration(storagecleanupExecutionIntervalConfigPath)
	workersStopTimeoutDuration := c.GetDuration(workers.WorkersStopTimeoutDurationPath)
	config := worker.Configuration{
		HealthControllerExecutionInterval: healthControllerExecutionInterval,
		StorageCleanupExecutionInterval:   storagecleanupExecutionInterval,
		WorkersStopTimeoutDuration:        workersStopTimeoutDuration,
	}

	return config, nil
}

// NewOperationManagerConfig instantiate a new OperationManagerConfig to be used by the OperationManager to customize its configuration.
func NewOperationManagerConfig(c config.Config) (operationmanager.OperationManagerConfig, error) {
	operationLeaseTTL := time.Duration(c.GetInt(operationLeaseTTLMillisConfigPath)) * time.Millisecond

	operationManagerConfig := operationmanager.OperationManagerConfig{
		OperationLeaseTtl: operationLeaseTTL,
	}

	return operationManagerConfig, nil
}

// NewEventsForwarderServiceConfig instantiate a new EventsForwarderConfig to be used by the EventsForwarder to customize its configuration.
func NewEventsForwarderServiceConfig(c config.Config) (events.EventsForwarderConfig, error) {
	schedulerCacheTTL := getSchedulerCacheTTL(c)

	eventsForwarderConfig := events.EventsForwarderConfig{
		SchedulerCacheTtl: schedulerCacheTTL,
	}

	return eventsForwarderConfig, nil
}

// getSchedulerCacheTTL returns the scheduler cache TTL from config or default value
func getSchedulerCacheTTL(c config.Config) time.Duration {
	defaultSchedulerCacheTTL := 5 * time.Minute

	if configuredSchedulerCacheTTLInt := c.GetInt(schedulerCacheTTLMillisConfigPath); configuredSchedulerCacheTTLInt > 0 {
		return time.Duration(configuredSchedulerCacheTTLInt) * time.Millisecond
	}
	return defaultSchedulerCacheTTL
}
