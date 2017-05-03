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

// RoomsStatusCount is the struct that defines the rooms status status count
type RoomsStatusCount struct {
	Creating    int
	Occupied    int
	Ready       int
	Terminating int
}

// Total returns the total number of rooms
func (c *RoomsStatusCount) Total() int {
	return c.Creating + c.Occupied + c.Ready + c.Terminating
}

// NewRoom is the room constructor
func NewRoom(id, schedulerName string) *Room {
	return &Room{
		ID:            id,
		SchedulerName: schedulerName,
		Status:        "creating",
		LastPingAt:    0,
	}
}

// GetRoomRedisKey gets the key that will keep the room state in redis
func (r *Room) GetRoomRedisKey() string {
	return fmt.Sprintf("scheduler:%s:rooms:%s", r.SchedulerName, r.ID)
}

// GetRoomStatusSetRedisKey gets the key for the set that will keep rooms in a determined state in redis
func GetRoomStatusSetRedisKey(schedulerName, status string) string {
	return fmt.Sprintf("scheduler:%s:status:%s", schedulerName, status)
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
	pipe.SRem(GetRoomStatusSetRedisKey(r.SchedulerName, "terminating"), r.GetRoomRedisKey())
	pipe.Del(r.GetRoomRedisKey())
	_, err := pipe.Exec()
	return err
}

// SetStatus updates the status of a given room in the database
func (r *Room) SetStatus(redisClient interfaces.RedisClient, lastStatus string, status string) error {
	r.Status = status
	if status == "terminated" {
		return r.remove(redisClient)
	}
	pipe := redisClient.TxPipeline()
	pipe.HMSet(r.GetRoomRedisKey(), map[string]interface{}{
		"status": r.Status,
	})
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
		"lastPing": time.Now(),
		"status":   status,
	})
	return s.Err()
}

// GetRoomsCountByStatus returns the count of rooms for each status
func GetRoomsCountByStatus(redisClient interfaces.RedisClient, schedulerName string) (*RoomsStatusCount, error) {
	pipe := redisClient.TxPipeline()
	sCreating := pipe.SCard(GetRoomStatusSetRedisKey(schedulerName, "creating"))
	sReady := pipe.SCard(GetRoomStatusSetRedisKey(schedulerName, "ready"))
	sOccupied := pipe.SCard(GetRoomStatusSetRedisKey(schedulerName, "occupied"))
	sTerminating := pipe.SCard(GetRoomStatusSetRedisKey(schedulerName, "terminating"))
	_, err := pipe.Exec()
	if err != nil {
		return nil, err
	}
	countByStatus := &RoomsStatusCount{}
	countByStatus.Creating = int(sCreating.Val())
	countByStatus.Ready = int(sReady.Val())
	countByStatus.Occupied = int(sOccupied.Val())
	countByStatus.Terminating = int(sTerminating.Val())
	return countByStatus, nil
}
