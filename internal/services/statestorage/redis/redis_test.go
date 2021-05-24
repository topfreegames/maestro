//+build integration

package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/orlangure/gnomock"
	predis "github.com/orlangure/gnomock/preset/redis"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/entities"
	"github.com/topfreegames/maestro/internal/services/statestorage"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var dbNumber int32 = 0
var lastPing = time.Unix(time.Now().Unix(), 0)
var redisContainer *gnomock.Container

func getRedisConnection(t *testing.T) *redis.Client {
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

func assertRedisState(t *testing.T, client *redis.Client, room *entities.GameRoom) {
	metadataCmd := client.Get(getRoomRedisKey(room.Scheduler, room.ID))
	require.NoError(t, metadataCmd.Err())

	var actualMetadata map[string]interface{}
	require.NoError(t, json.NewDecoder(strings.NewReader(metadataCmd.Val())).Decode(&actualMetadata))
	require.Equal(t, room.Metadata, actualMetadata)

	statusCmd := client.ZScore(getRoomStatusSetRedisKey(room.Scheduler), room.ID)
	require.NoError(t, statusCmd.Err())
	require.Equal(t, room.Status, entities.GameRoomStatus(statusCmd.Val()))

	pingCmd := client.ZScore(getRoomPingRedisKey(room.Scheduler), room.ID)
	require.NoError(t, pingCmd.Err())
	require.Equal(t, float64(room.LastPingAt.Unix()), pingCmd.Val())
}

func assertRedisStateNonExistent(t *testing.T, client *redis.Client, room *entities.GameRoom) {
	metadataCmd := client.Get(getRoomRedisKey(room.Scheduler, room.ID))
	require.Error(t, metadataCmd.Err())

	statusCmd := client.ZScore(getRoomStatusSetRedisKey(room.Scheduler), room.ID)
	require.Error(t, statusCmd.Err())

	pingCmd := client.ZScore(getRoomPingRedisKey(room.Scheduler), room.ID)
	require.Error(t, pingCmd.Err())
}

func requireErrorKind(t *testing.T, kind statestorage.ErrorKind, err error) {
	require.Error(t, err)
	require.Implements(t, (*statestorage.StateStorageError)(nil), err)
	require.Equal(t, err.(statestorage.StateStorageError).Kind(), kind)
}

func TestRedisStateStorage_CreateRoom(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection(t)
	storage := NewRedisStateStorage(client)

	t.Run("game room without metadata", func(t *testing.T) {
		room := &entities.GameRoom{
			ID:         "room-1",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: lastPing,
		}
		require.NoError(t, storage.CreateRoom(ctx, room))
		assertRedisState(t, client, room)
	})

	t.Run("game room with metadata", func(t *testing.T) {
		room := &entities.GameRoom{
			ID:         "room-2",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: lastPing,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		require.NoError(t, storage.CreateRoom(ctx, room))
		assertRedisState(t, client, room)
	})

	t.Run("error when creating existing room", func(t *testing.T) {
		firstRoom := &entities.GameRoom{
			ID:         "room-3",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: lastPing,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		require.NoError(t, storage.CreateRoom(ctx, firstRoom))
		assertRedisState(t, client, firstRoom)

		secondRoom := &entities.GameRoom{
			ID:         "room-3",
			Scheduler:  "game",
			Status:     entities.GameStatusOccupied,
			LastPingAt: lastPing,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		requireErrorKind(t, statestorage.ErrorRoomAlreadyExists, storage.CreateRoom(ctx, secondRoom))
		assertRedisState(t, client, firstRoom)
	})
}

func TestRedisStateStorage_UpdateRoom(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection(t)
	storage := NewRedisStateStorage(client)

	t.Run("game room without metadata", func(t *testing.T) {
		room := &entities.GameRoom{
			ID:         "room-1",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: lastPing,
		}

		require.NoError(t, storage.CreateRoom(ctx, room))

		room.Status = entities.GameStatusOccupied
		require.NoError(t, storage.UpdateRoom(ctx, room))
		assertRedisState(t, client, room)
	})

	t.Run("game room with metadata", func(t *testing.T) {
		room := &entities.GameRoom{
			ID:         "room-2",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: lastPing,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		require.NoError(t, storage.CreateRoom(ctx, room))

		room.Status = entities.GameStatusOccupied
		require.NoError(t, storage.UpdateRoom(ctx, room))
		assertRedisState(t, client, room)
	})

	t.Run("error when updating non existent room", func(t *testing.T) {
		room := &entities.GameRoom{
			ID:         "room-3",
			Scheduler:  "game",
			Status:     entities.GameStatusOccupied,
			LastPingAt: lastPing,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		requireErrorKind(t, statestorage.ErrorRoomNotFound, storage.UpdateRoom(ctx, room))
	})
}

func TestRedisStateStorage_DeleteRoom(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection(t)
	storage := NewRedisStateStorage(client)

	t.Run("game room exists", func(t *testing.T) {
		room := &entities.GameRoom{
			ID:         "room-1",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: lastPing,
		}

		require.NoError(t, storage.CreateRoom(ctx, room))
		require.NoError(t, storage.RemoveRoom(ctx, room.Scheduler, room.ID))
		assertRedisStateNonExistent(t, client, room)
	})

	t.Run("game room nonexistent", func(t *testing.T) {
		requireErrorKind(t, statestorage.ErrorRoomNotFound, storage.RemoveRoom(ctx, "game", "room-2"))
	})
}

func TestRedisStateStorage_GetRoom(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection(t)
	storage := NewRedisStateStorage(client)

	t.Run("game room without metadata", func(t *testing.T) {
		expectedRoom := &entities.GameRoom{
			ID:         "room-1",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: lastPing,
		}

		require.NoError(t, storage.CreateRoom(ctx, expectedRoom))

		actualRoom, err := storage.GetRoom(ctx, expectedRoom.Scheduler, expectedRoom.ID)
		require.NoError(t, err)
		require.Equal(t, expectedRoom, actualRoom)
	})

	t.Run("game room with metadata", func(t *testing.T) {
		expectedRoom := &entities.GameRoom{
			ID:         "room-2",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: lastPing,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		require.NoError(t, storage.CreateRoom(ctx, expectedRoom))

		actualRoom, err := storage.GetRoom(ctx, expectedRoom.Scheduler, expectedRoom.ID)
		require.NoError(t, err)
		require.Equal(t, expectedRoom, actualRoom)
	})

	t.Run("error when getting non existent room", func(t *testing.T) {
		_, err := storage.GetRoom(ctx, "game", "room-3")
		requireErrorKind(t, statestorage.ErrorRoomNotFound, err)
	})
}

func TestRedisStateStorage_SetRoomStatus(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection(t)
	storage := NewRedisStateStorage(client)

	room := &entities.GameRoom{
		ID:         "room-1",
		Scheduler:  "game",
		Status:     entities.GameStatusReady,
		LastPingAt: lastPing,
	}

	require.NoError(t, storage.CreateRoom(ctx, room))
	require.NoError(t, storage.SetRoomStatus(ctx, room.Scheduler, room.ID, entities.GameStatusOccupied))
	room.Status = entities.GameStatusOccupied
	assertRedisState(t, client, room)
}

func TestRedisStateStorage_GetAllRoomIDs(t *testing.T) {
	ctx := context.Background()
	client := getRedisConnection(t)
	storage := NewRedisStateStorage(client)

	rooms := []*entities.GameRoom{
		{
			ID:         "room-1",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: lastPing,
		},
		{
			ID:         "room-2",
			Scheduler:  "game",
			Status:     entities.GameStatusOccupied,
			LastPingAt: lastPing,
		},
		{
			ID:         "room-3",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: lastPing,
		},
		{
			ID:         "room-4",
			Scheduler:  "game",
			Status:     entities.GameStatusPending,
			LastPingAt: lastPing,
		},
		{
			ID:         "room-5",
			Scheduler:  "game",
			Status:     entities.GameStatusTerminating,
			LastPingAt: lastPing,
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
	client := getRedisConnection(t)
	storage := NewRedisStateStorage(client)

	rooms := []*entities.GameRoom{
		{
			ID:         "room-1",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: time.Unix(1, 0),
		},
		{
			ID:         "room-2",
			Scheduler:  "game",
			Status:     entities.GameStatusOccupied,
			LastPingAt: time.Unix(2, 0),
		},
		{
			ID:         "room-3",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: time.Unix(3, 0),
		},
		{
			ID:         "room-4",
			Scheduler:  "game",
			Status:     entities.GameStatusPending,
			LastPingAt: time.Unix(4, 0),
		},
		{
			ID:         "room-5",
			Scheduler:  "game",
			Status:     entities.GameStatusTerminating,
			LastPingAt: time.Unix(5, 0),
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
	client := getRedisConnection(t)
	storage := NewRedisStateStorage(client)

	rooms := []*entities.GameRoom{
		{
			ID:         "room-1",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: time.Unix(1, 0),
		},
		{
			ID:         "room-2",
			Scheduler:  "game",
			Status:     entities.GameStatusOccupied,
			LastPingAt: time.Unix(2, 0),
		},
		{
			ID:         "room-3",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: time.Unix(3, 0),
		},
		{
			ID:         "room-4",
			Scheduler:  "game",
			Status:     entities.GameStatusPending,
			LastPingAt: time.Unix(4, 0),
		},
		{
			ID:         "room-5",
			Scheduler:  "game",
			Status:     entities.GameStatusTerminating,
			LastPingAt: time.Unix(5, 0),
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
	client := getRedisConnection(t)
	storage := NewRedisStateStorage(client)

	rooms := []*entities.GameRoom{
		{
			ID:         "room-1",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: time.Unix(1, 0),
		},
		{
			ID:         "room-2",
			Scheduler:  "game",
			Status:     entities.GameStatusOccupied,
			LastPingAt: time.Unix(2, 0),
		},
		{
			ID:         "room-3",
			Scheduler:  "game",
			Status:     entities.GameStatusReady,
			LastPingAt: time.Unix(3, 0),
		},
		{
			ID:         "room-4",
			Scheduler:  "game",
			Status:     entities.GameStatusPending,
			LastPingAt: time.Unix(4, 0),
		},
		{
			ID:         "room-5",
			Scheduler:  "game",
			Status:     entities.GameStatusTerminating,
			LastPingAt: time.Unix(5, 0),
		},
	}

	for _, room := range rooms {
		require.NoError(t, storage.CreateRoom(ctx, room))
	}

	roomCountByStatus := map[entities.GameRoomStatus]int{
		entities.GameStatusPending:     1,
		entities.GameStatusUnready:     0,
		entities.GameStatusReady:       2,
		entities.GameStatusOccupied:    1,
		entities.GameStatusTerminating: 1,
		entities.GameStatusError:       0,
	}

	for status, expectedCount := range roomCountByStatus {
		roomCount, err := storage.GetRoomCountByStatus(ctx, "game", status)
		require.NoError(t, err)
		require.Equal(t, expectedCount, roomCount)
	}
}
