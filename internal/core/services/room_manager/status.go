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

// SetRoomStatus changes the game room status in the storage. It takes into
// count the current status.
func (m *RoomManager) SetRoomStatus(ctx context.Context, gameRoom *game_room.GameRoom, status game_room.GameRoomStatus) error {
	transitions, ok := validStatusTransitions[gameRoom.Status]
	if !ok {
		return fmt.Errorf("game rooms has an invalid status %s", gameRoom.Status.String())
	}

	if _, valid := transitions[status]; !valid {
		return fmt.Errorf("cannot change game room status from %s to %s", gameRoom.Status.String(), status.String())
	}

	err := m.roomStorage.SetRoomStatus(ctx, gameRoom.SchedulerID, gameRoom.ID, status)
	if err != nil {
		return fmt.Errorf("failed to update game room status on storage: %w", err)
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
