// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"strconv"

	"github.com/topfreegames/extensions/redis/interfaces"
)

const stateStr = "state"
const lastChangedAtStr = "lastChangedAt"
const lastScaleOpAtStr = "lastScaleOpAt"

// SchedulerState is the struct that defines a scheduler state in maestro
type SchedulerState struct {
	Name          string
	State         string
	LastChangedAt int64
	LastScaleOpAt int64
}

// NewSchedulerState is the scheduler state constructor
func NewSchedulerState(name, state string, lastChangedAt, lastScaleOpAt int64) *SchedulerState {
	return &SchedulerState{
		Name:          name,
		State:         state,
		LastChangedAt: lastChangedAt,
		LastScaleOpAt: lastScaleOpAt,
	}
}

// Save saves a scheduler state in redis
func (s *SchedulerState) Save(client interfaces.RedisClient) error {
	values := map[string]interface{}{
		stateStr:         s.State,
		lastChangedAtStr: s.LastChangedAt,
		lastScaleOpAtStr: s.LastScaleOpAt,
	}
	return client.HMSet(s.Name, values).Err()
}

// Load loads a scheduler state from redis
func (s *SchedulerState) Load(client interfaces.RedisClient) error {
	results, err := client.HGetAll(s.Name).Result()
	if err != nil {
		return err
	}
	s.State = results[stateStr]
	s.LastChangedAt, err = strconv.ParseInt(results[lastChangedAtStr], 10, 64)
	if err != nil {
		return err
	}
	s.LastScaleOpAt, err = strconv.ParseInt(results[lastScaleOpAtStr], 10, 64)
	if err != nil {
		return err
	}
	return nil
}
