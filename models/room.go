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

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"

	"github.com/topfreegames/extensions/redis/interfaces"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Addresses []*RoomAddress `json:"addresses"`
}

// RoomAddress struct
type RoomAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
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
	var podNodeIP string
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeExternalIP {
			podNodeIP = address.Address
		}
	}
	svc, err := kubernetesClient.CoreV1().Services(r.SchedulerName).Get(r.ID, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	for _, port := range svc.Spec.Ports {
		rAddresses.Addresses = append(rAddresses.Addresses, &RoomAddress{
			Name:    port.Name,
			Address: fmt.Sprintf("%s:%d", podNodeIP, port.NodePort),
		})
	}
	return rAddresses, nil
}
