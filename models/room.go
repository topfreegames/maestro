// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	e "errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/extensions/redis/interfaces"
	"github.com/topfreegames/maestro/reporters"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsClient "k8s.io/metrics/pkg/client/clientset_generated/clientset"
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
	Ports     []*RoomPort `json:"ports"`
	Host      string      `json:"host"`
	Ipv6Label string      `json:"ipv6Label"`
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
	scheduler *Scheduler,
) error {
	r.LastPingAt = time.Now().Unix()
	pipe := redisClient.TxPipeline()
	_, err := r.addStatusToRedisPipeAndExec(redisClient, db, mr, pipe, false, scheduler)
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
	metricsClientset metricsClient.Interface,
	mr *MixedMetricsReporter,
	status string,
	scheduler *Scheduler,
	returnRoomsCount bool,
) (*RoomsStatusCount, error) {
	allStatus := []string{StatusCreating, StatusReady, StatusOccupied, StatusTerminating, StatusTerminated}
	r.Status = status
	r.LastPingAt = time.Now().Unix()

	if status == StatusTerminated {
		return nil, r.ClearAll(redisClient, mr)
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

	r.addUtilizationMetricsToRedis(metricsClientset, pipe, scheduler)
	// remove from other statuses to be safe
	for _, st := range allStatus {
		if st != status {
			pipe.SRem(GetRoomStatusSetRedisKey(r.SchedulerName, st), r.GetRoomRedisKey())
		}
	}

	return r.addStatusToRedisPipeAndExec(redisClient, db, mr, pipe, returnRoomsCount, scheduler)
}

func (r *Room) addUtilizationMetricsToRedis(
	metricsClientset metricsClient.Interface,
	pipe redis.Pipeliner,
	scheduler *Scheduler,
) {
	sp := scheduler.GetAutoScalingPolicy()
	metricsMap := map[AutoScalingPolicyType]bool{}
	metricsTriggers := append(sp.Up.MetricsTrigger, sp.Down.MetricsTrigger...)
	for _, trigger := range metricsTriggers {
		if ResourcePolicyType(trigger.Type) {
			metricsMap[trigger.Type] = true
		}
	}

	requests := scheduler.GetResourcesRequests()
	pmetrics, err := metricsClientset.Metrics().PodMetricses(r.SchedulerName).Get(r.ID, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return
	}

	for metric := range metricsMap {
		usage := int64(0)
		if err != nil {
			// add rooms with unavailable metrics to the end of the sorted set
			usage = math.MaxInt64
		} else {
			for _, container := range pmetrics.Containers {
				usage += GetResourceUsage(container.Usage, metric)
			}
			reportUsage(scheduler.Game, scheduler.Name, string(metric), requests[metric], usage)
		}

		pipe.ZAdd(
			GetRoomMetricsRedisKey(r.SchedulerName, string(metric)),
			redis.Z{Member: r.ID, Score: float64(usage)},
		)
	}
}

func (r *Room) addStatusToRedisPipeAndExec(
	redisClient interfaces.RedisClient,
	db pginterfaces.DB,
	mr *MixedMetricsReporter,
	p redis.Pipeliner,
	returnRoomsCount bool,
	scheduler *Scheduler,
) (*RoomsStatusCount, error) {
	prevStatus := "nil"

	if reporters.HasReporters() {
		var rFields *redis.StringStringMapCmd
		mr.WithSegment(SegmentHGetAll, func() error {
			rFields = redisClient.HGetAll(r.GetRoomRedisKey())
			return nil
		})
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

	err := mr.WithSegment(SegmentPipeExec, func() error {
		var err error
		_, err = p.Exec()
		return err
	})
	if err != nil {
		return nil, err
	}

	roomsCountByStatus := RedisResultToRoomsCount(results)
	return roomsCountByStatus, r.reportStatus(redisClient, db, mr, r.Status, prevStatus != r.Status, scheduler)
}

func (r *Room) reportStatus(
	redisClient interfaces.RedisClient,
	db pginterfaces.DB,
	mr *MixedMetricsReporter,
	status string,
	statusChanged bool,
	scheduler *Scheduler,
) error {
	if !reporters.HasReporters() {
		return nil
	}
	pipe := redisClient.TxPipeline()
	nStatus := pipe.SCard(GetRoomStatusSetRedisKey(r.SchedulerName, status))
	err := mr.WithSegment(SegmentPipeExec, func() error {
		var err error
		_, err = pipe.Exec()
		return err
	})
	if err != nil {
		return err
	}

	if statusChanged {
		err = reportStatus(scheduler.Game, r.SchedulerName, status,
			fmt.Sprint(float64(nStatus.Val())))
	} else {
		err = reportPing(scheduler.Game, r.SchedulerName)
	}
	return err
}

func reportPing(game, scheduler string) error {
	return reporters.Report(reportersConstants.EventGruPing, map[string]interface{}{
		reportersConstants.TagGame:      game,
		reportersConstants.TagScheduler: scheduler,
	})
}

func reportStatus(game, scheduler, status, gauge string) error {
	return reporters.Report(reportersConstants.EventGruStatus, map[string]interface{}{
		reportersConstants.TagGame:      game,
		reportersConstants.TagScheduler: scheduler,
		"status":                        status,
		"gauge":                         gauge,
	})
}

func reportUsage(game, scheduler, metric string, requests, usage int64) error {
	if requests == 0 {
		return e.New("cannot divide by zero")
	}
	gauge := fmt.Sprintf("%.2f", float64(usage)/float64(requests))
	return reporters.Report(reportersConstants.EventGruMetricUsage, map[string]interface{}{
		reportersConstants.TagGame:      game,
		reportersConstants.TagScheduler: scheduler,
		reportersConstants.TagMetric:    metric,
		"gauge": gauge,
	})
}

// ClearAll removes all room keys from redis
func (r *Room) ClearAll(redisClient interfaces.RedisClient, mr *MixedMetricsReporter) error {
	pipe := redisClient.TxPipeline()
	r.clearAllWithPipe(pipe)
	err := mr.WithSegment(SegmentPipeExec, func() error {
		_, err := pipe.Exec()
		return err
	})
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
	for _, policy := range GetAvailablePolicyTypes() {
		if ResourcePolicyType(policy) {
			pipe.ZRem(GetRoomMetricsRedisKey(r.SchedulerName, string(policy)), r.ID)
		}
	}
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

// GetRoomMetricsRedisKey gets the key for the sortedset that keeps the rooms metrics in redis
func GetRoomMetricsRedisKey(schedulerName string, metric string) string {
	return fmt.Sprintf("scheduler:%s:metric:%s", schedulerName, metric)
}

// GetRoomInfos returns a map with room informations
func (r *Room) GetRoomInfos(
	db pginterfaces.DB,
	kubernetesClient kubernetes.Interface,
	schedulerCache *SchedulerCache,
	scheduler *Scheduler,
	addrGetter AddrGetter,
) (map[string]interface{}, error) {
	if scheduler == nil {
		cachedScheduler, err := schedulerCache.LoadScheduler(db, r.SchedulerName, true)
		if err != nil {
			return nil, err
		}
		scheduler = cachedScheduler.Scheduler
	}
	address, err := addrGetter.Get(r, kubernetesClient)
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
		"game":      scheduler.Game,
		"roomId":    r.ID,
		"host":      address.Host,
		"ipv6Label": address.Ipv6Label,
		"port":      selectedPort,
	}, nil
}

// GetRoomsNoPingSince returns a list of rooms ids that have lastPing < since
func GetRoomsNoPingSince(redisClient interfaces.RedisClient, schedulerName string, since int64, mr *MixedMetricsReporter) ([]string, error) {
	var result []string
	err := mr.WithSegment(SegmentZRangeBy, func() error {
		var err error
		result, err = redisClient.ZRangeByScore(
			GetRoomPingRedisKey(schedulerName),
			redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(since, 10)},
		).Result()
		return err
	})
	return result, err
}

// GetRoomsOccupiedTimeout return a list of rooms ids that have been occupied since 'since'
func GetRoomsOccupiedTimeout(redisClient interfaces.RedisClient, schedulerName string, since int64, mr *MixedMetricsReporter) ([]string, error) {
	var result []string
	err := mr.WithSegment(SegmentZRangeBy, func() error {
		var err error
		result, err = redisClient.ZRangeByScore(
			GetLastStatusRedisKey(schedulerName, StatusOccupied),
			redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(since, 10)},
		).Result()
		return err
	})
	return result, err
}

func getReadyRooms(tx redis.Pipeliner, schedulerName string, size int, mr *MixedMetricsReporter) ([]string, error) {
	var result []string
	rooms := tx.SRandMemberN(
		GetRoomStatusSetRedisKey(schedulerName, StatusReady),
		int64(size),
	)
	err := mr.WithSegment(SegmentSRandMember, func() error {
		_, err := tx.Exec()
		return err
	})
	if err != nil {
		return nil, err
	}
	result, err = rooms.Result()
	if err != nil {
		return nil, err
	}

	res := make([]string, len(result))
	for idx, s := range result {
		a := strings.Split(s, ":")
		res[idx] = a[len(a)-1]
	}
	return res, err
}

// GetRoomsByMetric returns a list of rooms ordered by metric
func GetRoomsByMetric(redisClient interfaces.RedisClient, schedulerName string, metricName string, size int, mr *MixedMetricsReporter) ([]string, error) {
	var result []string
	// TODO hacky, one day add ZRange and SRandMember to redis interface
	tx := redisClient.TxPipeline()
	if metricName == string(RoomAutoScalingPolicyType) || metricName == string(LegacyAutoScalingPolicyType) {
		return getReadyRooms(tx, schedulerName, size, mr)
	}
	rooms := tx.ZRange(
		GetRoomMetricsRedisKey(schedulerName, metricName),
		int64(0),
		int64(size-1),
	)
	err := mr.WithSegment(SegmentZRangeBy, func() error {
		_, err := tx.Exec()
		return err
	})
	if err != nil {
		return nil, err
	}
	result, err = rooms.Result()
	return result, err
}
