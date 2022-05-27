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

//go:build integration
// +build integration

package redis

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/test"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

var lastPing = time.Unix(time.Now().Unix(), 0)
var redisAddress string

func TestMain(m *testing.M) {
	var code int
	test.WithRedisContainer(func(redisContainerAddress string) {
		redisAddress = redisContainerAddress
		code = m.Run()
	})
	os.Exit(code)
}

func assertRedisState(t *testing.T, client *redis.Client, room *game_room.GameRoom) {
	roomHash := client.HGetAll(context.Background(), getRoomRedisKey(room.SchedulerID, room.ID))
	require.NoError(t, roomHash.Err())

	var actualMetadata map[string]interface{}
	require.NoError(t, json.NewDecoder(strings.NewReader(roomHash.Val()[metadataKey])).Decode(&actualMetadata))
	require.Equal(t, room.Metadata, actualMetadata)
	require.Equal(t, room.Version, roomHash.Val()[versionKey])

	isValidationRoom, err := strconv.ParseBool(roomHash.Val()[isValidationRoomKey])
	require.NoError(t, err)
	require.Equal(t, isValidationRoom, room.IsValidationRoom)

	createdAt, err := strconv.ParseInt(roomHash.Val()[createdAtKey], 10, 64)
	require.NoError(t, err)
	createdAtTime := time.Unix(createdAt, 0)
	now := time.Now()
	require.Equal(t, now.Year(), createdAtTime.Year())
	require.Equal(t, now.Month(), createdAtTime.Month())
	require.Equal(t, now.Day(), createdAtTime.Day())

	pingStatusInt, err := strconv.Atoi(roomHash.Val()[pingStatusKey])
	require.NoError(t, err)
	require.Equal(t, room.PingStatus, game_room.GameRoomPingStatus(pingStatusInt))

	statusCmd := client.ZScore(context.Background(), getRoomStatusSetRedisKey(room.SchedulerID), room.ID)
	require.NoError(t, statusCmd.Err())
	require.Equal(t, room.Status, game_room.GameRoomStatus(statusCmd.Val()))

	pingCmd := client.ZScore(context.Background(), getRoomPingRedisKey(room.SchedulerID), room.ID)
	require.NoError(t, pingCmd.Err())
	require.Equal(t, float64(room.LastPingAt.Unix()), pingCmd.Val())
}

func assertUpdateStatusEventPublished(t *testing.T, sub *redis.PubSub, room *game_room.GameRoom) {
	require.Eventually(t, func() bool {
		select {
		case msg := <-sub.Channel():
			event, err := decodeStatusEvent([]byte(msg.Payload))
			require.NoError(t, err)
			require.Equal(t, room.ID, event.RoomID)
			require.Equal(t, room.Status, event.Status)
			require.Equal(t, room.SchedulerID, event.SchedulerName)
			return true
		default:
			return false
		}
	}, time.Second, 100*time.Millisecond)
}

func assertRedisStateNonExistent(t *testing.T, client *redis.Client, room *game_room.GameRoom) {
	metadataCmd := client.Get(context.Background(), getRoomRedisKey(room.SchedulerID, room.ID))
	require.Error(t, metadataCmd.Err())

	statusCmd := client.ZScore(context.Background(), getRoomStatusSetRedisKey(room.SchedulerID), room.ID)
	require.Error(t, statusCmd.Err())

	pingCmd := client.ZScore(context.Background(), getRoomPingRedisKey(room.SchedulerID), room.ID)
	require.Error(t, pingCmd.Err())
}

func requireErrorKind(t *testing.T, expected error, err error) {
	require.Error(t, err)
	require.ErrorIs(t, expected, err)
}

func TestRedisStateStorage_CreateRoom(t *testing.T) {
	ctx := context.Background()
	client := test.GetRedisConnection(t, redisAddress)
	storage := NewRedisStateStorage(client)

	t.Run("game room without metadata", func(t *testing.T) {
		room := &game_room.GameRoom{
			ID:               "room-1",
			SchedulerID:      "game",
			Version:          "1.0",
			Status:           game_room.GameStatusReady,
			LastPingAt:       lastPing,
			IsValidationRoom: false,
		}
		require.NoError(t, storage.CreateRoom(ctx, room))
		assertRedisState(t, client, room)
	})

	t.Run("game room with metadata", func(t *testing.T) {
		room := &game_room.GameRoom{
			ID:               "room-2",
			SchedulerID:      "game",
			Version:          "1.0",
			Status:           game_room.GameStatusReady,
			LastPingAt:       lastPing,
			IsValidationRoom: false,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		require.NoError(t, storage.CreateRoom(ctx, room))
		assertRedisState(t, client, room)
	})

	t.Run("error when creating existing room", func(t *testing.T) {
		firstRoom := &game_room.GameRoom{
			ID:               "room-3",
			SchedulerID:      "game",
			Version:          "1.0",
			Status:           game_room.GameStatusReady,
			LastPingAt:       lastPing,
			IsValidationRoom: false,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		require.NoError(t, storage.CreateRoom(ctx, firstRoom))
		assertRedisState(t, client, firstRoom)

		secondRoom := &game_room.GameRoom{
			ID:               "room-3",
			SchedulerID:      "game",
			Version:          "1.0",
			Status:           game_room.GameStatusOccupied,
			LastPingAt:       lastPing,
			IsValidationRoom: false,
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
	client := test.GetRedisConnection(t, redisAddress)
	storage := NewRedisStateStorage(client)

	t.Run("game room without metadata", func(t *testing.T) {
		room := &game_room.GameRoom{
			ID:               "room-1",
			SchedulerID:      "game",
			Version:          "1.0",
			Status:           game_room.GameStatusReady,
			LastPingAt:       lastPing,
			IsValidationRoom: false,
		}

		require.NoError(t, storage.CreateRoom(ctx, room))

		require.NoError(t, storage.UpdateRoom(ctx, room))
		assertRedisState(t, client, room)
	})

	t.Run("game room with metadata", func(t *testing.T) {
		room := &game_room.GameRoom{
			ID:               "room-2",
			SchedulerID:      "game",
			Version:          "1.0",
			Status:           game_room.GameStatusReady,
			LastPingAt:       lastPing,
			IsValidationRoom: false,
			Metadata: map[string]interface{}{
				"region": "us",
			},
		}

		require.NoError(t, storage.CreateRoom(ctx, room))

		require.NoError(t, storage.UpdateRoom(ctx, room))
		assertRedisState(t, client, room)
	})
}

func TestRedisStateStorage_DeleteRoom(t *testing.T) {
	ctx := context.Background()
	client := test.GetRedisConnection(t, redisAddress)
	storage := NewRedisStateStorage(client)

	t.Run("game room exists", func(t *testing.T) {
		room := &game_room.GameRoom{
			ID:          "room-1",
			SchedulerID: "game",
			Version:     "1.0",
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
	client := test.GetRedisConnection(t, redisAddress)
	storage := NewRedisStateStorage(client)

	t.Run("game room without metadata", func(t *testing.T) {
		createdAt := time.Unix(time.Date(2020, 1, 1, 2, 2, 0, 0, time.UTC).Unix(), 0)
		expectedRoom := &game_room.GameRoom{
			ID:          "room-1",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
			CreatedAt:   createdAt,
		}
		metadataJson, _ := json.Marshal(expectedRoom.Metadata)

		p := client.TxPipeline()
		p.HSet(ctx, getRoomRedisKey(expectedRoom.SchedulerID, expectedRoom.ID), map[string]interface{}{
			versionKey:          expectedRoom.Version,
			metadataKey:         metadataJson,
			pingStatusKey:       strconv.Itoa(int(expectedRoom.PingStatus)),
			isValidationRoomKey: expectedRoom.IsValidationRoom,
			createdAtKey:        expectedRoom.CreatedAt.Unix(),
		})

		_ = p.ZAddNX(ctx, getRoomStatusSetRedisKey(expectedRoom.SchedulerID), &redis.Z{
			Member: expectedRoom.ID,
			Score:  float64(expectedRoom.Status),
		})

		_ = p.ZAddNX(ctx, getRoomPingRedisKey(expectedRoom.SchedulerID), &redis.Z{
			Member: expectedRoom.ID,
			Score:  float64(expectedRoom.LastPingAt.Unix()),
		})

		p.Exec(ctx)

		actualRoom, err := storage.GetRoom(ctx, expectedRoom.SchedulerID, expectedRoom.ID)
		require.NoError(t, err)
		require.Equal(t, expectedRoom, actualRoom)
	})

	t.Run("game room with metadata and validation flag true", func(t *testing.T) {
		createdAt := time.Unix(time.Date(2020, 1, 1, 2, 2, 0, 0, time.UTC).Unix(), 0)
		expectedRoom := &game_room.GameRoom{
			ID:          "room-2",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
			Metadata: map[string]interface{}{
				"region": "us",
			},
			IsValidationRoom: true,
			CreatedAt:        createdAt,
		}

		metadataJson, _ := json.Marshal(expectedRoom.Metadata)

		p := client.TxPipeline()
		p.HSet(ctx, getRoomRedisKey(expectedRoom.SchedulerID, expectedRoom.ID), map[string]interface{}{
			versionKey:          expectedRoom.Version,
			metadataKey:         metadataJson,
			pingStatusKey:       strconv.Itoa(int(expectedRoom.PingStatus)),
			isValidationRoomKey: expectedRoom.IsValidationRoom,
			createdAtKey:        expectedRoom.CreatedAt.Unix(),
		})

		_ = p.ZAddNX(ctx, getRoomStatusSetRedisKey(expectedRoom.SchedulerID), &redis.Z{
			Member: expectedRoom.ID,
			Score:  float64(expectedRoom.Status),
		})

		_ = p.ZAddNX(ctx, getRoomPingRedisKey(expectedRoom.SchedulerID), &redis.Z{
			Member: expectedRoom.ID,
			Score:  float64(expectedRoom.LastPingAt.Unix()),
		})
		p.Exec(ctx)

		actualRoom, err := storage.GetRoom(ctx, expectedRoom.SchedulerID, expectedRoom.ID)
		require.NoError(t, err)
		require.Equal(t, expectedRoom, actualRoom)
	})

	t.Run("error when getting non existent room", func(t *testing.T) {
		_, err := storage.GetRoom(ctx, "game", "room-3")
		requireErrorKind(t, errors.ErrNotFound, err)
	})
}

func TestRedisStateStorage_GetAllRoomIDs(t *testing.T) {
	ctx := context.Background()
	client := test.GetRedisConnection(t, redisAddress)
	storage := NewRedisStateStorage(client)

	rooms := []*game_room.GameRoom{
		{
			ID:          "room-1",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
		},
		{
			ID:          "room-2",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusOccupied,
			LastPingAt:  lastPing,
		},
		{
			ID:          "room-3",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
		},
		{
			ID:          "room-4",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusPending,
			LastPingAt:  lastPing,
		},
		{
			ID:          "room-5",
			SchedulerID: "game",
			Version:     "1.0",
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
	client := test.GetRedisConnection(t, redisAddress)
	storage := NewRedisStateStorage(client)

	rooms := []*game_room.GameRoom{
		{
			ID:          "room-1",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Unix(1, 0),
		},
		{
			ID:          "room-2",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusOccupied,
			LastPingAt:  time.Unix(2, 0),
		},
		{
			ID:          "room-3",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Unix(3, 0),
		},
		{
			ID:          "room-4",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusPending,
			LastPingAt:  time.Unix(4, 0),
		},
		{
			ID:          "room-5",
			SchedulerID: "game",
			Version:     "1.0",
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
	client := test.GetRedisConnection(t, redisAddress)
	storage := NewRedisStateStorage(client)

	rooms := []*game_room.GameRoom{
		{
			ID:          "room-1",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Unix(1, 0),
		},
		{
			ID:          "room-2",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusOccupied,
			LastPingAt:  time.Unix(2, 0),
		},
		{
			ID:          "room-3",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Unix(3, 0),
		},
		{
			ID:          "room-4",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusPending,
			LastPingAt:  time.Unix(4, 0),
		},
		{
			ID:          "room-5",
			SchedulerID: "game",
			Version:     "1.0",
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
	client := test.GetRedisConnection(t, redisAddress)
	storage := NewRedisStateStorage(client)

	rooms := []*game_room.GameRoom{
		{
			ID:          "room-1",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Unix(1, 0),
		},
		{
			ID:          "room-2",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusOccupied,
			LastPingAt:  time.Unix(2, 0),
		},
		{
			ID:          "room-3",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  time.Unix(3, 0),
		},
		{
			ID:          "room-4",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusPending,
			LastPingAt:  time.Unix(4, 0),
		},
		{
			ID:          "room-5",
			SchedulerID: "game",
			Version:     "1.0",
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
	client := test.GetRedisConnection(t, redisAddress)
	storage := NewRedisStateStorage(client)

	room := &game_room.GameRoom{
		ID:          "room-1",
		SchedulerID: "game",
		Version:     "1.0",
		Status:      game_room.GameStatusReady,
		LastPingAt:  lastPing,
	}
	expectedStatus := game_room.GameStatusOccupied

	watcher, err := storage.WatchRoomStatus(ctx, room)
	defer watcher.Stop()
	require.NoError(t, err)

	require.NoError(t, storage.CreateRoom(ctx, room))
	require.NoError(t, storage.UpdateRoomStatus(ctx, room.SchedulerID, room.ID, expectedStatus))

	require.Eventually(t, func() bool {
		select {
		case event := <-watcher.ResultChan():
			require.Equal(t, room.ID, event.RoomID)
			require.Equal(t, expectedStatus, event.Status)
			return true
		default:
		}

		return false
	}, 5*time.Second, 100*time.Millisecond)
}

func TestRedisStateStorage_UpdateRoomStatus(t *testing.T) {
	ctx := context.Background()
	client := test.GetRedisConnection(t, redisAddress)
	storage := NewRedisStateStorage(client)

	t.Run("game room updates room status", func(t *testing.T) {
		room := &game_room.GameRoom{
			ID:          "room-1",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
		}

		sub := client.Subscribe(context.Background(), getRoomStatusUpdateChannel(room.SchedulerID, room.ID))
		defer sub.Close()

		require.NoError(t, storage.CreateRoom(ctx, room))
		room.Status = game_room.GameStatusOccupied
		require.NoError(t, storage.UpdateRoomStatus(ctx, room.SchedulerID, room.ID, room.Status))
		assertRedisState(t, client, room)
		assertUpdateStatusEventPublished(t, sub, room)
	})

	t.Run("game room doesn't update room status when room doesn't exists", func(t *testing.T) {
		room := &game_room.GameRoom{
			ID:          "room-not-found",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
		}

		sub := client.Subscribe(context.Background(), getRoomStatusUpdateChannel(room.SchedulerID, room.ID))
		defer sub.Close()

		require.Error(t, storage.UpdateRoomStatus(ctx, room.SchedulerID, room.ID, room.Status))

		select {
		case <-sub.Channel():
			require.Fail(t, "received an update")
		case <-time.After(time.Second):
		}
	})
}

func TestRedisStateStorage_GetRoomIDsByStatus(t *testing.T) {
	ctx := context.Background()
	client := test.GetRedisConnection(t, redisAddress)
	storage := NewRedisStateStorage(client)

	rooms := []*game_room.GameRoom{
		{
			ID:          "room-1",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
		},
		{
			ID:          "room-2",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusOccupied,
			LastPingAt:  lastPing,
		},
		{
			ID:          "room-3",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusReady,
			LastPingAt:  lastPing,
		},
		{
			ID:          "room-4",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusPending,
			LastPingAt:  lastPing,
		},
		{
			ID:          "room-5",
			SchedulerID: "game",
			Version:     "1.0",
			Status:      game_room.GameStatusTerminating,
			LastPingAt:  lastPing,
		},
	}

	for _, room := range rooms {
		require.NoError(t, storage.CreateRoom(ctx, room))
	}

	readyRooms, err := storage.GetRoomIDsByStatus(ctx, "game", game_room.GameStatusReady)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"room-1", "room-3"}, readyRooms)

	occupiedRooms, err := storage.GetRoomIDsByStatus(ctx, "game", game_room.GameStatusOccupied)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"room-2"}, occupiedRooms)

	pendingRooms, err := storage.GetRoomIDsByStatus(ctx, "game", game_room.GameStatusPending)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"room-4"}, pendingRooms)

	terminatingRooms, err := storage.GetRoomIDsByStatus(ctx, "game", game_room.GameStatusTerminating)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"room-5"}, terminatingRooms)
}
