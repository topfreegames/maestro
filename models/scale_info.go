// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"fmt"
	"strconv"

	goredis "github.com/go-redis/redis"
	redis "github.com/topfreegames/extensions/redis/interfaces"
)

type scaleType string

const (
	// ScaleTypeUp defines up scale type
	ScaleTypeUp scaleType = "up"
	// ScaleTypeDown defines down scale type
	ScaleTypeDown scaleType = "down"
)

//ScaleInfo holds information about last time scheduler was verified if it needed to be scaled
// and how many time it was above or below threshold
type ScaleInfo struct {
	redis redis.RedisClient
}

// NewScaleInfo returns a new ScaleInfo
func NewScaleInfo(redis redis.RedisClient) *ScaleInfo {
	return &ScaleInfo{
		redis: redis,
	}
}

// Key returns the redis key from scheduler name
func (s *ScaleInfo) Key(schedulerName string, metric AutoScalingPolicyType) string {
	return fmt.Sprintf("maestro:scale:%s:%s", metric, schedulerName)
}

// Capacity calculates the number of points to store
func (s *ScaleInfo) Capacity(triggerTime, autoScalingPeriod int) int {
	return triggerTime / autoScalingPeriod
}

// ReturnStatus check the list of Usages.
// If this list of usages has a % of points above threshold,
// returns true.
func (s *ScaleInfo) ReturnStatus(
	schedulerName string,
	metric AutoScalingPolicyType,
	scaleType scaleType,
	size, total, threshold int,
	usage float32,
) (bool, error) {
	size64 := int64(size)
	key := s.Key(schedulerName, metric)
	pipe := s.redis.TxPipeline()
	usagesRedis := s.returnCircularList(pipe, key, size64)

	_, err := pipe.Exec()
	if err != nil {
		return false, err
	}

	usages, _ := s.convertStringCmdToFloats(usagesRedis)

	return s.isAboveThreshold(scaleType, usages, usage, threshold, size64), nil
}

// SendUsage saves a new usage percentage on Redis
func (s *ScaleInfo) SendUsage(
	schedulerName string,
	metric AutoScalingPolicyType,
	currentUsage float32,
	size int64,
) error {
	key := s.Key(schedulerName, metric)
	pipe := s.redis.TxPipeline()
	s.pushToCircularList(pipe, key, currentUsage, size)
	_, err := pipe.Exec()
	return err
}

func (s *ScaleInfo) pushToCircularList(pipe goredis.Pipeliner, key string, usage float32, size int64) {
	pipe.LPush(key, usage)
	pipe.LTrim(key, int64(0), size)
}

func (s *ScaleInfo) returnCircularList(pipe goredis.Pipeliner, key string, size int64) *goredis.StringSliceCmd {
	return pipe.LRange(key, int64(0), size)
}

func (s *ScaleInfo) isAboveThreshold(scaleType scaleType, usages []float32, usage float32, threshold int, size int64) bool {
	pointsAboveUsage := 0
	for _, usageFromArr := range usages {
		if scaleType == ScaleTypeUp && usageFromArr > usage {
			pointsAboveUsage = pointsAboveUsage + 1
		} else if scaleType == ScaleTypeDown && usageFromArr < usage {
			pointsAboveUsage = pointsAboveUsage + 1
		}
	}
	return pointsAboveUsage*100 > threshold*int(size)
}

func (s *ScaleInfo) convertStringCmdToFloats(usagesRedis *goredis.StringSliceCmd) ([]float32, error) {
	usagesStr, err := usagesRedis.Result()
	if err != nil {
		return nil, err
	}

	usages := make([]float32, len(usagesStr))
	for idx, usage := range usagesStr {
		value, err := strconv.ParseFloat(usage, 32)
		if err != nil {
			return nil, err
		}

		usages[idx] = float32(value)
	}

	return usages, nil
}
