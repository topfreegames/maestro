// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"fmt"

	"github.com/topfreegames/extensions/redis/interfaces"
)

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

// GetRoomStatusSetRedisKey gets the key for the set that will keep rooms in a determined state in redis
func GetRoomStatusSetRedisKey(schedulerName, status string) string {
	return fmt.Sprintf("scheduler:%s:status:%s", schedulerName, status)
}

// GetRoomsCountByStatus returns the count of rooms for each status
func GetRoomsCountByStatus(redisClient interfaces.RedisClient, schedulerName string) (*RoomsStatusCount, error) {
	// TODO: treat redis nil error
	pipe := redisClient.TxPipeline()
	sCreating := pipe.SCard(GetRoomStatusSetRedisKey(schedulerName, StatusCreating))
	sReady := pipe.SCard(GetRoomStatusSetRedisKey(schedulerName, StatusReady))
	sOccupied := pipe.SCard(GetRoomStatusSetRedisKey(schedulerName, StatusOccupied))
	sTerminating := pipe.SCard(GetRoomStatusSetRedisKey(schedulerName, StatusTerminating))
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
