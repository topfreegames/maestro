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
	"k8s.io/api/core/v1"

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
	loopTime        time.Duration
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
		loopTime:      10 * time.Second,
	}
	o.operationKey = o.buildKey()
	return o
}

// SetLoopTime sets how much will wait to check again if operation was canceled
// Default is 10s
func (o *OperationManager) SetLoopTime(t time.Duration) {
	o.loopTime = t
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

	l.Debug("starting check loop on a goroutine")
	go func() {
		ticker := time.NewTicker(o.loopTime)
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

// StopLoop sets continueLoop to false
func (o *OperationManager) StopLoop() {
	o.continueLoop = false
}

// IsStopped returns true if operation manager is not in loop
func (o *OperationManager) IsStopped() bool {
	return !o.continueLoop
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
	scheduler Scheduler, totalPods []v1.Pod, status map[string]string,
) float64 {
	new := float64(0)
	total := float64(len(totalPods))

	for _, pod := range totalPods {
		if pod.GetObjectMeta().GetLabels()["version"] == scheduler.Version {
			new++
		}
	}

	// if the percentage of gameservers with the actual version is 100% but the
	// operation is not 'rolling update', the new scheduler version has not been stored as the actual version yet.
	// it means that the rolling update didn't started and so the progress should be 0%
	if status["description"] != OpManagerRollingUpdate && new/total > 0.99 {
		return 0
	}

	return 100.0 * new / total
}

// GetOperationStatus returns an operation status with progress percentage
func (o *OperationManager) GetOperationStatus(scheduler Scheduler, totalPods []v1.Pod) (map[string]string, error) {
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

	progress := o.getOperationRollingProgress(scheduler, totalPods, status)

	status["progress"] = strconv.FormatFloat(progress, 'f', 2, 64)
	return status, nil
}
