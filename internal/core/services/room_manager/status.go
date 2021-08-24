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

package room_manager

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

// validStatusTransitions this map has all possible status changes for a game
// room.
var validStatusTransitions = map[game_room.GameRoomStatus]map[game_room.GameRoomStatus]struct{}{
	game_room.GameStatusPending: {
		game_room.GameStatusReady:       struct{}{},
		game_room.GameStatusTerminating: struct{}{},
		game_room.GameStatusUnready:     struct{}{},
		game_room.GameStatusError:       struct{}{},
	},
	game_room.GameStatusReady: {
		game_room.GameStatusOccupied:    struct{}{},
		game_room.GameStatusTerminating: struct{}{},
		game_room.GameStatusUnready:     struct{}{},
		game_room.GameStatusError:       struct{}{},
	},
	game_room.GameStatusUnready: {
		game_room.GameStatusTerminating: struct{}{},
		game_room.GameStatusReady:       struct{}{},
		game_room.GameStatusError:       struct{}{},
	},
	game_room.GameStatusOccupied: {
		game_room.GameStatusReady:       struct{}{},
		game_room.GameStatusTerminating: struct{}{},
		game_room.GameStatusUnready:     struct{}{},
		game_room.GameStatusError:       struct{}{},
	},
	game_room.GameStatusError: {
		game_room.GameStatusTerminating: struct{}{},
		game_room.GameStatusUnready:     struct{}{},
		game_room.GameStatusReady:       struct{}{},
	},
	game_room.GameStatusTerminating: {},
}

// validateRoomStatusTransition validates that a transition from currentStatus to newStatus can happen.
func (m *RoomManager) validateRoomStatusTransition(currentStatus game_room.GameRoomStatus, newStatus game_room.GameRoomStatus) error {
	transitions, ok := validStatusTransitions[currentStatus]
	if !ok {
		return fmt.Errorf("game rooms has an invalid status %s", currentStatus.String())
	}

	if _, valid := transitions[newStatus]; !valid {
		return fmt.Errorf("cannot change game room status from %s to %s", currentStatus.String(), newStatus.String())
	}

	return nil
}

// WaitRoomStatus blocks the caller until the context is canceled, an error
// happens in the process or the game room has the desired status.
func (m *RoomManager) WaitRoomStatus(ctx context.Context, gameRoom *game_room.GameRoom, status game_room.GameRoomStatus) error {
	var err error
	watcher, err := m.roomStorage.WatchRoomStatus(ctx, gameRoom)
	if err != nil {
		return fmt.Errorf("failed to start room status watcher: %w", err)
	}

	defer watcher.Stop()

	fromStorage, err := m.roomStorage.GetRoom(ctx, gameRoom.SchedulerID, gameRoom.ID)
	if err != nil {
		return fmt.Errorf("error while retrieving current game room status: %w", err)
	}

	// the room has the desired state already
	if fromStorage.Status == status {
		return nil
	}

watchLoop:
	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break watchLoop
		case gameRoomEvent := <-watcher.ResultChan():
			if gameRoomEvent.Status == status {
				break watchLoop
			}
		}
	}

	if err != nil {
		return fmt.Errorf("failed to wait until room has desired status: %w", err)
	}

	return nil
}
