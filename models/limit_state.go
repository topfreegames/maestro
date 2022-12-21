package models

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	goredis "github.com/go-redis/redis"
	redisinterfaces "github.com/topfreegames/extensions/v9/redis/interfaces"
)

// LimitManager helps to manage scale ups on limit case.
// It only lets happen one panic scale up per scheduler at once.
type LimitManager struct {
	logger logrus.FieldLogger
	redis  redisinterfaces.RedisClient
	config *viper.Viper
}

// NewLimitManager ...
func NewLimitManager(
	logger logrus.FieldLogger,
	redis redisinterfaces.RedisClient,
	config *viper.Viper,
) *LimitManager {
	return &LimitManager{
		logger: logger.WithField("operation", "LimitManager"),
		redis:  redis,
		config: config,
	}
}

func (l *LimitManager) key(room *Room) string {
	return fmt.Sprintf("maestro:panic:lock:%s", room.SchedulerName)
}

// Lock saves a lock on redis
func (l *LimitManager) Lock(room *Room, configYaml *ConfigYAML) error {
	logger := l.logger.WithField("method", "Lock")
	key := l.key(room)

	logger.WithField("key", key).Info("locking redis for limit manager scale up")

	pipe := l.redis.TxPipeline()
	pipe.HMSet(key, map[string]interface{}{
		"image":     configYaml.Image,
		"deltaPods": configYaml.AutoScaling.Up.Delta,
		"timestamp": time.Now(),
	})

	pipe.Expire(key, l.config.GetDuration("api.limitManager.keyTimeout"))

	_, err := pipe.Exec()

	if err != nil {
		logger.WithField("key", key).
			WithError(err).Error("failed to lock redis for limit manager scale up")
	}

	return err
}

// IsLocked ...
func (l *LimitManager) IsLocked(room *Room) (bool, error) {
	logger := l.logger.WithField("method", "IsLocked")

	logger.Info("checking if locked")

	key := l.key(room)

	result, err := l.redis.HGetAll(key).Result()
	if err != nil && err != goredis.Nil {
		logger.WithError(err).Error("failed to access redis")
		// TODO: se nao achar, err é nil?
		return false, err
	}

	if result == nil || len(result) == 0 {
		return false, nil
	}

	logger.WithFields(logrus.Fields{
		"image":     result["image"],
		"deltaPods": result["deltaPods"],
		"timestamp": result["timestamp"],
	}).Info("redis is locked")

	return true, nil
}

// Unlock ...
func (l *LimitManager) Unlock(room *Room) error {
	key := l.key(room)
	return l.redis.Del(key).Err()
}
