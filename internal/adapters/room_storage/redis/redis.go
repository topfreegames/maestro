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

package redis

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/topfreegames/maestro/internal/adapters/metrics"

	"github.com/go-redis/redis/v8"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
)

const (
	maxRoomStatusEvents = 100

	// Room hash keys.
	metadataKey         = "metadata"
	pingStatusKey       = "ping_status"
	versionKey          = "version"
	isValidationRoomKey = "is_validation_room"
	createdAtKey        = "created_at"
)

type redisStateStorage struct {
	client *redis.Client
}

var _ ports.RoomStorage = (*redisStateStorage)(nil)

const roomStorageMetricLabel = "room-storage"

func NewRedisStateStorage(client *redis.Client) *redisStateStorage {
	return &redisStateStorage{client: client}
}

func (r redisStateStorage) GetRoom(ctx context.Context, scheduler, roomID string) (room *game_room.GameRoom, err error) {
	room = &game_room.GameRoom{
		ID:          roomID,
		SchedulerID: scheduler,
	}

	p := r.client.Pipeline()
	roomHashCmd := r.client.HGetAll(ctx, getRoomRedisKey(room.SchedulerID, room.ID))
	statusCmd := p.ZScore(ctx, getRoomStatusSetRedisKey(room.SchedulerID), room.ID)
	pingCmd := p.ZScore(ctx, getRoomPingRedisKey(room.SchedulerID), room.ID)
	metrics.RunWithMetrics(roomStorageMetricLabel, func() error {
		_, err = p.Exec(ctx)
		return err
	})
	if err != nil {
		if err == redis.Nil {
			return nil, errors.NewErrNotFound("room %s not found in scheduler %s", roomID, scheduler)
		}
		return nil, errors.NewErrUnexpected("error when getting room %s on redis", roomID).WithError(err)
	}

	room.Status = game_room.GameRoomStatus(statusCmd.Val())
	room.LastPingAt = time.Unix(int64(pingCmd.Val()), 0)
	err = json.NewDecoder(strings.NewReader(roomHashCmd.Val()[metadataKey])).Decode(&room.Metadata)
	if err != nil {
		return nil, errors.NewErrEncoding("error unmarshalling room %s json", roomID).WithError(err)
	}

	pingStatusInt, err := strconv.Atoi(roomHashCmd.Val()[pingStatusKey])
	if err != nil {
		return nil, errors.NewErrEncoding("failed to parse operation status").WithError(err)
	}

	boolIsValidationRoom, _ := strconv.ParseBool(roomHashCmd.Val()[isValidationRoomKey])
	room.IsValidationRoom = boolIsValidationRoom

	room.PingStatus = game_room.GameRoomPingStatus(pingStatusInt)
	room.Version = roomHashCmd.Val()[versionKey]
	createdAt, err := strconv.ParseInt(roomHashCmd.Val()[createdAtKey], 10, 64)
	if err != nil {
		return nil, errors.NewErrEncoding("error parsing room %s createdAtKey", roomID).WithError(err)
	}

	room.CreatedAt = time.Unix(createdAt, 0)
	return room, nil
}

func (r *redisStateStorage) CreateRoom(ctx context.Context, room *game_room.GameRoom) error {
	metadataJson, err := json.Marshal(room.Metadata)
	if err != nil {
		return err
	}

	p := r.client.TxPipeline()
	p.HSet(ctx, getRoomRedisKey(room.SchedulerID, room.ID), map[string]interface{}{
		versionKey:          room.Version,
		metadataKey:         metadataJson,
		pingStatusKey:       strconv.Itoa(int(room.PingStatus)),
		isValidationRoomKey: room.IsValidationRoom,
		createdAtKey:        time.Now().Unix(),
	})

	statusCmd := p.ZAddNX(ctx, getRoomStatusSetRedisKey(room.SchedulerID), &redis.Z{
		Member: room.ID,
		Score:  float64(room.Status),
	})

	pingCmd := p.ZAddNX(ctx, getRoomPingRedisKey(room.SchedulerID), &redis.Z{
		Member: room.ID,
		Score:  float64(room.LastPingAt.Unix()),
	})

	metrics.RunWithMetrics(roomStorageMetricLabel, func() error {
		_, err = p.Exec(ctx)
		return err
	})
	if err != nil {
		return errors.NewErrUnexpected("error storing room %s on redis", room.ID).WithError(err)
	}

	if statusCmd.Val() < 1 || pingCmd.Val() < 1 {
		return errors.NewErrAlreadyExists("room %s already exists in scheduler %s", room.ID, room.SchedulerID)
	}

	return nil
}

// UpdateRoom update all GameRoom fields, expect `Status`. For updating the game
// room status, check UpdateRoomStatus function.
//
// TODO(gabrielcorado): add a mechanism to know if the room doesn't exists. we
// could do some optimistic lock.
func (r *redisStateStorage) UpdateRoom(ctx context.Context, room *game_room.GameRoom) error {
	metadataJson, err := json.Marshal(room.Metadata)
	if err != nil {
		return err
	}

	p := r.client.TxPipeline()
	p.HSet(ctx, getRoomRedisKey(room.SchedulerID, room.ID), map[string]interface{}{
		metadataKey:   metadataJson,
		pingStatusKey: strconv.Itoa(int(room.PingStatus)),
	})

	p.ZAddXXCh(ctx, getRoomPingRedisKey(room.SchedulerID), &redis.Z{
		Member: room.ID,
		Score:  float64(room.LastPingAt.Unix()),
	})

	metrics.RunWithMetrics(roomStorageMetricLabel, func() error {
		_, err = p.Exec(ctx)
		return err
	})
	if err != nil {
		return errors.NewErrUnexpected("error updating room %s on redis", room.ID).WithError(err)
	}

	return nil
}

func (r *redisStateStorage) DeleteRoom(ctx context.Context, scheduler, roomID string) (err error) {
	var cmders []redis.Cmder
	p := r.client.TxPipeline()
	p.Del(ctx, getRoomRedisKey(scheduler, roomID))
	p.ZRem(ctx, getRoomStatusSetRedisKey(scheduler), roomID)
	p.ZRem(ctx, getRoomPingRedisKey(scheduler), roomID)
	metrics.RunWithMetrics(roomStorageMetricLabel, func() error {
		cmders, err = p.Exec(ctx)
		return err
	})
	if err != nil {
		return errors.NewErrUnexpected("error removing room %s from redis", roomID).WithError(err)
	}
	for _, cmder := range cmders {
		cmd := cmder.(*redis.IntCmd)
		if cmd.Val() == 0 {
			return errors.NewErrNotFound("room %s not found in scheduler %s", roomID, scheduler)
		}
	}
	return nil
}

func (r *redisStateStorage) GetAllRoomIDs(ctx context.Context, scheduler string) (rooms []string, err error) {
	metrics.RunWithMetrics(roomStorageMetricLabel, func() error {
		rooms, err = r.client.ZRange(ctx, getRoomStatusSetRedisKey(scheduler), 0, -1).Result()
		return err
	})

	if err != nil {
		return nil, errors.NewErrUnexpected("error listing rooms on redis").WithError(err)
	}
	return rooms, nil
}

func (r *redisStateStorage) GetRoomIDsByStatus(ctx context.Context, scheduler string, status game_room.GameRoomStatus) (rooms []string, err error) {
	statusIntStr := fmt.Sprint(int(status))
	metrics.RunWithMetrics(roomStorageMetricLabel, func() error {
		rooms, err = r.client.ZRangeByScore(ctx, getRoomStatusSetRedisKey(scheduler), &redis.ZRangeBy{
			Min: statusIntStr,
			Max: statusIntStr,
		}).Result()
		return err
	})

	if err != nil {
		return nil, errors.NewErrUnexpected("error listing rooms on redis").WithError(err)
	}
	return rooms, nil
}

func (r *redisStateStorage) GetRoomIDsByLastPing(ctx context.Context, scheduler string, threshold time.Time) (rooms []string, err error) {
	metrics.RunWithMetrics(roomStorageMetricLabel, func() error {
		rooms, err = r.client.ZRangeByScore(ctx, getRoomPingRedisKey(scheduler), &redis.ZRangeBy{
			Min: "-inf",
			Max: strconv.FormatInt(threshold.Unix(), 10),
		}).Result()
		return err
	})

	if err != nil {
		return nil, errors.NewErrUnexpected("error listing rooms on redis").WithError(err)
	}
	return rooms, nil
}

func (r *redisStateStorage) GetRoomCount(ctx context.Context, scheduler string) (count int, err error) {
	client := r.client
	var resultCount int64
	metrics.RunWithMetrics(roomStorageMetricLabel, func() error {
		resultCount, err = client.ZCard(ctx, getRoomStatusSetRedisKey(scheduler)).Result()
		return err

	})
	if err != nil {
		return 0, errors.NewErrUnexpected("error counting rooms on redis").WithError(err)
	}
	count = int(resultCount)
	return count, nil
}

func (r *redisStateStorage) GetRoomCountByStatus(ctx context.Context, scheduler string, status game_room.GameRoomStatus) (count int, err error) {
	client := r.client
	statusIntStr := fmt.Sprint(int(status))
	var resultCount int64
	metrics.RunWithMetrics(roomStorageMetricLabel, func() error {
		resultCount, err = client.ZCount(ctx, getRoomStatusSetRedisKey(scheduler), statusIntStr, statusIntStr).Result()

		return err
	})
	if err != nil {
		return 0, errors.NewErrUnexpected("error counting rooms on redis").WithError(err)
	}
	return int(resultCount), nil
}

func (r *redisStateStorage) UpdateRoomStatus(ctx context.Context, scheduler, roomId string, status game_room.GameRoomStatus) error {
	var statusCmd *redis.IntCmd
	metrics.RunWithMetrics(roomStorageMetricLabel, func() error {
		statusCmd = r.client.ZAddXXCh(ctx, getRoomStatusSetRedisKey(scheduler), &redis.Z{
			Member: roomId,
			Score:  float64(status),
		})
		return statusCmd.Err()
	})

	if statusCmd.Err() != nil {
		return errors.NewErrUnexpected("error updating room %s status on redis", roomId).WithError(statusCmd.Err())
	}

	if statusCmd.Val() < 1 {
		return errors.NewErrNotFound("room %s not found in scheduler %s", roomId, scheduler)
	}

	encodedEvent, err := encodeStatusEvent(&game_room.StatusEvent{RoomID: roomId, SchedulerName: scheduler, Status: status})
	if err != nil {
		return errors.NewErrEncoding("failed to encode status event").WithError(err)
	}

	var publishCmd *redis.IntCmd
	metrics.RunWithMetrics(roomStorageMetricLabel, func() error {
		publishCmd = r.client.Publish(ctx, getRoomStatusUpdateChannel(scheduler, roomId), encodedEvent)
		return publishCmd.Err()
	})
	if publishCmd.Err() != nil {
		return errors.NewErrUnexpected("error sending update room %s status event on redis", roomId).WithError(publishCmd.Err())
	}

	return nil
}

func (r *redisStateStorage) WatchRoomStatus(ctx context.Context, room *game_room.GameRoom) (ports.RoomStorageStatusWatcher, error) {
	var sub *redis.PubSub
	metrics.RunWithMetrics(roomStorageMetricLabel, func() error {
		sub = r.client.Subscribe(ctx, getRoomStatusUpdateChannel(room.SchedulerID, room.ID))
		return nil
	})

	watcherCtx, cancelWatcherCtx := context.WithCancel(ctx)
	watcher := &redisStatusWatcher{
		resultChan: make(chan game_room.StatusEvent, maxRoomStatusEvents),
		cancelFn:   cancelWatcherCtx,
	}

	go func() {
		defer sub.Close()

		for {
			select {
			case msg := <-sub.Channel():
				event, _ := decodeStatusEvent([]byte(msg.Payload))
				watcher.resultChan <- *event
			case <-watcherCtx.Done():
				close(watcher.resultChan)
				return
			}
		}
	}()

	return watcher, nil
}

type redisStatusWatcher struct {
	resultChan chan game_room.StatusEvent
	cancelFn   context.CancelFunc
	stopOnce   sync.Once
}

func (sw *redisStatusWatcher) ResultChan() chan game_room.StatusEvent {
	return sw.resultChan
}

func (sw *redisStatusWatcher) Stop() {
	sw.stopOnce.Do(func() {
		sw.cancelFn()
	})
}

func encodeStatusEvent(event *game_room.StatusEvent) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(event)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decodeStatusEvent(encoded []byte) (*game_room.StatusEvent, error) {
	dec := gob.NewDecoder(bytes.NewBuffer(encoded))

	var result game_room.StatusEvent
	err := dec.Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func getRoomRedisKey(scheduler, roomID string) string {
	return fmt.Sprintf("scheduler:%s:rooms:%s", scheduler, roomID)
}

func getRoomStatusSetRedisKey(scheduler string) string {
	return fmt.Sprintf("scheduler:%s:status", scheduler)
}

func getRoomPingRedisKey(scheduler string) string {
	return fmt.Sprintf("scheduler:%s:ping", scheduler)
}

func getRoomStatusUpdateChannel(scheduler, roomID string) string {
	return fmt.Sprintf("scheduler:%s:rooms:%s:updatechan", scheduler, roomID)
}
