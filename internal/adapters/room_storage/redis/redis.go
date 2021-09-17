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

	"github.com/go-redis/redis"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
)

const maxRoomStatusEvents = 100

type redisStateStorage struct {
	client *redis.Client
}

var _ ports.RoomStorage = (*redisStateStorage)(nil)

func NewRedisStateStorage(client *redis.Client) *redisStateStorage {
	return &redisStateStorage{client: client}
}

func (r redisStateStorage) GetRoom(ctx context.Context, scheduler, roomID string) (*game_room.GameRoom, error) {
	room := &game_room.GameRoom{
		ID:          roomID,
		SchedulerID: scheduler,
	}

	p := r.client.WithContext(ctx).Pipeline()
	metadataCmd := p.Get(getRoomRedisKey(room.SchedulerID, room.ID))
	statusCmd := p.ZScore(getRoomStatusSetRedisKey(room.SchedulerID), room.ID)
	pingCmd := p.ZScore(getRoomPingRedisKey(room.SchedulerID), room.ID)
	_, err := p.Exec()
	if err != nil {
		if err == redis.Nil {
			return nil, errors.NewErrNotFound("room %s not found in scheduler %s", roomID, scheduler)
		}
		return nil, errors.NewErrUnexpected("error when getting room %s on redis", roomID).WithError(err)
	}

	room.Status = game_room.GameRoomStatus(statusCmd.Val())
	room.LastPingAt = time.Unix(int64(pingCmd.Val()), 0)
	err = json.NewDecoder(strings.NewReader(metadataCmd.Val())).Decode(&room.Metadata)
	if err != nil {
		return nil, errors.NewErrEncoding("error unmarshalling room %s json", roomID).WithError(err)
	}

	return room, nil
}

func (r *redisStateStorage) CreateRoom(ctx context.Context, room *game_room.GameRoom) error {
	metadataJson, err := json.Marshal(room.Metadata)
	if err != nil {
		return err
	}

	p := r.client.WithContext(ctx).TxPipeline()
	roomCmd := p.SetNX(getRoomRedisKey(room.SchedulerID, room.ID), metadataJson, 0)
	statusCmd := p.ZAddNX(getRoomStatusSetRedisKey(room.SchedulerID), redis.Z{
		Member: room.ID,
		Score:  float64(room.Status),
	})
	pingCmd := p.ZAddNX(getRoomPingRedisKey(room.SchedulerID), redis.Z{
		Member: room.ID,
		Score:  float64(room.LastPingAt.Unix()),
	})

	_, err = p.Exec()
	if err != nil {
		return errors.NewErrUnexpected("error storing room %s on redis", room.ID).WithError(err)
	}

	if !roomCmd.Val() || statusCmd.Val() < 1 || pingCmd.Val() < 1 {
		return errors.NewErrAlreadyExists("room %s already exists in scheduler %s", room.ID, room.SchedulerID)
	}

	return nil
}

func (r *redisStateStorage) UpdateRoom(ctx context.Context, room *game_room.GameRoom) error {
	metadataJson, err := json.Marshal(room.Metadata)
	if err != nil {
		return err
	}

	p := r.client.WithContext(ctx).TxPipeline()
	roomCmd := p.SetXX(getRoomRedisKey(room.SchedulerID, room.ID), metadataJson, 0)
	p.ZAddXXCh(getRoomStatusSetRedisKey(room.SchedulerID), redis.Z{
		Member: room.ID,
		Score:  float64(room.Status),
	})
	p.ZAddXXCh(getRoomPingRedisKey(room.SchedulerID), redis.Z{
		Member: room.ID,
		Score:  float64(room.LastPingAt.Unix()),
	})

	encodedEvent, err := encodeStatusEvent(&game_room.StatusEvent{RoomID: room.ID, SchedulerName: room.SchedulerID, Status: room.Status})
	if err != nil {
		return errors.NewErrEncoding("failed to encode status event").WithError(err)
	}

	p.Publish(getRoomStatusUpdateChannel(room.SchedulerID, room.ID), encodedEvent)

	_, err = p.Exec()
	if err != nil {
		return errors.NewErrUnexpected("error updating room %s on redis", room.ID).WithError(err)
	}

	if !roomCmd.Val() {
		return errors.NewErrNotFound("room %s not found in scheduler %s", room.ID, room.SchedulerID)
	}

	return nil
}

func (r *redisStateStorage) DeleteRoom(ctx context.Context, scheduler, roomID string) error {
	p := r.client.WithContext(ctx).TxPipeline()
	p.Del(getRoomRedisKey(scheduler, roomID))
	p.ZRem(getRoomStatusSetRedisKey(scheduler), roomID)
	p.ZRem(getRoomPingRedisKey(scheduler), roomID)
	cmders, err := p.Exec()
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

func (r *redisStateStorage) GetAllRoomIDs(ctx context.Context, scheduler string) ([]string, error) {
	rooms, err := r.client.WithContext(ctx).ZRange(getRoomStatusSetRedisKey(scheduler), 0, -1).Result()
	if err != nil {
		return nil, errors.NewErrUnexpected("error listing rooms on redis").WithError(err)
	}
	return rooms, nil
}

func (r *redisStateStorage) GetRoomIDsByLastPing(ctx context.Context, scheduler string, threshold time.Time) ([]string, error) {
	rooms, err := r.client.WithContext(ctx).ZRangeByScore(getRoomPingRedisKey(scheduler), redis.ZRangeBy{
		Min: "-inf",
		Max: strconv.FormatInt(threshold.Unix(), 10),
	}).Result()
	if err != nil {
		return nil, errors.NewErrUnexpected("error listing rooms on redis").WithError(err)
	}
	return rooms, nil
}

func (r *redisStateStorage) GetRoomCount(ctx context.Context, scheduler string) (int, error) {
	client := r.client.WithContext(ctx)
	count, err := client.ZCard(getRoomStatusSetRedisKey(scheduler)).Result()
	if err != nil {
		return 0, errors.NewErrUnexpected("error counting rooms on redis").WithError(err)
	}
	return int(count), nil
}

func (r *redisStateStorage) GetRoomCountByStatus(ctx context.Context, scheduler string, status game_room.GameRoomStatus) (int, error) {
	client := r.client.WithContext(ctx)
	statusIntStr := fmt.Sprint(int(status))
	count, err := client.ZCount(getRoomStatusSetRedisKey(scheduler), statusIntStr, statusIntStr).Result()
	if err != nil {
		return 0, errors.NewErrUnexpected("error counting rooms on redis").WithError(err)
	}
	return int(count), nil
}

func (r *redisStateStorage) WatchRoomStatus(ctx context.Context, room *game_room.GameRoom) (ports.RoomStorageStatusWatcher, error) {
	sub := r.client.WithContext(ctx).Subscribe(getRoomStatusUpdateChannel(room.SchedulerID, room.ID))

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