// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"fmt"

	"github.com/go-redis/redis"
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
	pipe := redisClient.TxPipeline()
	results := GetRoomsCountByStatusWithPipe(schedulerName, pipe)
	_, err := pipe.Exec()
	if err != nil {
		return nil, err
	}
	countByStatus := RedisResultToRoomsCount(results)
	return countByStatus, nil
}

//GetRoomsCountByStatusWithPipe adds to the redis pipeline the operations that count the number of elements
//  on the redis sets
func GetRoomsCountByStatusWithPipe(
	schedulerName string,
	pipe redis.Pipeliner,
) map[string]*redis.IntCmd {
	results := make(map[string]*redis.IntCmd)

	results[StatusCreating] = pipe.SCard(GetRoomStatusSetRedisKey(schedulerName, StatusCreating))
	results[StatusReady] = pipe.SCard(GetRoomStatusSetRedisKey(schedulerName, StatusReady))
	results[StatusOccupied] = pipe.SCard(GetRoomStatusSetRedisKey(schedulerName, StatusOccupied))
	results[StatusTerminating] = pipe.SCard(GetRoomStatusSetRedisKey(schedulerName, StatusTerminating))

	return results
}

//RedisResultToRoomsCount converts the redis results to ints and returns the RoomsStatusCount struct
func RedisResultToRoomsCount(results map[string]*redis.IntCmd) *RoomsStatusCount {
	if results == nil {
		return nil
	}

	countByStatus := &RoomsStatusCount{}
	countByStatus.Creating = int(results[StatusCreating].Val())
	countByStatus.Ready = int(results[StatusReady].Val())
	countByStatus.Occupied = int(results[StatusOccupied].Val())
	countByStatus.Terminating = int(results[StatusTerminating].Val())

	return countByStatus
}
