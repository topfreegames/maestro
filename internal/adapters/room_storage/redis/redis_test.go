//+build integration

package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/ports/errors"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/go-redis/redis"
	"github.com/orlangure/gnomock"
	predis "github.com/orlangure/gnomock/preset/redis"
	"github.com/stretchr/testify/require"
)

var dbNumber int32 = 0
var lastPing = time.Unix(time.Now().Unix(), 0)
var redisContainer *gnomock.Container

func getRedisConnection() *redis.Client {
	db := atomic.AddInt32(&dbNumber, 1)
	return redis.NewClient(&redis.Options{
		Addr: redisContainer.DefaultAddress(),
		DB:   int(db),
	})
}

func TestMain(m *testing.M) {
	var err error
	redisContainer, err = gnomock.Start(predis.Preset())
	if err != nil {
		panic(fmt.Sprintf("error creating redis docker instance: %s\n", err))
	}
	code := m.Run()
	_ = gnomock.Stop(redisContainer)
	os.Exit(code)
}

func assertRedisState(t *testing.T, client *redis.Client, room *game_room.GameRoom) {
	metadataCmd := client.Get(getRoomRedisKey(room.SchedulerID, room.ID))
	require.NoError(t, metadataCmd.Err())

	var actualMetadata map[string]interface{}
	require.NoError(t, json.NewDecoder(strings.NewReader(metadataCmd.Val())).Decode(&actualMetadata))
	require.Equal(t, room.Metadata, actualMetadata)

	statusCmd := client.ZScore(getRoomStatusSetRedisKey(room.SchedulerID), room.ID)
	require.NoError(t, statusCmd.Err())
	require.Equal(t, room.Status, game_room.GameRoomStatus(statusCmd.Val()))

	pingCmd := client.ZScore(getRoomPingRedisKey(room.SchedulerID), room.ID)
	require.NoError(t, pingCmd.Err())
	require.Equal(t, float64(room.LastPingAt.Unix()), pingCmd.Val())
}

func assertRedisStateNonExistent(t *testing.T, client *redis.Client, room *game_room.GameRoom) {
	metadataCmd := client.Get(getRoomRedisKey(room.SchedulerID, room.ID))
	require.Error(t, metadataCmd.Err())

	statusCmd := client.ZScore(getRoomStatusSetRedisKey(room.SchedulerID), room.ID)
	require.Error(t, statusCmd.Err())

	pingCmd := client.ZScore(getRoomPingRedisKey(room.SchedulerID), room.ID)
	require.Error(t, pingCmd.Err())
}

func requireErrorKind(t *testing.T, expected error, err error) {
	require.Error(t, err)
	require.ErrorIs(t, expected, err)
}

func TestRedisStateStorage_CreateRoom(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection()
	storage := NewRedisStateStorage(client)

	t.Run("game room without metadata", func(t *testing.T) {
		room := &game_room.GameRoom{
			ID:          "room-1",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
		}
		require.NoError(t, storage.CreateRoom(ctx, room))
		assertRedisState(t, client, room)
	})

	t.Run("game room with metadata", func(t *testing.T) {
		room := &game_room.GameRoom{
			ID:          "room-2",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		require.NoError(t, storage.CreateRoom(ctx, room))
		assertRedisState(t, client, room)
	})

	t.Run("error when creating existing room", func(t *testing.T) {
		firstRoom := &game_room.GameRoom{
			ID:          "room-3",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		require.NoError(t, storage.CreateRoom(ctx, firstRoom))
		assertRedisState(t, client, firstRoom)

		secondRoom := &game_room.GameRoom{
			ID:          "room-3",
			SchedulerID: "game",
			Status:      game_room.GameStatusOccupied,
			LastPingAt:  lastPing,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		requireErrorKind(t, errors.ErrAlreadyExists, storage.CreateRoom(ctx, secondRoom))
		assertRedisState(t, client, firstRoom)
	})
}

func TestRedisStateStorage_UpdateRoom(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection()
	storage := NewRedisStateStorage(client)

	t.Run("game room without metadata", func(t *testing.T) {
		room := &game_room.GameRoom{
			ID:          "room-1",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
		}

		require.NoError(t, storage.CreateRoom(ctx, room))

		room.Status = game_room.GameStatusOccupied
		require.NoError(t, storage.UpdateRoom(ctx, room))
		assertRedisState(t, client, room)
	})

	t.Run("game room with metadata", func(t *testing.T) {
		room := &game_room.GameRoom{
			ID:          "room-2",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		require.NoError(t, storage.CreateRoom(ctx, room))

		room.Status = game_room.GameStatusOccupied
		require.NoError(t, storage.UpdateRoom(ctx, room))
		assertRedisState(t, client, room)
	})

	t.Run("error when updating non existent room", func(t *testing.T) {
		room := &game_room.GameRoom{
			ID:          "room-3",
			SchedulerID: "game",
			Status:      game_room.GameStatusOccupied,
			LastPingAt:  lastPing,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		requireErrorKind(t, errors.ErrNotFound, storage.UpdateRoom(ctx, room))
	})
}

func TestRedisStateStorage_DeleteRoom(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection()
	storage := NewRedisStateStorage(client)

	t.Run("game room exists", func(t *testing.T) {
		room := &game_room.GameRoom{
			ID:          "room-1",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
		}

		require.NoError(t, storage.CreateRoom(ctx, room))
		require.NoError(t, storage.DeleteRoom(ctx, room.SchedulerID, room.ID))
		assertRedisStateNonExistent(t, client, room)
	})

	t.Run("game room nonexistent", func(t *testing.T) {
		requireErrorKind(t, errors.ErrNotFound, storage.DeleteRoom(ctx, "game", "room-2"))
	})
}

func TestRedisStateStorage_GetRoom(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection()
	storage := NewRedisStateStorage(client)

	t.Run("game room without metadata", func(t *testing.T) {
		expectedRoom := &game_room.GameRoom{
			ID:          "room-1",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
		}

		require.NoError(t, storage.CreateRoom(ctx, expectedRoom))

		actualRoom, err := storage.GetRoom(ctx, expectedRoom.SchedulerID, expectedRoom.ID)
		require.NoError(t, err)
		require.Equal(t, expectedRoom, actualRoom)
	})

	t.Run("game room with metadata", func(t *testing.T) {
		expectedRoom := &game_room.GameRoom{
			ID:          "room-2",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		require.NoError(t, storage.CreateRoom(ctx, expectedRoom))

		actualRoom, err := storage.GetRoom(ctx, expectedRoom.SchedulerID, expectedRoom.ID)
		require.NoError(t, err)
		require.Equal(t, expectedRoom, actualRoom)
	})

	t.Run("error when getting non existent room", func(t *testing.T) {
		_, err := storage.GetRoom(ctx, "game", "room-3")
		requireErrorKind(t, errors.ErrNotFound, err)
	})
}

func TestRedisStateStorage_SetRoomStatus(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection()
	storage := NewRedisStateStorage(client)

	room := &game_room.GameRoom{
		ID:          "room-1",
		SchedulerID: "game",
		Status:      game_room.GameStatusReady,
		LastPingAt:  lastPing,
	}

	sub := client.Subscribe(getRoomStatusUpdateChannel(room.SchedulerID, room.ID))
	defer sub.Close()

	require.NoError(t, storage.CreateRoom(ctx, room))
	require.NoError(t, storage.SetRoomStatus(ctx, room.SchedulerID, room.ID, game_room.GameStatusOccupied))
	room.Status = game_room.GameStatusOccupied
	assertRedisState(t, client, room)

	require.Eventually(t, func() bool {
		select {
		case msg := <-sub.Channel():
			event, err := decodeStatusEvent([]byte(msg.Payload))
			require.NoError(t, err)
			require.Equal(t, room.ID, event.RoomID)
			require.Equal(t, room.Status, event.Status)

			return true
		default:
		}

		return false
	}, time.Second, 100*time.Millisecond)
}

func TestRedisStateStorage_GetAllRoomIDs(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection()
	storage := NewRedisStateStorage(client)

	rooms := []*game_room.GameRoom{
		{
			ID:          "room-1",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
		},
		{
			ID:          "room-2",
			SchedulerID: "game",
			Status:      game_room.GameStatusOccupied,
			LastPingAt:  lastPing,
		},
		{
			ID:          "room-3",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
		},
		{
			ID:          "room-4",
			SchedulerID: "game",
			Status:      game_room.GameStatusPending,
			LastPingAt:  lastPing,
		},
		{
			ID:          "room-5",
			SchedulerID: "game",
			Status:      game_room.GameStatusTerminating,
			LastPingAt:  lastPing,
		},
	}

	for _, room := range rooms {
		require.NoError(t, storage.CreateRoom(ctx, room))
	}

	allRooms, err := storage.GetAllRoomIDs(ctx, "game")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"room-1", "room-2", "room-3", "room-4", "room-5"}, allRooms)
}

func TestRedisStateStorage_GetRoomIDsByLastPing(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection()
	storage := NewRedisStateStorage(client)

	rooms := []*game_room.GameRoom{
		{
			ID:          "room-1",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Unix(1, 0),
		},
		{
			ID:          "room-2",
			SchedulerID: "game",
			Status:      game_room.GameStatusOccupied,
			LastPingAt:  time.Unix(2, 0),
		},
		{
			ID:          "room-3",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Unix(3, 0),
		},
		{
			ID:          "room-4",
			SchedulerID: "game",
			Status:      game_room.GameStatusPending,
			LastPingAt:  time.Unix(4, 0),
		},
		{
			ID:          "room-5",
			SchedulerID: "game",
			Status:      game_room.GameStatusTerminating,
			LastPingAt:  time.Unix(5, 0),
		},
	}

	for _, room := range rooms {
		require.NoError(t, storage.CreateRoom(ctx, room))
	}

	allRooms, err := storage.GetRoomIDsByLastPing(ctx, "game", time.Unix(3, 0))
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"room-1", "room-2", "room-3"}, allRooms)
}

func TestRedisStateStorage_GetRoomCount(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection()
	storage := NewRedisStateStorage(client)

	rooms := []*game_room.GameRoom{
		{
			ID:          "room-1",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Unix(1, 0),
		},
		{
			ID:          "room-2",
			SchedulerID: "game",
			Status:      game_room.GameStatusOccupied,
			LastPingAt:  time.Unix(2, 0),
		},
		{
			ID:          "room-3",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Unix(3, 0),
		},
		{
			ID:          "room-4",
			SchedulerID: "game",
			Status:      game_room.GameStatusPending,
			LastPingAt:  time.Unix(4, 0),
		},
		{
			ID:          "room-5",
			SchedulerID: "game",
			Status:      game_room.GameStatusTerminating,
			LastPingAt:  time.Unix(5, 0),
		},
	}

	for _, room := range rooms {
		require.NoError(t, storage.CreateRoom(ctx, room))
	}

	roomCount, err := storage.GetRoomCount(ctx, "game")
	require.NoError(t, err)
	require.Equal(t, 5, roomCount)
}

func TestRedisStateStorage_GetRoomCountByStatus(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection()
	storage := NewRedisStateStorage(client)

	rooms := []*game_room.GameRoom{
		{
			ID:          "room-1",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Unix(1, 0),
		},
		{
			ID:          "room-2",
			SchedulerID: "game",
			Status:      game_room.GameStatusOccupied,
			LastPingAt:  time.Unix(2, 0),
		},
		{
			ID:          "room-3",
			SchedulerID: "game",
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Unix(3, 0),
		},
		{
			ID:          "room-4",
			SchedulerID: "game",
			Status:      game_room.GameStatusPending,
			LastPingAt:  time.Unix(4, 0),
		},
		{
			ID:          "room-5",
			SchedulerID: "game",
			Status:      game_room.GameStatusTerminating,
			LastPingAt:  time.Unix(5, 0),
		},
	}

	for _, room := range rooms {
		require.NoError(t, storage.CreateRoom(ctx, room))
	}

	roomCountByStatus := map[game_room.GameRoomStatus]int{
		game_room.GameStatusPending:     1,
		game_room.GameStatusUnready:     0,
		game_room.GameStatusReady:       2,
		game_room.GameStatusOccupied:    1,
		game_room.GameStatusTerminating: 1,
		game_room.GameStatusError:       0,
	}

	for status, expectedCount := range roomCountByStatus {
		roomCount, err := storage.GetRoomCountByStatus(ctx, "game", status)
		require.NoError(t, err)
		require.Equal(t, expectedCount, roomCount)
	}
}

func TestRedisStateStorage_WatchRoomStatus(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection()
	storage := NewRedisStateStorage(client)

	room := &game_room.GameRoom{
		ID:          "room-1",
		SchedulerID: "game",
		Status:      game_room.GameStatusReady,
		LastPingAt:  lastPing,
	}

	watcher, err := storage.WatchRoomStatus(ctx, room)
	defer watcher.Stop()
	require.NoError(t, err)

	require.NoError(t, storage.CreateRoom(ctx, room))
	require.NoError(t, storage.SetRoomStatus(ctx, room.SchedulerID, room.ID, game_room.GameStatusOccupied))

	require.Eventually(t, func() bool {
		select {
		case event := <-watcher.ResultChan():
			require.Equal(t, room.ID, event.RoomID)
			require.Equal(t, game_room.GameStatusOccupied, event.Status)
			return true
		default:
		}

		return false
	}, time.Second, 100*time.Millisecond)
}
