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

package newschedulerversion

import (
	"context"
	"sync"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/services/interfaces"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"
	"go.uber.org/zap"
)

type CreateNewSchedulerVersionExecutor struct {
	roomManager        *room_manager.RoomManager
	schedulerManager   interfaces.SchedulerManager
	roomsBeingReplaced *sync.Map
}

func NewExecutor(roomManager *room_manager.RoomManager, schedulerManager interfaces.SchedulerManager) *CreateNewSchedulerVersionExecutor {
	return &CreateNewSchedulerVersionExecutor{
		roomManager:        roomManager,
		schedulerManager:   schedulerManager,
		roomsBeingReplaced: &sync.Map{},
	}
}

// Execute the process of updating a scheduler consists of the following:
// 1. Update the scheduler configuration using the one present on the operation
//    definition;
// 2. If this update creates a minor version, the operation is done since there
//    is no necessity of replacing game rooms;
// 3. For a major version, Fetch the MaxSurge for the scheduler;
// 4. Creates "replace" goroutines (same number as MaxSurge);
// 5. Each goroutine will listen to a channel and create a new room using the
//    new configuration. After the room is ready, it will then delete the room
//    being replaced;
// 6. List all game rooms that need to be replaced and produce them into the
//    replace goroutines channel;
func (ex *CreateNewSchedulerVersionExecutor) Execute(ctx context.Context, op *operation.Operation, definition operations.Definition) error {
	logger := zap.L().With(
		zap.String("scheduler_name", op.SchedulerName),
		zap.String("operation_definition", definition.Name()),
		zap.String("operation_id", op.ID),
	)
	logger.Debug("start updating scheduler")

	// Check if it is major

	// If it is major, validate game room

	// Create new scheduler version in the DB

	// Enqueue switch active version operation

	logger.Debug("scheduler update finishes with success")
	return nil
}

func (ex *CreateNewSchedulerVersionExecutor) OnError(ctx context.Context, op *operation.Operation, definition operations.Definition, executeErr error) error {
	return nil
}

func (ex *CreateNewSchedulerVersionExecutor) Name() string {
	return OperationName
}
