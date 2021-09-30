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
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

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
	// GetRoomIDsByLastPing gets all room ids in a scheduler where ping is less than threshold
	GetRoomIDsByLastPing(ctx context.Context, scheduler string, threshold time.Time) ([]string, error)
	// GetRoomCount gets the total count of rooms in a scheduler
	GetRoomCount(ctx context.Context, scheduler string) (int, error)
	// GetRoomCountByStatus gets the count of rooms with a specific status in a scheduler
	GetRoomCountByStatus(ctx context.Context, scheduler string, status game_room.GameRoomStatus) (int, error)
	// UpdateRoomStatus updates the game room status.
	UpdateRoomStatus(ctx context.Context, scheduler , roomId string, status game_room.GameRoomStatus) error

	// WatchRoomStatus watche for status changes on the storage.
	WatchRoomStatus(ctx context.Context, room *game_room.GameRoom) (RoomStorageStatusWatcher, error)
}

// RoomStorageStatusWatcher  defines a process of watcher, it will have a chan
// with the game rooms status changes.
type RoomStorageStatusWatcher interface {
	// ResultChan returns the channel where the changes will be forwarded.
	ResultChan() chan game_room.StatusEvent
	// Stop stops the watcher.
	Stop()
}
