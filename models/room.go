// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"fmt"
	"time"

	"github.com/topfreegames/extensions/redis/interfaces"
)

// Room is the struct that defines a room in maestro
type Room struct {
	ID            string
	SchedulerName string
	Status        string
	LastPingAt    int64
}

// NewRoom is the room constructor
func NewRoom(id, schedulerName string) *Room {
	return &Room{
		ID:            id,
		SchedulerName: schedulerName,
		Status:        StatusCreating,
		LastPingAt:    0,
	}
}

// GetRoomRedisKey gets the key that will keep the room state in redis
func (r *Room) GetRoomRedisKey() string {
	return fmt.Sprintf("scheduler:%s:rooms:%s", r.SchedulerName, r.ID)
}

// Create creates a room in and update redis
func (r *Room) Create(redisClient interfaces.RedisClient) error {
	pipe := redisClient.TxPipeline()
	pipe.HMSet(r.GetRoomRedisKey(), map[string]interface{}{
		"status":   r.Status,
		"lastPing": r.LastPingAt,
	})
	pipe.SAdd(GetRoomStatusSetRedisKey(r.SchedulerName, r.Status), r.GetRoomRedisKey())
	_, err := pipe.Exec()
	return err
}

func (r *Room) remove(redisClient interfaces.RedisClient) error {
	pipe := redisClient.TxPipeline()
	pipe.SRem(GetRoomStatusSetRedisKey(r.SchedulerName, StatusTerminating), r.GetRoomRedisKey())
	pipe.Del(r.GetRoomRedisKey())
	_, err := pipe.Exec()
	return err
}

// SetStatus updates the status of a given room in the database
func (r *Room) SetStatus(redisClient interfaces.RedisClient, lastStatus string, status string) error {
	r.Status = status
	if status == StatusTerminated {
		return r.remove(redisClient)
	}
	pipe := redisClient.TxPipeline()
	pipe.HMSet(r.GetRoomRedisKey(), map[string]interface{}{
		"status": r.Status,
	})
	// TODO: search for roomrediskey in all status in case the client desynchronized
	if len(lastStatus) > 0 {
		pipe.SRem(GetRoomStatusSetRedisKey(r.SchedulerName, lastStatus), r.GetRoomRedisKey())
	}
	pipe.SAdd(GetRoomStatusSetRedisKey(r.SchedulerName, r.Status), r.GetRoomRedisKey())
	_, err := pipe.Exec()
	return err
}

// Ping updates the last_ping_at field of a given room in the database
func (r *Room) Ping(redisClient interfaces.RedisClient, status string) error {
	// TODO ver se estado é coerente?
	// TODO talvez seja melhor um script lua, ve o estado atual, remove se n bater e adiciona no novo
	s := redisClient.HMSet(r.GetRoomRedisKey(), map[string]interface{}{
		"lastPing": time.Now().Unix(),
		"status":   status,
	})
	return s.Err()
}
