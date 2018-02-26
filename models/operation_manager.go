package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/google/uuid"

	goredis "github.com/go-redis/redis"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
)

// OperationManager controls wheter a maestro operation should
// continue or not
type OperationManager struct {
	schedulerName   string
	redisClient     redisinterfaces.RedisClient
	logger          logrus.FieldLogger
	operationKey    string
	wasCanceledBool bool
	continueLoop    bool
	operationName   string
}

// NewOperationManager returns an instance of operation manager
func NewOperationManager(
	schedulerName string,
	redisClient redisinterfaces.RedisClient,
	logger logrus.FieldLogger,
) *OperationManager {
	o := &OperationManager{
		schedulerName: schedulerName,
		redisClient:   redisClient,
		logger:        logger,
		continueLoop:  true,
	}
	o.operationKey = o.buildKey()
	return o
}

// GetOperationKey returns the operation key
func (o *OperationManager) GetOperationKey() string {
	return o.operationKey
}

func (o *OperationManager) buildKey() string {
	token := uuid.New().String()
	return fmt.Sprintf("opmanager:%s:%s", o.schedulerName, token)
}

// Start saves on redis that an operation
// started on this scheduler
func (o *OperationManager) Start(
	timeout time.Duration,
	operationName string,
) (err error) {
	o.operationName = operationName

	l := o.logger.WithFields(logrus.Fields{
		"source":       "OperationManager",
		"operation":    "Start",
		"operationKey": o.operationKey,
	})

	l.Debug("saving operation key on redis")

	txpipeline := o.redisClient.TxPipeline()
	txpipeline.HMSet(o.operationKey, map[string]interface{}{
		"operation": operationName,
		"progress":  "running",
	})
	txpipeline.Expire(o.operationKey, timeout)
	_, err = txpipeline.Exec()

	if err == goredis.Nil {
		err = nil
	}
	if err != nil {
		return err
	}

	l.Debug("starting check loop on a goroutine")
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for o.continueLoop {
			select {
			case <-ticker.C:
				l.Debug("operationManager loop")
				wasCanceled, err := o.wasCanceled()
				if err != nil {
					l.WithError(err).Warn("error getting operationKey on redis")
					continue
				}
				if wasCanceled {
					l.Debug("operation was canceled")
					o.continueLoop = false
					o.wasCanceledBool = true
					continue
				}

				l.Debug("operation was not canceled, continuing...")
			}
		}
	}()

	return nil
}

// WasCanceled returns a channel that is true when operation was canceled
func (o *OperationManager) WasCanceled() bool {
	if o == nil {
		return false
	}

	return o.wasCanceledBool
}

func (o *OperationManager) wasCanceled() (bool, error) {
	l := o.logger.WithFields(logrus.Fields{
		"source":       "OperationManager",
		"operation":    "wasCanceled",
		"operationKey": o.operationKey,
	})

	if !o.continueLoop {
		return true, nil
	}

	l.Debug("checking if operation was canceled")
	r, err := o.redisClient.HGetAll(o.operationKey).Result()
	l.Debug("successfully accessed redis")

	if err != nil {
		return false, err
	}

	wasCanceled := r == nil || len(r) == 0
	return wasCanceled, nil
}

// Cancel removes operationKey from redis and this cancels an operation
func (o *OperationManager) Cancel(operationKey string) error {
	o.wasCanceledBool = true
	o.continueLoop = false

	l := o.logger.WithFields(logrus.Fields{
		"source":       "OperationManager",
		"operation":    "Cancel",
		"operationKey": operationKey,
	})

	l.Debug("canceling operation")
	if !o.isValid(operationKey) {
		l.Error("operation key is invalid")
		return fmt.Errorf("operationKey is not valid: %s", operationKey)
	}

	txpipeline := o.redisClient.TxPipeline()
	txpipeline.Del(operationKey)
	_, err := txpipeline.Exec()

	if err != nil {
		l.WithError(err).Error("error deleting key from redis")
		return err
	}

	l.Debug("successfully deleted key from redis")
	return nil
}

// Finish updates operationKey on redis to the finish state
func (o *OperationManager) Finish(status int, description string, opErr error) error {
	o.continueLoop = false

	l := o.logger.WithFields(logrus.Fields{
		"source":       "OperationManager",
		"operation":    "Finish",
		"operationKey": o.operationKey,
	})

	result := map[string]interface{}{
		"success":   opErr == nil,
		"status":    status,
		"operation": o.operationName,
	}

	if opErr != nil {
		result["description"] = description
		result["error"] = opErr.Error()
	} else {
		result["progress"] = "100%"
	}

	l.WithFields(logrus.Fields(result)).Info("saving result on redis")

	txpipeline := o.redisClient.TxPipeline()
	txpipeline.HMSet(o.operationKey, result)
	txpipeline.Expire(o.operationKey, 10*time.Minute)
	_, err := txpipeline.Exec()

	if err == goredis.Nil {
		err = nil
	}

	if err != nil {
		l.WithError(err).Error("error saving result on redis")
		return err
	}

	l.WithFields(logrus.Fields(result)).Info("successfully saved on redis")
	return nil
}

func (o *OperationManager) isValid(operationKey string) bool {
	prefix := fmt.Sprintf("opmanager:%s", o.schedulerName)
	return strings.HasPrefix(operationKey, prefix)
}

// Get returns the information about the operationKey on redis
func (o *OperationManager) Get(operationKey string) (map[string]string, error) {
	if !o.isValid(operationKey) {
		return nil, fmt.Errorf("operationKey is not valid: %s", operationKey)
	}

	result, err := o.redisClient.HGetAll(operationKey).Result()
	if err == goredis.Nil {
		// go-redis returns goredis.Nil when key does not exist
		return nil, nil
	}

	return result, err
}

// StopLoop sets continueLoop to false
func (o *OperationManager) StopLoop() {
	o.continueLoop = false
}

// IsStopped returns true if operation manager is not in loop
func (o *OperationManager) IsStopped() bool {
	return !o.continueLoop
}
