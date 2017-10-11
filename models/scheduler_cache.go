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
	cache  *cache.Cache
	logger logrus.FieldLogger
}

// CachedScheduler is the struct for the scheduler that will be kept in the cache
type CachedScheduler struct {
	Scheduler  *Scheduler
	ConfigYAML *ConfigYAML
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
func (s *SchedulerCache) LoadScheduler(
	db interfaces.DB, schedulerName string, useCache bool,
) (*CachedScheduler, error) {
	if s == nil {
		scheduler, err := loadFromDB(db, schedulerName)
		if err != nil {
			return nil, err
		}

		configYaml, err := NewConfigYAML(scheduler.YAML)
		if err != nil {
			return nil, err
		}

		return &CachedScheduler{
			Scheduler:  scheduler,
			ConfigYAML: configYaml,
		}, nil

	}

	schedulerKey := SchedulerKey(schedulerName)
	logger := s.logger.WithFields(logrus.Fields{
		"operation": "CacheLoad",
		"scheduler": schedulerName,
	})

	if useCache {
		cachedScheduler, found := s.cache.Get(schedulerKey)
		if found {
			logger.Debug("found on cache")
			return cachedScheduler.(*CachedScheduler), nil
		}
		logger.Debug("not found on cache")
	}

	scheduler, err := loadFromDB(db, schedulerName)
	if err != nil {
		return nil, err
	}

	configYaml, err := NewConfigYAML(scheduler.YAML)
	if err != nil {
		return nil, err
	}
	if configYaml.AutoScaling.Up.Trigger.Limit == 0 {
		configYaml.AutoScaling.Up.Trigger.Limit = 90
	}

	cachedScheduler := &CachedScheduler{
		Scheduler:  scheduler,
		ConfigYAML: configYaml,
	}
	logger.Debug("adding scheduler on cache")
	s.cache.Set(schedulerKey, cachedScheduler, cache.DefaultExpiration)
	return cachedScheduler, nil
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
