// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/topfreegames/extensions/redis/interfaces"
	maestroErrors "github.com/topfreegames/maestro/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

// Room is the struct that defines a room in maestro
type Room struct {
	ID            string
	SchedulerName string
	Status        string
	LastPingAt    int64
}

// RoomAddresses struct
type RoomAddresses struct {
	Ports []*RoomPort `json:"ports"`
	Host  string      `json:"host"`
}

// RoomAddress struct
type RoomPort struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
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
	r.LastPingAt = time.Now().Unix()
	pipe := redisClient.TxPipeline()
	pipe.HMSet(r.GetRoomRedisKey(), map[string]interface{}{
		"status":   r.Status,
		"lastPing": r.LastPingAt,
	})
	pipe.SAdd(GetRoomStatusSetRedisKey(r.SchedulerName, r.Status), r.GetRoomRedisKey())
	pipe.ZAdd(GetRoomPingRedisKey(r.SchedulerName), redis.Z{
		Score:  float64(r.LastPingAt),
		Member: r.ID,
	})
	_, err := pipe.Exec()
	return err
}

const ZaddIfNotExists = `
local c = redis.call('zscore', KEYS[1], ARGV[1])
if not c then
	redis.call('zadd', KEYS[1], KEYS[2], ARGV[1])
end
return 'OK'
`

// SetStatus updates the status of a given room in the database
func (r *Room) SetStatus(redisClient interfaces.RedisClient, status string) error {
	allStatus := []string{StatusCreating, StatusReady, StatusOccupied, StatusTerminating, StatusTerminated}
	r.Status = status
	r.LastPingAt = time.Now().Unix()
	if status == StatusTerminated {
		return r.ClearAll(redisClient)
	}
	pipe := redisClient.TxPipeline()
	pipe.HMSet(r.GetRoomRedisKey(), map[string]interface{}{
		"status":   r.Status,
		"lastPing": r.LastPingAt,
	})
	pipe.ZAdd(GetRoomPingRedisKey(r.SchedulerName), redis.Z{
		Score:  float64(r.LastPingAt),
		Member: r.ID,
	})

	lastStatusChangedKey := GetLastStatusRedisKey(r.SchedulerName, StatusOccupied)
	lastPingAtStr := strconv.FormatInt(r.LastPingAt, 10)
	if status == StatusOccupied {
		pipe.Eval(
			ZaddIfNotExists,
			[]string{lastStatusChangedKey, lastPingAtStr},
			r.ID,
		)
	} else {
		pipe.ZRem(lastStatusChangedKey, r.ID)
	}

	// remove from other statuses to be safe
	for _, st := range allStatus {
		if st != status {
			pipe.SRem(GetRoomStatusSetRedisKey(r.SchedulerName, st), r.GetRoomRedisKey())
		}
	}
	pipe.SAdd(GetRoomStatusSetRedisKey(r.SchedulerName, r.Status), r.GetRoomRedisKey())
	_, err := pipe.Exec()
	return err
}

// ClearAll removes all room keys from redis
func (r *Room) ClearAll(redisClient interfaces.RedisClient) error {
	allStatus := []string{StatusCreating, StatusReady, StatusOccupied, StatusTerminating, StatusTerminated}
	pipe := redisClient.TxPipeline()
	for _, st := range allStatus {
		pipe.SRem(GetRoomStatusSetRedisKey(r.SchedulerName, st), r.GetRoomRedisKey())
		pipe.ZRem(GetLastStatusRedisKey(r.SchedulerName, st), r.ID)
	}
	pipe.ZRem(GetRoomPingRedisKey(r.SchedulerName), r.ID)
	pipe.Del(r.GetRoomRedisKey())
	_, err := pipe.Exec()
	return err
}

// GetRoomPingRedisKey gets the key for the sortedset that keeps the rooms ping timestamp in redis
func GetRoomPingRedisKey(schedulerName string) string {
	return fmt.Sprintf("scheduler:%s:ping", schedulerName)
}

// GetLastStatusRedisKey gets the key for the sortedset that keeps the last timestamp a room changed its status
func GetLastStatusRedisKey(schedulerName, status string) string {
	return fmt.Sprintf("scheduler:%s:last:status:%s", schedulerName, status)
}

// GetAddresses gets room public addresses
func (r *Room) GetAddresses(kubernetesClient kubernetes.Interface) (*RoomAddresses, error) {
	rAddresses := &RoomAddresses{}
	roomPod, err := kubernetesClient.CoreV1().Pods(r.SchedulerName).Get(r.ID, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if len(roomPod.Spec.NodeName) == 0 {
		return rAddresses, nil
	}

	node, err := kubernetesClient.CoreV1().Nodes().Get(roomPod.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeExternalIP {
			rAddresses.Host = address.Address
			break
		}
	}
	if rAddresses.Host == "" {
		return nil, maestroErrors.NewKubernetesError("no host found", errors.New("no node found to host room"))
	}
	svc, err := kubernetesClient.CoreV1().Services(r.SchedulerName).Get(r.ID, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	for _, port := range svc.Spec.Ports {
		if port.NodePort != 0 {
			rAddresses.Ports = append(rAddresses.Ports, &RoomPort{
				Name: port.Name,
				Port: port.NodePort,
			})
		}
	}
	if len(rAddresses.Ports) == 0 {
		return nil, maestroErrors.NewKubernetesError("no ports found", errors.New("no node port found to host room"))
	}
	return rAddresses, nil
}

// GetRoomsNoPingSince returns a list of rooms ids that have lastPing < since
func GetRoomsNoPingSince(redisClient interfaces.RedisClient, schedulerName string, since int64) ([]string, error) {
	return redisClient.ZRangeByScore(
		GetRoomPingRedisKey(schedulerName),
		redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(since, 10)},
	).Result()
}

// GetRoomsOccupiedTimeout return a list of rooms ids that have been occupied since 'since'
func GetRoomsOccupiedTimeout(redisClient interfaces.RedisClient, schedulerName string, since int64) ([]string, error) {
	return redisClient.ZRangeByScore(
		GetLastStatusRedisKey(schedulerName, StatusOccupied),
		redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(since, 10)},
	).Result()
}
