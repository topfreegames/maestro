// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package controller

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	goredis "github.com/go-redis/redis"
	uuid "github.com/satori/go.uuid"
	clockinterfaces "github.com/topfreegames/extensions/clock/interfaces"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/extensions/redis"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	maestroErrors "github.com/topfreegames/maestro/errors"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	"github.com/topfreegames/maestro/storage"
	yaml "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes"
)

var getOptions = metav1.GetOptions{}
var deleteOptions = &metav1.DeleteOptions{}

// CreateScheduler creates a new scheduler from a yaml configuration
func CreateScheduler(
	config *viper.Viper,
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
	configYAML *models.ConfigYAML,
	timeoutSec int,
) (err error) {
	logger = logger.WithField("operation", "controller.CreateScheduler")

	configYAML.EnsureDefaultValues()

	// min number of rooms cannot be greater than max
	if configYAML.AutoScaling.Max > 0 && configYAML.AutoScaling.Min > configYAML.AutoScaling.Max {
		logger.Error("autoscaling min is greater than max")
		return fmt.Errorf("autoscaling min is greater than max")
	}

	// if using resource scaling (cpu, mem) requests must be set
	err = validateMetricsTrigger(configYAML, logger)
	if err != nil {
		return err
	}

	logger.Info("unmarshalling config yaml")
	configBytes, err := yaml.Marshal(configYAML)
	if err != nil {
		return err
	}
	yamlString := string(configBytes)

	// by creating the namespace first we ensure it is available in kubernetes
	namespace := models.NewNamespace(configYAML.Name)

	logger.Info("checking if namespace exists")
	var exists bool
	err = mr.WithSegment(models.SegmentNamespace, func() error {
		exists, err = namespace.Exists(clientset)
		return err
	})
	if err != nil {
		logger.WithError(err).Error("error accessing namespace of Kubernetes")
		return err
	}

	if !exists {
		err = mr.WithSegment(models.SegmentNamespace, func() error {
			return namespace.Create(clientset)
		})

		if err != nil {
			logger.WithError(err).Error("error creating namespace")

			deleteErr := mr.WithSegment(models.SegmentNamespace, func() error {
				return namespace.Delete(clientset)
			})
			if deleteErr != nil {
				logger.WithError(err).Error("error deleting namespace")
				return deleteErr
			}
			return err
		}
	}

	scheduler := models.NewScheduler(configYAML.Name, configYAML.Game, yamlString)
	err = mr.WithSegment(models.SegmentInsert, func() error {
		return scheduler.Create(db)
	})

	if err != nil {
		logger.WithError(err).Error("error creating scheduler on database")
		deleteErr := deleteSchedulerHelper(logger, mr, db, redisClient, clientset, scheduler, namespace, timeoutSec)
		if deleteErr != nil {
			logger.WithError(err).Error("deleting scheduler after error")
			err = deleteErr
		}
		return err
	}

	logger.Info("creating ports pool if necessary")
	usesPortRange, err := checkPortRange(nil, configYAML, logger, db, redisClient)
	if err != nil {
		logger.WithError(err).Error("error checking port range, deleting scheduler")
		deleteErr := deleteSchedulerHelper(logger, mr, db, redisClient, clientset, scheduler, namespace, timeoutSec)
		if deleteErr != nil {
			logger.WithError(err).Error("error deleting scheduler after check port range error")
		}
		return err
	}

	logger.Info("creating pods of new scheduler")
	err = ScaleUp(logger, roomManager, mr, db, redisClient, clientset, scheduler, configYAML.AutoScaling.Min, timeoutSec, true, config, false)
	if err != nil {
		logger.WithError(err).Error("error scaling up scheduler, deleting it")
		if scheduler.LastScaleOpAt == int64(0) {
			// this prevents error: null value in column \"last_scale_op_at\" violates not-null constraint
			scheduler.LastScaleOpAt = int64(1)
		}
		deleteErr := deleteSchedulerHelper(logger, mr, db, redisClient, clientset, scheduler, namespace, timeoutSec)
		if deleteErr != nil {
			logger.WithError(err).Error("error deleting scheduler after scale up error")
			return deleteErr
		}

		if usesPortRange {
			logger.Info("deleting newly created ports pool due to scheduler creation error")
		}

		return err
	}

	scheduler.State = models.StateInSync
	scheduler.StateLastChangedAt = time.Now().Unix()
	scheduler.LastScaleOpAt = time.Now().Unix()
	err = mr.WithSegment(models.SegmentUpdate, func() error {
		return scheduler.Update(db)
	})
	if err != nil {
		deleteErr := deleteSchedulerHelper(logger, mr, db, redisClient, clientset, scheduler, namespace, timeoutSec)
		if deleteErr != nil {
			logger.WithError(err).Error("error deleting scheduler after database update error")
			return deleteErr
		}
		return err
	}

	return nil
}

// CreateNamespaceIfNecessary creates a namespace in kubernetes if it does not exist
func CreateNamespaceIfNecessary(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	clientset kubernetes.Interface,
	scheduler *models.Scheduler,
) error {
	if scheduler.State == models.StateTerminating {
		return nil
	}

	namespace := models.NewNamespace(scheduler.Name)
	var err error
	var exists bool
	err = mr.WithSegment(models.SegmentNamespace, func() error {
		exists, err = namespace.Exists(clientset)
		return err
	})
	if err != nil {
		return err
	}
	if exists {
		logger.WithField("name", scheduler.Name).Debug("namespace already exists")
		return nil
	}
	logger.WithField("name", scheduler.Name).Info("creating namespace in kubernetes")
	return mr.WithSegment(models.SegmentNamespace, func() error {
		return namespace.Create(clientset)
	})
}

// UpdateSchedulerState updates a scheduler state
func UpdateSchedulerState(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, scheduler *models.Scheduler) error {
	return mr.WithSegment(models.SegmentUpdate, func() error {
		return scheduler.UpdateState(db)
	})
}

// UpdateScheduler updates a scheduler
func UpdateScheduler(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, scheduler *models.Scheduler) error {
	return mr.WithSegment(models.SegmentUpdate, func() error {
		return scheduler.Update(db)
	})
}

// DeleteScheduler deletes a scheduler from a yaml configuration
func DeleteScheduler(
	ctx context.Context,
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	clientset kubernetes.Interface,
	config *viper.Viper,
	schedulerName string,
	timeoutSec int,
) error {
	scheduler := models.NewScheduler(schedulerName, "", "")
	err := mr.WithSegment(models.SegmentSelect, func() error {
		return scheduler.Load(db)
	})
	if err != nil {
		return err
	}
	if scheduler.YAML == "" {
		return maestroErrors.NewValidationFailedError(
			fmt.Errorf("scheduler \"%s\" not found", schedulerName),
		)
	}
	namespace := models.NewNamespace(scheduler.Name)

	// lock so autoscaler doesn't recreate rooms deleted
	terminationLockKey := models.GetSchedulerTerminationLockKey(config.GetString("watcher.lockKey"), schedulerName)
	terminationLock, _, err := AcquireLock(
		ctx,
		logger,
		redisClient,
		config,
		nil,
		terminationLockKey,
		schedulerName,
	)

	if err != nil {
		logger.WithError(err).Error("failed to acquire lock")
		return err
	}

	defer ReleaseLock(
		logger,
		redisClient,
		terminationLock,
		schedulerName,
	)

	return deleteSchedulerHelper(logger, mr, db, redisClient.Trace(ctx), clientset, scheduler, namespace, timeoutSec)
}

// GetSchedulerScalingInfo returns the scheduler scaling policies and room count by status
func GetSchedulerScalingInfo(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	client redisinterfaces.RedisClient,
	schedulerName string,
) (
	*models.Scheduler,
	*models.AutoScaling,
	*models.RoomsStatusCount,
	error,
) {
	scheduler := models.NewScheduler(schedulerName, "", "")
	err := mr.WithSegment(models.SegmentSelect, func() error {
		return scheduler.Load(db)
	})

	if err != nil {
		return nil, nil, nil, err
	} else if scheduler.YAML == "" {
		return nil, nil, nil, maestroErrors.NewValidationFailedError(
			fmt.Errorf("scheduler \"%s\" not found", schedulerName),
		)
	}
	scalingPolicy := scheduler.GetAutoScalingPolicy()
	var roomCountByStatus *models.RoomsStatusCount
	err = mr.WithSegment(models.SegmentGroupBy, func() error {
		roomCountByStatus, err = models.GetRoomsCountByStatus(client, scheduler.Name)
		return err
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return scheduler, scalingPolicy, roomCountByStatus, nil
}

// ListSchedulersNames list all scheduler names
func ListSchedulersNames(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB) ([]string, error) {
	var names []string
	err := mr.WithSegment(models.SegmentSelect, func() error {
		var err error
		names, err = models.ListSchedulersNames(db)
		return err
	})
	if err != nil {
		return []string{}, err
	}
	return names, nil
}

// DeleteUnavailableRooms delete rooms that are not available
func DeleteUnavailableRooms(
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
	scheduler *models.Scheduler,
	roomNames []string,
	reason string,
) error {
	if len(roomNames) == 0 {
		logger.Debug("no rooms need to be deleted")
		return nil
	}

	configYaml, err := models.NewConfigYAML(scheduler.YAML)
	if err != nil {
		return err
	}

	logger.Info("deleting unvavailable pods")
	for _, roomName := range roomNames {
		err := roomManager.Delete(logger, mr, clientset, redisClient, configYaml, roomName, reason)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			logger.WithFields(logrus.Fields{"roomName": roomName, "function": "DeleteUnavailableRooms"}).WithError(err).Error("error deleting room")
		} else {
			if err != nil {
				logger.WithFields(logrus.Fields{"roomName": roomName, "function": "DeleteUnavailableRooms"}).WithError(err).Error("pod was not found, deleting room from redis")
			}

			room := models.NewRoom(roomName, scheduler.Name)
			err = room.ClearAll(redisClient, mr)
			if err != nil {
				logger.WithField("roomName", roomName).WithError(err).Error("error removing room info from redis")
			}
		}
	}
	logger.Info("successfully deleted unavailable pods")

	return nil
}

// ScaleUp scales up a scheduler using its config
func ScaleUp(
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
	scheduler *models.Scheduler,
	amount, timeoutSec int,
	initalOp bool,
	config *viper.Viper,
	shouldRespectLimit bool,
) error {
	scaleUpLimit := 0
	if shouldRespectLimit {
		scaleUpLimit = config.GetInt("watcher.maxScaleUpAmount")
	}
	l := logger.WithFields(logrus.Fields{
		"source":       "scaleUp",
		"scheduler":    scheduler.Name,
		"amount":       amount,
		"scaleUpLimit": scaleUpLimit,
	})

	configYAML, err := models.NewConfigYAML(scheduler.YAML)
	if err != nil {
		return fmt.Errorf("failed to load scheduler configuration: %w", err)
	}

	notReadyPods, err := hasNotReadyPods(config, redisClient, scheduler.Name, configYAML.PreventRoomsCreationWithError, mr)
	if err != nil {
		return fmt.Errorf("failed to list pending or failed pods: %w", err)
	}

	if notReadyPods {
		return errors.New("scheduler has not ready pods (pending or with error)")
	}

	amount, err = SetScalingAmount(
		logger,
		mr,
		db,
		redisClient,
		scheduler,
		configYAML.AutoScaling.Max,
		configYAML.AutoScaling.Min,
		amount,
		false,
	)
	if err != nil {
		return err
	}

	// SAFETY - hard cap on scale up amount in order to not break etcd
	if shouldRespectLimit && amount > scaleUpLimit {
		l.Warnf("amount to scale up is higher than limit")
		amount = scaleUpLimit
	}

	pods := make([]*v1.Pod, amount)
	var creationErr error
	willTimeoutAt := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	j := 0
	for i := 0; i < amount; i++ {
		pod, err := roomManager.Create(l, mr, redisClient, db, clientset, configYAML, scheduler)
		if err != nil {
			l.WithError(err).Error("scale up error")
			if initalOp {
				return err
			}
			creationErr = err
		}
		pods[i] = pod
		j = j + 1
		if j >= amount/10 && j >= 10 {
			j = 0
			time.Sleep(100 * time.Millisecond)
		}
	}

	if willTimeoutAt.Sub(time.Now()) <= 0 {
		return errors.New("timeout scaling up scheduler")
	}

	rand.Seed(time.Now().UnixNano())
	err = waitForPods(
		willTimeoutAt.Sub(time.Now()),
		clientset,
		redisClient,
		configYAML,
		pods,
		l,
		mr,
	)
	if err != nil {
		return err
	}

	return creationErr
}

// ScaleDown scales down the number of rooms
func ScaleDown(
	ctx context.Context,
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	clientset kubernetes.Interface,
	scheduler *models.Scheduler,
	amount, timeoutSec int,
) error {
	l := logger.WithFields(logrus.Fields{
		"source":    "scaleDown",
		"scheduler": scheduler.Name,
		"amount":    amount,
	})

	redisClientWithContext := redisClient.Trace(ctx)
	willTimeoutAt := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	l.Debug("accessing redis")
	//TODO: check redis version and use SPopN if version is >= 3.2, since SPopN is O(1)
	roomSet := make(map[*goredis.StringCmd]bool)
	readyKey := models.GetRoomStatusSetRedisKey(scheduler.Name, models.StatusReady)
	pipe := redisClientWithContext.TxPipeline()
	configYAML, err := models.NewConfigYAML(scheduler.YAML)
	if err != nil {
		return err
	}

	amount, err = SetScalingAmount(
		logger,
		mr,
		db,
		redisClientWithContext,
		scheduler,
		configYAML.AutoScaling.Max,
		configYAML.AutoScaling.Min,
		amount,
		true,
	)
	if err != nil {
		return err
	}

	for i := 0; i < amount; i++ {
		// It is not guaranteed that maestro will delete 'amount' rooms
		// SPop returns a random room name that can already selected
		roomSet[pipe.SPop(readyKey)] = true
	}
	l.Debugf("popped %d ready rooms to scale down", amount)
	err = mr.WithSegment(models.SegmentPipeExec, func() error {
		var err error
		_, err = pipe.Exec()
		return err
	})
	if err != nil {
		l.WithError(err).WithFields(logrus.Fields{
			"amount": int64(amount),
			"key":    models.GetRoomStatusSetRedisKey(scheduler.Name, models.StatusReady),
		}).Error("scale down error, failed to retrieve ready rooms from redis")
		return err
	}
	idleRooms := []string{}
	i := 0
	for strCmd := range roomSet {
		idleRooms = append(idleRooms, strCmd.Val())
		i = i + 1
	}

	for i, key := range idleRooms {
		pieces := strings.Split(key, ":")
		name := pieces[len(pieces)-1]
		idleRooms[i] = name
	}

	var deletionErr error

	for _, roomName := range idleRooms {
		err := roomManager.Delete(logger, mr, clientset, redisClientWithContext, configYAML, roomName, reportersConstants.ReasonScaleDown)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			logger.WithField("roomName", roomName).WithError(err).Error("error deleting room")
			deletionErr = err
		}
	}

	if willTimeoutAt.Sub(time.Now()) < 0 {
		return errors.New("timeout scaling down scheduler")
	}
	timeout := time.NewTimer(willTimeoutAt.Sub(time.Now()))
	defer timeout.Stop()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		exit := true
		select {
		case <-timeout.C:
			return errors.New("timeout scaling down scheduler")
		case <-ticker.C:
			for _, name := range idleRooms {
				var pod *models.Pod
				err := mr.WithSegment(models.SegmentPod, func() error {
					var err error
					pod, err = models.GetPodFromRedis(redisClientWithContext, mr, name, scheduler.Name)
					return err
				})
				if err == nil && pod != nil {
					if pod.IsTerminating {
						logger.WithField("pod", pod.Name).Debugf("pod is terminating")
						exit = false
						continue
					}

					logger.WithField("pod", pod.Name).Debugf("pod still exists, deleting again")
					err := roomManager.Delete(logger, mr, clientset, redisClientWithContext, configYAML, pod.Name, reportersConstants.ReasonScaleDown)
					if err != nil && !strings.Contains(err.Error(), "not found") {
						logger.WithField("roomName", pod.Name).WithError(err).Error("error deleting room")
						deletionErr = err
						exit = false
					} else if err != nil && strings.Contains(err.Error(), "not found") {
						logger.WithField("pod", pod.Name).Debugf("pod already deleted")
					}

					if err == nil {
						exit = false
					}
				} else if pod != nil {
					l.WithError(err).Error("scale down pod error")
					exit = false
				}
			}
			l.Debug("scaling down scheduler...")
		}
		if exit {
			l.Info("finished scaling down scheduler")
			break
		}
	}

	return deletionErr
}

// UpdateSchedulerConfig updates a Scheduler with new image, scale info, etc
// Old pods are deleted and recreated with new config
// Scale up and down are locked
func UpdateSchedulerConfig(
	ctx context.Context,
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	clientset kubernetes.Interface,
	configYAML *models.ConfigYAML,
	maxSurge int,
	clock clockinterfaces.Clock,
	schedulerOrNil *models.Scheduler,
	config *viper.Viper,
	operationManager *models.OperationManager,
	eventsStorage storage.SchedulerEventStorage,
) error {
	configYAML.EnsureDefaultValues()
	schedulerName := configYAML.Name
	l := logger.WithFields(logrus.Fields{
		"source":    "updateSchedulerConfig",
		"scheduler": schedulerName,
	})

	l.Info("starting scheduler update")

	err := validateConfig(logger, configYAML, maxSurge)
	if err != nil {
		return err
	}

	// Lock updates on scheduler during all the process
	configLockKey := models.GetSchedulerConfigLockKey(config.GetString("watcher.lockKey"), configYAML.Name)
	configLock, canceled, err := AcquireLock(
		ctx,
		logger,
		redisClient,
		config,
		operationManager,
		configLockKey,
		schedulerName,
	)

	defer ReleaseLock(
		logger,
		redisClient,
		configLock,
		schedulerName,
	)

	if err != nil {
		return err
	}

	if canceled {
		return nil
	}

	operationManager.SetDescription(models.OpManagerRunning)

	scheduler, oldConfig, err := LoadScheduler(
		mr,
		db,
		schedulerOrNil,
		schedulerName,
	)
	if err != nil {
		return err
	}

	changedPortRange, err := checkPortRange(&oldConfig, configYAML, l, db, redisClient.Client)
	if err != nil {
		return err
	}

	oldVersion := scheduler.Version
	shouldRecreatePods := changedPortRange || MustUpdatePods(&oldConfig, configYAML)

	if shouldRecreatePods {
		l.Info("pods must be recreated, starting process")
		scheduler.NextMajorVersion()
	} else {
		l.Info("pods do not need to be recreated")
		scheduler.NextMinorVersion()
	}

	scheduler.RollingUpdateStatus = inProgressStatus
	err = saveConfigYAML(
		ctx,
		logger,
		mr,
		db,
		configYAML,
		scheduler,
		oldConfig,
		config,
		"",
	)
	if err != nil {
		return err
	}

	persistEventErr := eventsStorage.PersistSchedulerEvent(
		models.NewSchedulerEvent(
			models.StartUpdateEventName, schedulerName,
			map[string]interface{}{
				models.SchedulerVersionMetadataName: scheduler.Version,
			},
		),
	)
	if persistEventErr != nil {
		l.WithError(persistEventErr).Warn("failed to persist update started event")
	}

	if shouldRecreatePods {
		var status map[string]string

		// wait for watcher.EnsureCorrectRooms to rolling update the pods
		for err == nil {
			status, err = operationManager.GetOperationStatus(mr, *scheduler)
			if err != nil {
				logger.WithError(err).Warn("errored trying to get rolling update progress")
				err = nil
				continue
			}

			// operation canceled
			if status == nil || len(status) <= 0 {
				scheduler.RollingUpdateStatus = canceledStatus
				err = errors.New("operation canceled")
				break
			}

			// operation returned error
			if status["description"] == models.OpManagerErrored {
				err = errors.New(status["error"])
				scheduler.RollingUpdateStatus = erroredStatus(err.Error())
				break
			}

			// operation timedout
			if status["description"] == models.OpManagerTimedout {
				scheduler.RollingUpdateStatus = timedoutStatus
				err = errors.New("operation timedout")
				break
			}

			progress, _ := strconv.ParseFloat(status["progress"], 64)

			if progress > 99.9 {
				break
			}
			time.Sleep(time.Second * 1)
		}

		// delete invalidRooms key as EnsureCorrectRooms finished
		models.RemoveInvalidRoomsKey(redisClient.Client, mr, schedulerName)

		if err != nil {
			persistEventErr = eventsStorage.PersistSchedulerEvent(
				models.NewSchedulerEvent(
					models.FailedUpdateEventName,
					schedulerName,
					map[string]interface{}{
						models.ErrorMetadataName: err,
					},
				),
			)
			if persistEventErr != nil {
				l.WithError(persistEventErr).Warn("failed to persist update failed event")
			}

			persistEventErr := eventsStorage.PersistSchedulerEvent(
				models.NewSchedulerEvent(
					models.TriggerRollbackEventName,
					schedulerName,
					map[string]interface{}{
						models.SchedulerVersionMetadataName: oldVersion,
					},
				),
			)
			if persistEventErr != nil {
				l.WithError(persistEventErr).Warn("failed to persist rollback triggered event")
			}

			l.WithError(err).Error("error during UpdateSchedulerConfig. Rolling back database")
			dbRollbackErr := DBRollback(
				ctx,
				logger,
				mr,
				db,
				redisClient,
				configYAML,
				&oldConfig,
				clock,
				scheduler,
				config,
				oldVersion,
			)

			if dbRollbackErr != nil {
				l.WithError(dbRollbackErr).Error("error during scheduler database roll back")
			}

			return err
		}
	}

	scheduler.RollingUpdateStatus = deployedStatus
	updateErr := scheduler.UpdateVersionStatus(db)
	if updateErr != nil {
		l.WithError(updateErr).Errorf("error updating scheduler_version status to %s", scheduler.RollingUpdateStatus)
	}

	persistEventErr = eventsStorage.PersistSchedulerEvent(
		models.NewSchedulerEvent(models.FinishedUpdateEventName, schedulerName, map[string]interface{}{}),
	)
	if persistEventErr != nil {
		logger.WithError(persistEventErr).Warn("failed to persist worker update finished event")
	}

	return nil
}

// MustUpdatePods returns true if it's necessary to delete old pod and create a new one
//
//	so this have the new configuration.
func MustUpdatePods(old, new *models.ConfigYAML) bool {
	if old.Version() == "v1" && new.Version() == "v2" && len(new.Containers) != 1 {
		return true
	}

	if old.Version() == "v2" && new.Version() == "v1" && len(old.Containers) != 1 {
		return true
	}

	if old.Version() == "v2" && new.Version() == "v2" && len(old.Containers) != len(new.Containers) {
		return true
	}

	samePorts := func(ps1, ps2 []*models.Port) bool {
		if len(ps1) != len(ps2) {
			return false
		}
		set := make(map[models.Port]bool)
		for _, p1 := range ps1 {
			set[*p1] = true
		}

		for _, p2 := range ps2 {
			if _, contains := set[*p2]; !contains {
				return false
			}
		}
		return true
	}

	sameEnv := func(envs1, envs2 []*models.EnvVar) bool {
		if len(envs1) != len(envs2) {
			return false
		}
		set := make(map[models.EnvVar]bool)
		for _, env1 := range envs1 {
			set[*env1] = true
		}
		for _, env2 := range envs2 {
			if _, contains := set[*env2]; !contains {
				return false
			}
		}
		return true
	}

	sameCmd := func(cmd1, cmd2 []string) bool {
		if len(cmd1) != len(cmd2) {
			return false
		}
		for i, cmd := range cmd1 {
			if cmd != cmd2[i] {
				return false
			}
		}
		return true
	}

	// per container avaliations goes here
	mustUpdate := func(old, new models.ContainerIface) bool {
		switch {
		case old.GetImage() != new.GetImage():
			return true
		case !samePorts(old.GetPorts(), new.GetPorts()):
			return true
		case old.GetLimits() != nil && new.GetLimits() != nil && *old.GetLimits() != *new.GetLimits():
			return true
		case old.GetRequests() != nil && new.GetRequests() != nil && *old.GetRequests() != *new.GetRequests():
			return true
		case !sameCmd(old.GetCmd(), new.GetCmd()):
			return true
		case !sameEnv(old.GetEnv(), new.GetEnv()):
			return true
		case new.GetName() != old.GetName():
			return true
		default:
			return false
		}
	}

	// per pod avaliation goes here
	if old.NodeAffinity != new.NodeAffinity {
		return true
	} else if old.NodeToleration != new.NodeToleration {
		return true
	}

	if old.Version() == "v1" && new.Version() == "v1" {
		return mustUpdate(old, new)
	}

	for _, oldContainer := range old.Containers {
		for _, newContainer := range new.Containers {
			if oldContainer.Name == newContainer.Name {
				if mustUpdate(oldContainer, newContainer) {
					return true
				}

				break
			}
		}
	}

	return false
}

// UpdateSchedulerImage is a UpdateSchedulerConfig sugar that updates only the image
func UpdateSchedulerImage(
	ctx context.Context,
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	clientset kubernetes.Interface,
	schedulerName string,
	imageParams *models.SchedulerImageParams,
	maxSurge int,
	clock clockinterfaces.Clock,
	config *viper.Viper,
	operationManager *models.OperationManager,
	eventsStorage storage.SchedulerEventStorage,
) error {
	scheduler, configYaml, err := schedulerAndConfigFromName(mr, db, schedulerName)
	if err != nil {
		return err
	}

	updated, err := configYaml.UpdateImage(imageParams)
	if err != nil {
		return err
	}
	if !updated {
		logger.Infof("scheduler was not updated because the image is the same: %s", imageParams.Image)
		return nil
	}

	return UpdateSchedulerConfig(
		ctx,
		logger,
		roomManager,
		mr,
		db,
		redisClient,
		clientset,
		&configYaml,
		maxSurge,
		clock,
		scheduler,
		config,
		operationManager,
		eventsStorage,
	)
}

// UpdateSchedulerMin is a UpdateSchedulerConfig sugar that updates only the image
func UpdateSchedulerMin(
	ctx context.Context,
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	schedulerName string,
	schedulerMin int,
	clock clockinterfaces.Clock,
	config *viper.Viper,
	operationManager *models.OperationManager,
	eventsStorage storage.SchedulerEventStorage,
) error {
	scheduler, configYaml, err := schedulerAndConfigFromName(mr, db, schedulerName)
	if err != nil {
		return err
	}
	if configYaml.AutoScaling.Min == schedulerMin {
		return nil
	}
	configYaml.AutoScaling.Min = schedulerMin
	return UpdateSchedulerConfig(
		ctx,
		logger,
		roomManager,
		mr,
		db,
		redisClient,
		nil,
		&configYaml,
		100,
		clock,
		scheduler,
		config,
		operationManager,
		eventsStorage,
	)
}

// ScaleScheduler scale up or down, depending on what parameters are passed
func ScaleScheduler(
	ctx context.Context,
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	clientset kubernetes.Interface,
	config *viper.Viper,
	timeoutScaleup, timeoutScaledown int,
	amountUp, amountDown, replicas uint,
	schedulerName string,
) error {
	var err error

	if amountUp > 0 && amountDown > 0 || replicas > 0 && amountUp > 0 || replicas > 0 && amountDown > 0 {
		return errors.New("invalid scale parameter: can't handle more than one parameter")
	}

	redisClientWithContext := redisClient.Trace(ctx)

	scheduler := models.NewScheduler(schedulerName, "", "")
	err = scheduler.Load(db)
	if err != nil {
		logger.WithError(err).Errorf("error loading scheduler %s from db", schedulerName)
		return maestroErrors.NewDatabaseError(err)
	} else if scheduler.YAML == "" {
		msg := fmt.Sprintf("scheduler '%s' not found", schedulerName)
		logger.Errorf(msg)
		return errors.New(msg)
	}

	if amountUp > 0 {
		logger.Infof("manually scaling up scheduler %s in %d GRUs", schedulerName, amountUp)
		err = ScaleUp(
			logger,
			roomManager,
			mr,
			db,
			redisClientWithContext,
			clientset,
			scheduler,
			int(amountUp),
			timeoutScaleup,
			false,
			config,
			false,
		)
	} else if amountDown > 0 {
		logger.Infof("manually scaling down scheduler %s in %d GRUs", schedulerName, amountDown)

		// lock so autoscaler doesn't recreate rooms deleted
		downscalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), scheduler.Name)
		downscalingLock, _, err := AcquireLock(
			ctx,
			logger,
			redisClient,
			config,
			nil,
			downscalingLockKey,
			scheduler.Name,
		)
		defer ReleaseLock(
			logger,
			redisClient,
			downscalingLock,
			scheduler.Name,
		)
		if err != nil {
			logger.WithError(err).Error("not able to acquire downScalingLock. Not scaling down")
			return err
		}

		err = ScaleDown(
			ctx,
			logger,
			roomManager,
			mr,
			db,
			redisClient,
			clientset,
			scheduler,
			int(amountDown),
			timeoutScaledown,
		)
	} else {
		logger.Infof("manually scaling scheduler %s to %d GRUs", schedulerName, replicas)
		// get list of actual pods
		podCount, err := models.GetPodCountFromRedis(redisClientWithContext, mr, schedulerName)
		if err != nil {
			return err
		}

		nPods := uint(podCount)
		logger.Debugf("current number of pods: %d", nPods)

		if replicas > nPods {
			err = ScaleUp(
				logger,
				roomManager,
				mr,
				db,
				redisClientWithContext,
				clientset,
				scheduler,
				int(replicas-nPods),
				timeoutScaleup,
				false,
				config,
				false,
			)
		} else if replicas < nPods {
			// lock so autoscaler doesn't recreate rooms deleted
			downscalingLockKey := models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), scheduler.Name)
			downscalingLock, _, err := AcquireLock(
				ctx,
				logger,
				redisClient,
				config,
				nil,
				downscalingLockKey,
				scheduler.Name,
			)
			defer ReleaseLock(
				logger,
				redisClient,
				downscalingLock,
				scheduler.Name,
			)
			if err != nil {
				logger.WithError(err).Error("not able to acquire downScalingLock. Not scaling down")
				return err
			}

			err = ScaleDown(
				ctx,
				logger,
				roomManager,
				mr,
				db,
				redisClient,
				clientset,
				scheduler,
				int(nPods-replicas),
				timeoutScaledown,
			)
		}
	}
	return nil
}

// SetRoomStatus updates room status and scales up if necessary
func SetRoomStatus(
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	redisClient redisinterfaces.RedisClient,
	db pginterfaces.DB,
	mr *models.MixedMetricsReporter,
	clientset kubernetes.Interface,
	roomPayload *models.RoomStatusPayload,
	config *viper.Viper,
	room *models.Room,
	schedulerCache *models.SchedulerCache,
) error {
	log := logger.WithFields(logrus.Fields{
		"operation": "SetRoomStatus",
		"status":    roomPayload.Status,
	})

	log.Debug("Updating room status")

	cachedScheduler, err := schedulerCache.LoadScheduler(db, room.SchedulerName, true)
	if err != nil {
		return err
	}
	roomsCountByStatus, err := room.SetStatus(
		redisClient, db, mr,
		roomPayload, cachedScheduler.Scheduler)
	if err != nil {
		return err
	}

	if roomPayload.Status != models.StatusOccupied || !cachedScheduler.ConfigYAML.AutoScaling.EnablePanicScale {
		return nil
	}

	limitManager := models.NewLimitManager(logger, redisClient, config)

	limit := roomsCountByStatus.Available() * cachedScheduler.ConfigYAML.AutoScaling.Up.Trigger.Limit
	occupied := 100 * roomsCountByStatus.Occupied
	if occupied > limit {
		isLocked, err := limitManager.IsLocked(room)
		if err != nil {
			return err
		}

		if !isLocked {
			scaleUpLog := log.WithFields(logrus.Fields{
				"scheduler":  room.SchedulerName,
				"readyRooms": roomsCountByStatus.Ready,
				"id":         uuid.NewV4().String(),
			})
			scaleUpLog.Info("few ready rooms, scaling up")
			go func() {
				err := mr.WithSegment(models.SegmentPipeExec, func() error {
					return limitManager.Lock(room, cachedScheduler.ConfigYAML)
				})
				if err != nil {
					log.WithError(err).Error(err)
					return
				}
				defer limitManager.Unlock(room)

				err = ScaleUp(
					logger,
					roomManager,
					mr,
					db,
					redisClient,
					clientset,
					cachedScheduler.Scheduler,
					cachedScheduler.ConfigYAML.AutoScaling.Up.Delta,
					config.GetInt("scaleUpTimeoutSeconds"),
					false,
					config,
					false,
				)
				if err != nil {
					log.WithError(err).Error(err)
					return
				}
				scaleUpLog.Info("finished scaling up")
			}()
			return nil
		}
	}

	return nil
}
