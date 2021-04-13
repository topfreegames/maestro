// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2018 Top Free Games <backend@tfgco.com>

package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	goredis "github.com/go-redis/redis"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
)

// OperationManager controls wheter a maestro operation should
// continue or not
type OperationManager struct {
	schedulerName string
	redisClient   redisinterfaces.RedisClient
	logger        logrus.FieldLogger
	operationKey  string
	operationName string
	loopTime      time.Duration
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
	}
	o.operationKey = o.buildKey()
	return o
}

// GetOperationKey returns the operation key
func (o *OperationManager) GetOperationKey() string {
	return o.operationKey
}

// SetOperationKey sets the operation key
func (o *OperationManager) SetOperationKey(operationKey string) {
	o.operationKey = operationKey
}

func (o *OperationManager) buildKey() string {
	token := uuid.New().String()
	return fmt.Sprintf("opmanager:%s:%s", o.schedulerName, token)
}

// BuildCurrOpKey builds the key that gets current op on redis
func (o *OperationManager) BuildCurrOpKey() string {
	return fmt.Sprintf("opmanager:%s:current", o.schedulerName)
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
		"operation":   operationName,
		"description": OpManagerWaitingLock,
	})
	txpipeline.Expire(o.operationKey, timeout)
	txpipeline.Set(o.BuildCurrOpKey(), o.operationKey, timeout)
	_, err = txpipeline.Exec()

	if err == goredis.Nil {
		err = nil
	}
	if err != nil {
		return err
	}

	return nil
}

func (o *OperationManager) WasCanceled() (bool, error) {
	if o == nil {
		return false, nil
	}

	l := o.logger.WithFields(logrus.Fields{
		"source":       "OperationManager",
		"operation":    "WasCanceled",
		"operationKey": o.operationKey,
	})

	l.Debug("checking if operation was canceled")
	r, err := o.redisClient.HGetAll(o.operationKey).Result()

	if err != nil {
		return false, err
	}
	l.Debug("successfully accessed redis")

	wasCanceled := r == nil || len(r) == 0
	return wasCanceled, nil
}

// Cancel removes operationKey from redis and this cancels an operation
func (o *OperationManager) Cancel(operationKey string) error {
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
	l := o.logger.WithFields(logrus.Fields{
		"source":       "OperationManager",
		"operation":    "Finish",
		"operationKey": o.operationKey,
	})

	result := map[string]interface{}{
		"success":     opErr == nil,
		"status":      status,
		"operation":   o.operationName,
		"description": OpManagerFinished,
	}

	if opErr != nil {
		result["description"] = description
		result["error"] = opErr.Error()
	} else {
		result["progress"] = "100"
	}

	l.WithFields(logrus.Fields(result)).Info("saving result on redis")

	txpipeline := o.redisClient.TxPipeline()
	txpipeline.HMSet(o.operationKey, result)
	txpipeline.Expire(o.operationKey, 10*time.Minute)
	txpipeline.Del(o.BuildCurrOpKey())
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

// CurrentOperation returns the current operation key that is in progress
func (o *OperationManager) CurrentOperation() (string, error) {
	currOp, err := o.redisClient.Get(o.BuildCurrOpKey()).Result()
	if err == goredis.Nil {
		err = nil
	}

	return currOp, err
}

// SetDescription sets the description and error of the operation current state
func (o *OperationManager) SetDescription(description string) error {
	return o.redisClient.HMSet(o.operationKey, map[string]interface{}{
		"description": description,
	}).Err()
}

// SetError sets the error string of the operation
func (o *OperationManager) SetError(err string) error {
	return o.redisClient.HMSet(o.operationKey, map[string]interface{}{
		"error":       err,
		"description": OpManagerErrored,
	}).Err()
}

func (o *OperationManager) getOperationRollingProgress(
	mr *MixedMetricsReporter, scheduler Scheduler, status map[string]string,
) (float64, error) {
	progress := float64(0)

	roomsCount, err := GetPodCountFromRedis(o.redisClient, mr, scheduler.Name)
	if err != nil {
		return 0, err
	}

	invalidCount, err := GetCurrentInvalidRoomsCount(o.redisClient, mr, scheduler.Name)
	if err != nil {
		return 0, err
	}

	if roomsCount <= 0 {
		progress = 1
	} else {
		progress = 1 - float64(invalidCount)/float64(roomsCount)
	}

	// if the percentage of gameservers with the actual version is 100% but the
	// operation is not 'rolling update', the new scheduler version has not been stored as the actual version yet.
	// it means that the rolling update didn't started and so the progress should be 0%
	if status["description"] != OpManagerRollingUpdate && progress > 0.99 {
		return 0, nil
	}

	return 100.0 * progress, nil
}

// GetOperationStatus returns an operation status with progress percentage
func (o *OperationManager) GetOperationStatus(mr *MixedMetricsReporter, scheduler Scheduler) (map[string]string, error) {
	status, err := o.Get(o.operationKey)
	if err != nil {
		return nil, err
	}

	if status == nil || len(status) == 0 {
		return nil, nil
	}

	// if status is set, the operation is finished. Progress is set to 100 in Finish method
	if _, ok := status["status"]; ok {
		return status, nil
	}

	progress, err := o.getOperationRollingProgress(mr, scheduler, status)

	status["progress"] = strconv.FormatFloat(progress, 'f', 2, 64)
	return status, nil
}
