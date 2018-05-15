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
	"strings"
	"time"

	"github.com/go-redis/redis"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/extensions/redis/interfaces"
	maestroErrors "github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/reporters"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// RoomReady string representation
	RoomReady = "ready"
	// RoomOccupied string representation
	RoomOccupied = "occupied"
	// RoomTerminating string representation
	RoomTerminating = "terminating"
	// RoomTerminated string representation
	RoomTerminated = "terminated"
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

// RoomPort struct
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

// RoomFromRedisKey gets the room name from redis key
func RoomFromRedisKey(key string) string {
	splits := strings.Split(key, ":")
	len := len(splits)
	for idx, split := range splits {
		if split == "rooms" && idx+1 < len {
			return splits[idx+1]
		}
	}
	return ""
}

// Create creates a room in and update redis
func (r *Room) Create(
	redisClient interfaces.RedisClient,
	db pginterfaces.DB,
	mr *MixedMetricsReporter,
	configYaml *ConfigYAML,
) error {
	r.LastPingAt = time.Now().Unix()
	pipe := redisClient.TxPipeline()
	_, err := r.addStatusToRedisPipeAndExec(redisClient, db, mr, pipe, false, configYaml)
	return err
}

//ZaddIfNotExists adds to zset if not
const ZaddIfNotExists = `
local c = redis.call('zscore', KEYS[1], ARGV[1])
if not c then
	redis.call('zadd', KEYS[1], KEYS[2], ARGV[1])
end
return 'OK'
`

// SetStatus updates the status of a given room in the database and returns how many
//  ready rooms has left
func (r *Room) SetStatus(
	redisClient interfaces.RedisClient,
	db pginterfaces.DB,
	mr *MixedMetricsReporter,
	status string,
	configYaml *ConfigYAML,
	returnRoomsCount bool,
) (*RoomsStatusCount, error) {
	allStatus := []string{StatusCreating, StatusReady, StatusOccupied, StatusTerminating, StatusTerminated}
	r.Status = status
	r.LastPingAt = time.Now().Unix()

	if status == StatusTerminated {
		return nil, r.ClearAll(redisClient)
	}

	lastStatusChangedKey := GetLastStatusRedisKey(r.SchedulerName, StatusOccupied)
	lastPingAtStr := strconv.FormatInt(r.LastPingAt, 10)
	pipe := redisClient.TxPipeline()
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

	return r.addStatusToRedisPipeAndExec(redisClient, db, mr, pipe, returnRoomsCount, configYaml)
}

func (r *Room) addStatusToRedisPipeAndExec(
	redisClient interfaces.RedisClient,
	db pginterfaces.DB,
	mr *MixedMetricsReporter,
	p redis.Pipeliner,
	returnRoomsCount bool,
	configYaml *ConfigYAML,
) (*RoomsStatusCount, error) {
	prevStatus := "nil"

	if reporters.HasReporters() {
		rFields := redisClient.HGetAll(r.GetRoomRedisKey())
		val, prs := rFields.Val()["status"]
		if prs {
			prevStatus = val
		}
	}

	p.HMSet(r.GetRoomRedisKey(), map[string]interface{}{
		"status":   r.Status,
		"lastPing": r.LastPingAt,
	})
	p.SAdd(GetRoomStatusSetRedisKey(r.SchedulerName, r.Status), r.GetRoomRedisKey())
	p.ZAdd(GetRoomPingRedisKey(r.SchedulerName), redis.Z{
		Score:  float64(r.LastPingAt),
		Member: r.ID,
	})

	var results map[string]*redis.IntCmd
	if returnRoomsCount {
		results = GetRoomsCountByStatusWithPipe(r.SchedulerName, p)
	}

	_, err := p.Exec()
	if err != nil {
		return nil, err
	}

	roomsCountByStatus := RedisResultToRoomsCount(results)
	return roomsCountByStatus, r.reportStatus(redisClient, db, mr, r.Status, prevStatus != r.Status, configYaml)
}

func (r *Room) reportStatus(
	redisClient interfaces.RedisClient,
	db pginterfaces.DB,
	mr *MixedMetricsReporter,
	status string,
	statusChanged bool,
	configYaml *ConfigYAML,
) error {
	if !reporters.HasReporters() {
		return nil
	}
	pipe := redisClient.TxPipeline()
	nStatus := pipe.SCard(GetRoomStatusSetRedisKey(r.SchedulerName, status))
	_, err := pipe.Exec()
	if err != nil {
		return err
	}

	if statusChanged {
		err = reportStatus(configYaml.Game, r.SchedulerName, status,
			fmt.Sprint(float64(nStatus.Val())))
	} else {
		err = reportPing(configYaml.Game, r.SchedulerName)
	}
	return err
}

func reportPing(game, scheduler string) error {
	return reporters.Report(reportersConstants.EventGruPing, map[string]string{
		reportersConstants.TagGame:      game,
		reportersConstants.TagScheduler: scheduler,
	})
}

func reportStatus(game, scheduler, status, gauge string) error {
	return reporters.Report(reportersConstants.EventGruStatus, map[string]string{
		reportersConstants.TagGame:      game,
		reportersConstants.TagScheduler: scheduler,
		"status":                        status,
		"gauge":                         gauge,
	})
}

// ClearAll removes all room keys from redis
func (r *Room) ClearAll(redisClient interfaces.RedisClient) error {
	pipe := redisClient.TxPipeline()
	r.clearAllWithPipe(pipe)
	_, err := pipe.Exec()
	return err
}

func (r *Room) clearAllWithPipe(
	pipe redis.Pipeliner,
) {
	allStatus := []string{StatusCreating, StatusReady, StatusOccupied, StatusTerminating, StatusTerminated}
	for _, st := range allStatus {
		pipe.SRem(GetRoomStatusSetRedisKey(r.SchedulerName, st), r.GetRoomRedisKey())
		pipe.ZRem(GetLastStatusRedisKey(r.SchedulerName, st), r.ID)
	}
	pipe.ZRem(GetRoomPingRedisKey(r.SchedulerName), r.ID)
	pipe.Del(r.GetRoomRedisKey())
}

// GetRoomPingRedisKey gets the key for the sortedset that keeps the rooms ping timestamp in redis
func GetRoomPingRedisKey(schedulerName string) string {
	return fmt.Sprintf("scheduler:%s:ping", schedulerName)
}

// GetLastStatusRedisKey gets the key for the sortedset that keeps the last timestamp a room changed its status
func GetLastStatusRedisKey(schedulerName, status string) string {
	return fmt.Sprintf("scheduler:%s:last:status:%s", schedulerName, status)
}

// GetRoomInfos returns a map with room informations
func (r *Room) GetRoomInfos(
	db pginterfaces.DB,
	kubernetesClient kubernetes.Interface,
	schedulerCache *SchedulerCache,
	scheduler *Scheduler,
) (map[string]interface{}, error) {
	if scheduler == nil {
		cachedScheduler, err := schedulerCache.LoadScheduler(db, r.SchedulerName, true)
		if err != nil {
			return nil, err
		}
		scheduler = cachedScheduler.Scheduler
	}
	address, err := r.GetAddresses(kubernetesClient)
	if err != nil {
		return nil, err
	}
	var selectedPort int32
	if len(address.Ports) > 0 {
		selectedPort = address.Ports[0].Port
	}
	for _, p := range address.Ports {
		if p.Name == "clientPort" {
			selectedPort = p.Port
		}
	}
	return map[string]interface{}{
		"game":   scheduler.Game,
		"roomId": r.ID,
		"host":   address.Host,
		"port":   selectedPort,
	}, nil
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
	for _, port := range roomPod.Spec.Containers[0].Ports {
		//TODO: check if port.HostPort is available (another process not using it)
		if port.HostPort != 0 {
			rAddresses.Ports = append(rAddresses.Ports, &RoomPort{
				Name: port.Name,
				Port: port.HostPort,
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
