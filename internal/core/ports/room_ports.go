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

package ports

import (
	"context"
	"sync"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

// Primary ports (input, driving ports)

// RoomManager is an interface for working with rooms in a high level way
type RoomManager interface {
	// DeleteRoomAndWaitForRoomTerminating receives a room to be deleted, and only finish execution when the
	// room reach terminating status. If the room cannot be terminated by any means, it will return an error
	// specifying why
	DeleteRoomAndWaitForRoomTerminating(ctx context.Context, gameRoom *game_room.GameRoom) error
	// SchedulerMaxSurge calculates the current scheduler max surge based on
	// the number of rooms the scheduler has.
	SchedulerMaxSurge(ctx context.Context, scheduler *entities.Scheduler) (int, error)
	// ListRoomsWithDeletionPriority returns a specified number of rooms, following
	// the priority of it being deleted and filtering the ignored version,
	// the function will return rooms discarding such filter option.
	//
	// The priority is:
	//
	// - On error rooms;
	// - No ping received for x time rooms;
	// - Pending rooms;
	// - Ready rooms;
	// - Occupied rooms;
	//
	// This function can return less rooms than the `amount` since it might not have
	// enough rooms on the scheduler.
	ListRoomsWithDeletionPriority(ctx context.Context, schedulerName, ignoredVersion string, amount int, roomsBeingReplaced *sync.Map) ([]*game_room.GameRoom, error)
	// CleanRoomState cleans the remaining state of a room. This function is
	// intended to be used after a `DeleteRoomAndWaitForRoomTerminating`, where the room instance is
	// signaled to terminate.
	//
	// It wouldn't return an error if the room was already cleaned.
	CleanRoomState(ctx context.Context, schedulerName, roomId string) error
	// UpdateRoomInstance updates the instance information.
	UpdateRoomInstance(ctx context.Context, gameRoomInstance *game_room.Instance) error
	// UpdateRoom updates the game room information.
	UpdateRoom(ctx context.Context, gameRoom *game_room.GameRoom) error
	// CreateRoomAndWaitForReadiness creates a room and only returns it when the room is ready (sent a ping to maestro).
	// If the room is created and don't succeed on sending a ping to maestro, it will try to delete the room, and return
	// an error. The "isValidationRoom" parameter must be passed "true" if we want to create a room that won't be forwarded
	CreateRoomAndWaitForReadiness(ctx context.Context, scheduler entities.Scheduler, isValidationRoom bool) (*game_room.GameRoom, *game_room.Instance, error)
	// CreateRoom creates a game room in maestro runtime and storages without waiting the room to reach ready status.
	CreateRoom(ctx context.Context, scheduler entities.Scheduler, isValidationRoom bool) (*game_room.GameRoom, *game_room.Instance, error)
	// GetRoomInstance returns the game room instance.
	GetRoomInstance(ctx context.Context, scheduler, roomID string) (*game_room.Instance, error)
	// UpdateGameRoomStatus updates the game based on the ping and runtime status.
	UpdateGameRoomStatus(ctx context.Context, schedulerId, gameRoomId string) error
	// WaitRoomStatus blocks the caller until the context is canceled, an error
	// happens in the process or the game room has the desired status.
	WaitRoomStatus(ctx context.Context, gameRoom *game_room.GameRoom, status game_room.GameRoomStatus) error
}

// Secondary Ports (output, driven ports)

// RoomStorage is an interface for retrieving and updating room status and ping information
type RoomStorage interface {
	// GetRoom retrieves a specific room from a scheduler name and roomID
	// returns an error when the room does not exist
	GetRoom(ctx context.Context, scheduler string, roomID string) (*game_room.GameRoom, error)
	// CreateRoom creates a room and returns an error if the room already exists
	CreateRoom(ctx context.Context, room *game_room.GameRoom) error
	// UpdateRoom updates a room metadata, status and lastPingAt then publish an status update event
	// It returns an error if the room does not exist
	UpdateRoom(ctx context.Context, room *game_room.GameRoom) error
	// DeleteRoom deletes a room and returns an error if the room does not exist
	DeleteRoom(ctx context.Context, scheduler string, roomID string) error
	// GetAllRoomIDs gets all room ids in a scheduler
	GetAllRoomIDs(ctx context.Context, scheduler string) ([]string, error)
	// GetRoomIDsByStatus gets all room ids in a scheduler by a specific status
	GetRoomIDsByStatus(ctx context.Context, scheduler string, status game_room.GameRoomStatus) ([]string, error)
	// GetRoomIDsByLastPing gets all room ids in a scheduler where ping is less than threshold
	GetRoomIDsByLastPing(ctx context.Context, scheduler string, threshold time.Time) ([]string, error)
	// GetRoomCount gets the total count of rooms in a scheduler
	GetRoomCount(ctx context.Context, scheduler string) (int, error)
	// GetRoomCountByStatus gets the count of rooms with a specific status in a scheduler
	GetRoomCountByStatus(ctx context.Context, scheduler string, status game_room.GameRoomStatus) (int, error)
	// UpdateRoomStatus updates the game room status.
	UpdateRoomStatus(ctx context.Context, scheduler, roomId string, status game_room.GameRoomStatus) error
	// WatchRoomStatus watch for status changes on the storage.
	WatchRoomStatus(ctx context.Context, room *game_room.GameRoom) (RoomStorageStatusWatcher, error)
}

// RoomStorageStatusWatcher defines a process of watcher, it will have a chan
// with the game rooms status changes.
type RoomStorageStatusWatcher interface {
	// ResultChan returns the channel where the changes will be forwarded.
	ResultChan() chan game_room.StatusEvent
	// Stop stops the watcher.
	Stop()
}
