package models

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pmylund/go-cache"
	"github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/maestro/errors"
)

// SchedulerCache holds the scheduler in memory for x minutes for performance boost
type SchedulerCache struct {
	configYaml *ConfigYAML
	cache      *cache.Cache
	logger     logrus.FieldLogger
}

//NewSchedulerCache returns a new *SchedulerCache
func NewSchedulerCache(
	expirationTime, purgeTime time.Duration,
	logger logrus.FieldLogger,
) *SchedulerCache {
	return &SchedulerCache{
		cache:  cache.New(expirationTime, purgeTime),
		logger: logger,
	}
}

// SchedulerKey returns the key for the scheduler cache
func SchedulerKey(schedulerName string) string {
	return fmt.Sprintf("scheduler:key:%s", schedulerName)
}

// LoadScheduler returns scheduler from cache
//  If cache does not have scheduler or useCache is false, scheduler is loaded from database
//  In this case, configYaml is also updated
func (s *SchedulerCache) LoadScheduler(db interfaces.DB, schedulerName string, useCache bool) (*Scheduler, error) {
	if s == nil {
		return loadFromDB(db, schedulerName)
	}

	schedulerKey := SchedulerKey(schedulerName)

	logger := s.logger.WithFields(logrus.Fields{
		"operation": "CacheLoad",
		"scheduler": schedulerName,
	})

	if useCache {
		scheduler, found := s.cache.Get(schedulerKey)
		if found {
			logger.Debug("found on cache")
			return scheduler.(*Scheduler), nil
		}
		logger.Debug("not found on cache")
	}

	scheduler, err := loadFromDB(db, schedulerName)
	if err != nil {
		return nil, err
	}
	logger.Debug("adding scheduler on cache")

	s.cache.Set(schedulerKey, scheduler, cache.DefaultExpiration)
	s.configYaml, err = NewConfigYAML(scheduler.YAML)
	if err != nil {
		return nil, err
	}

	if s.configYaml.AutoScaling.Up.Trigger.Limit == 0 {
		s.configYaml.AutoScaling.Up.Trigger.Limit = 90
	}

	return scheduler, nil
}

func loadFromDB(db interfaces.DB, schedulerName string) (*Scheduler, error) {
	scheduler := NewScheduler(schedulerName, "", "")
	err := scheduler.Load(db)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	if scheduler.YAML == "" {
		msg := fmt.Errorf("scheduler \"%s\" not found", schedulerName)
		return nil, errors.NewValidationFailedError(msg)
	}
	return scheduler, nil
}

// LoadConfigYaml returns scheduler from cache
//  If cache does not have scheduler or useCache is false, scheduler is loaded from database
//  In this case, configYaml is also updated updated
func (s *SchedulerCache) LoadConfigYaml(db interfaces.DB, schedulerName string, useCache bool) (*ConfigYAML, error) {
	_, err := s.LoadScheduler(db, schedulerName, useCache)
	if err != nil {
		return nil, err
	}
	return s.configYaml, nil
}
