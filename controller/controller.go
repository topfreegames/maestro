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
	"strings"
	"time"

	redisLock "github.com/bsm/redis-lock"
	goredis "github.com/go-redis/redis"
	uuid "github.com/satori/go.uuid"
	clockinterfaces "github.com/topfreegames/extensions/clock/interfaces"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/extensions/redis"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	maestroErrors "github.com/topfreegames/maestro/errors"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/models"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

var getOptions = metav1.GetOptions{}
var deleteOptions = &metav1.DeleteOptions{}

// CreateScheduler creates a new scheduler from a yaml configuration
func CreateScheduler(
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

	if exists {
		logger.Error("namespace already exists, aborting scheduler creation")
		return fmt.Errorf(`namespace "%s" already exists`, namespace.Name)
	}

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
	err = ScaleUp(logger, roomManager, mr, db, redisClient, clientset, scheduler, configYAML.AutoScaling.Min, timeoutSec, true)
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

// UpdateScheduler updates a scheduler
func UpdateScheduler(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, scheduler *models.Scheduler) error {
	return mr.WithSegment(models.SegmentUpdate, func() error {
		return scheduler.Update(db)
	})
}

// DeleteScheduler deletes a scheduler from a yaml configuration
func DeleteScheduler(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
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
	return deleteSchedulerHelper(logger, mr, db, redisClient, clientset, scheduler, namespace, timeoutSec)
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
) error {
	l := logger.WithFields(logrus.Fields{
		"source":    "scaleUp",
		"scheduler": scheduler.Name,
		"amount":    amount,
	})
	configYAML, _ := models.NewConfigYAML(scheduler.YAML)

	existPendingPods, err := pendingPods(clientset, scheduler.Name, mr)
	if err != nil {
		return err
	}
	if existPendingPods {
		return errors.New("there are pending pods, check if there are enough CPU and memory to allocate new rooms")
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

	err = waitForPods(
		willTimeoutAt.Sub(time.Now()),
		clientset,
		configYAML.Name,
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
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
	scheduler *models.Scheduler,
	amount, timeoutSec int,
) error {
	l := logger.WithFields(logrus.Fields{
		"source":    "scaleDown",
		"scheduler": scheduler.Name,
		"amount":    amount,
	})

	willTimeoutAt := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	l.Debug("accessing redis")
	//TODO: check redis version and use SPopN if version is >= 3.2, since SPopN is O(1)
	roomSet := make(map[*goredis.StringCmd]bool)
	readyKey := models.GetRoomStatusSetRedisKey(scheduler.Name, models.StatusReady)
	pipe := redisClient.TxPipeline()
	for i := 0; i < amount; i++ {
		// It is not guaranteed that maestro will delete 'amount' rooms
		// SPop returns a random room name that can already selected
		roomSet[pipe.SPop(readyKey)] = true
	}
	err := mr.WithSegment(models.SegmentPipeExec, func() error {
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

	configYaml, err := models.NewConfigYAML(scheduler.YAML)
	if err != nil {
		return err
	}

	for _, roomName := range idleRooms {
		err := roomManager.Delete(logger, mr, clientset, redisClient, configYaml, roomName, reportersConstants.ReasonScaleDown)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			logger.WithField("roomName", roomName).WithError(err).Error("error deleting room")
			deletionErr = err
		} else {
			room := models.NewRoom(roomName, scheduler.Name)
			err = room.ClearAll(redisClient, mr)
			if err != nil {
				logger.WithField("roomName", roomName).WithError(err).Error("error removing room info from redis")
				return err
			}
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
				err := mr.WithSegment(models.SegmentPod, func() error {
					var err error
					_, err = clientset.CoreV1().Pods(scheduler.Name).Get(name, metav1.GetOptions{})
					return err
				})
				if err == nil {
					exit = false
				} else if !strings.Contains(err.Error(), "not found") {
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
) error {
	configYAML.EnsureDefaultValues()

	timeoutSec := config.GetInt("updateTimeoutSeconds")
	lockTimeoutMS := config.GetInt("watcher.lockTimeoutMs")
	lockKey := GetLockKey(config.GetString("watcher.lockKey"), configYAML.Name)
	maxVersions := config.GetInt("schedulers.versions.toKeep")

	schedulerName := configYAML.Name
	l := logger.WithFields(logrus.Fields{
		"source":    "updateSchedulerConfig",
		"scheduler": schedulerName,
	})

	l.Info("starting scheduler update")

	if maxSurge <= 0 {
		return errors.New("invalid parameter: maxsurge must be greater than 0")
	}

	// Lock watchers so they don't scale up or down and the scheduler is not
	//  overwritten with older version on database
	var lock *redisLock.Lock
	timeoutDur := time.Duration(timeoutSec) * time.Second
	willTimeoutAt := clock.Now().Add(timeoutDur)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.NewTimer(timeoutDur)
	defer timeout.Stop()

	var err error

waitForLock:
	for {
		lock, err = redisClient.EnterCriticalSection(
			redisClient.Trace(ctx),
			lockKey,
			time.Duration(lockTimeoutMS)*time.Millisecond,
			0, 0,
		)
		select {
		case <-timeout.C:
			return errors.New("timeout while wating for redis lock")
		case <-ticker.C:
			if operationManager.WasCanceled() {
				l.Warn("operation was canceled")
				return nil
			}

			if lock == nil || err != nil {
				if err != nil {
					l.WithError(err).Error("error getting watcher lock")
					return err
				} else if lock == nil {
					l.Warnf("unable to get watcher %s lock, maybe some other process has it...", schedulerName)
				}
			} else if lock.IsLocked() {
				break waitForLock
			}
		}
	}

	defer func() {
		if lock != nil {
			err := redisClient.LeaveCriticalSection(lock)
			if err != nil {
				l.WithError(err).Error("error retrieving lock. Either wait of remove it manually from redis")
			}
		}
	}()

	operationManager.SetDescription("running")

	var scheduler *models.Scheduler
	var oldConfig models.ConfigYAML
	if schedulerOrNil != nil {
		scheduler = schedulerOrNil
		err = yaml.Unmarshal([]byte(scheduler.YAML), &oldConfig)
		if err != nil {
			return err
		}
	} else {
		scheduler, oldConfig, err = schedulerAndConfigFromName(mr, db, schedulerName)
		if err != nil {
			return err
		}
	}

	currentVersion := scheduler.Version

	changedPortRange, err := checkPortRange(&oldConfig, configYAML, l, db, redisClient.Client)
	if err != nil {
		return err
	}

	if changedPortRange || MustUpdatePods(&oldConfig, configYAML) {
		scheduler.NextMajorVersion()
		l.Info("pods must be recreated, starting process")

		var kubePods *v1.PodList
		err := mr.WithSegment(models.SegmentPod, func() error {
			var err error
			kubePods, err = clientset.CoreV1().Pods(schedulerName).List(metav1.ListOptions{
				LabelSelector: labels.Set{}.AsSelector().String(),
				FieldSelector: fields.Everything().String(),
			})
			return err
		})
		if err != nil {
			return maestroErrors.NewKubernetesError("error when listing pods", err)
		}

		podChunks := segmentPods(kubePods.Items, maxSurge)
		createdPods := []v1.Pod{}
		deletedPods := []v1.Pod{}

		for i, chunk := range podChunks {
			l.Debugf("deleting chunk %d: %v", i, names(chunk))

			newlyCreatedPods, newlyDeletedPods, timedout, canceled := replacePodsAndWait(
				l, roomManager, mr, clientset, db, redisClient.Client,
				willTimeoutAt, clock, configYAML, chunk,
				scheduler, operationManager,
			)
			createdPods = append(createdPods, newlyCreatedPods...)
			deletedPods = append(deletedPods, newlyDeletedPods...)

			if timedout || canceled {
				errMsg := "timedout waiting rooms to be replaced, rolled back"
				if canceled {
					errMsg = "operation was canceled, rolled back"
				}

				l.Debug(errMsg)
				rollErr := rollback(
					l, roomManager, mr, db, redisClient.Client, clientset,
					&oldConfig, maxSurge, 2*timeoutDur, createdPods, deletedPods,
					scheduler, currentVersion)
				if rollErr != nil {
					l.WithError(rollErr).Debug("error during update roll back")
					err = rollErr
				}
				return errors.New(errMsg)
			}
		}
	} else {
		scheduler.NextMinorVersion()
		l.Info("pods do not need to be recreated")
	}

	l.Info("updating configYaml on database")

	// Update new config on DB
	configBytes, err := yaml.Marshal(configYAML)
	if err != nil {
		return err
	}
	yamlString := string(configBytes)
	scheduler.Game = configYAML.Game
	scheduler.YAML = yamlString

	if string(oldConfig.ToYAML()) != string(configYAML.ToYAML()) {
		err = mr.WithSegment(models.SegmentUpdate, func() error {
			created, err := scheduler.UpdateVersion(db, maxVersions)
			if !created {
				return err
			}
			if err != nil {
				l.WithError(err).Error("error on operation on scheduler_verions table. But the newest one was created.")
			}
			return nil
		})
		if err != nil {
			l.WithError(err).Error("failed to update scheduler on database")
			return err
		}

		l.Info("updated configYaml on database")
	} else {
		l.Info("config yaml is the same, skipping")
	}

	return nil
}

func deleteSchedulerHelper(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
	scheduler *models.Scheduler,
	namespace *models.Namespace,
	timeoutSec int,
) error {
	var err error
	if scheduler.ID != "" {
		scheduler.State = models.StateTerminating
		scheduler.StateLastChangedAt = time.Now().Unix()
		if scheduler.LastScaleOpAt == 0 {
			scheduler.LastScaleOpAt = 1
		}
		err = mr.WithSegment(models.SegmentUpdate, func() error {
			return scheduler.Update(db)
		})
		if err != nil {
			logger.WithError(err).Error("failed to update scheduler state")
			return err
		}
	}

	configYAML, _ := models.NewConfigYAML(scheduler.YAML)
	// Delete pods and wait for graceful termination before deleting the namespace
	err = mr.WithSegment(models.SegmentPod, func() error {
		return namespace.DeletePods(clientset, redisClient, scheduler)
	})
	if err != nil {
		logger.WithError(err).Error("failed to delete namespace pods")
		return err
	}
	timeoutPods := time.NewTimer(time.Duration(2*configYAML.ShutdownTimeout) * time.Second)
	defer timeoutPods.Stop()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	time.Sleep(10 * time.Nanosecond) //This negligible sleep avoids race condition
	exit := false
	for !exit {
		select {
		case <-timeoutPods.C:
			return errors.New("timeout deleting scheduler pods")
		case <-ticker.C:
			var pods *v1.PodList
			listErr := mr.WithSegment(models.SegmentPod, func() error {
				var err error
				pods, err = clientset.CoreV1().Pods(scheduler.Name).List(metav1.ListOptions{})
				return err
			})
			if listErr != nil {
				logger.WithError(listErr).Error("error listing pods")
			} else if len(pods.Items) == 0 {
				exit = true
			}
			logger.Debug("deleting scheduler pods")
		}
	}

	err = mr.WithSegment(models.SegmentNamespace, func() error {
		return namespace.Delete(clientset)
	})
	if err != nil {
		logger.WithError(err).Error("failed to delete namespace while deleting scheduler")
		return err
	}
	timeoutNamespace := time.NewTimer(time.Duration(timeoutSec) * time.Second)
	defer timeoutNamespace.Stop()

	time.Sleep(10 * time.Nanosecond) //This negligible sleep avoids race condition
	exit = false
	for !exit {
		select {
		case <-timeoutNamespace.C:
			return errors.New("timeout deleting namespace")
		default:
			exists, existsErr := namespace.Exists(clientset)
			if existsErr != nil {
				logger.WithError(existsErr).Error("error checking namespace existence")
			} else if !exists {
				exit = true
			}
			logger.Debug("deleting scheduler pods")
			time.Sleep(time.Duration(1) * time.Second)
		}
	}

	// Delete from DB must be the last operation because
	// if kubernetes failed to delete pods, watcher will recreate
	// and keep the last state
	err = mr.WithSegment(models.SegmentDelete, func() error {
		return scheduler.Delete(db)
	})
	if err != nil {
		logger.WithError(err).Error("failed to delete scheduler from database while deleting scheduler")
		return err
	}

	return nil
}

// MustUpdatePods returns true if it's necessary to delete old pod and create a new one
//  so this have the new configuration.
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
	)
}

func schedulerAndConfigFromName(
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	schedulerName string,
) (
	*models.Scheduler,
	models.ConfigYAML,
	error,
) {
	var configYaml models.ConfigYAML
	scheduler := models.NewScheduler(schedulerName, "", "")
	err := mr.WithSegment(models.SegmentSelect, func() error {
		return scheduler.Load(db)
	})
	if err != nil {
		return nil, configYaml, maestroErrors.NewDatabaseError(err)
	}

	// Check if scheduler to Update exists indeed
	if scheduler.YAML == "" {
		msg := fmt.Sprintf("scheduler %s not found, create it first", schedulerName)
		return nil, configYaml, maestroErrors.NewValidationFailedError(errors.New(msg))
	}
	err = yaml.Unmarshal([]byte(scheduler.YAML), &configYaml)
	if err != nil {
		return nil, configYaml, err
	}
	return scheduler, configYaml, nil
}

// ScaleScheduler scale up or down, depending on what parameters are passed
func ScaleScheduler(
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
	timeoutScaleup, timeoutScaledown int,
	amountUp, amountDown, replicas uint,
	schedulerName string,
) error {
	var err error

	if amountUp > 0 && amountDown > 0 || replicas > 0 && amountUp > 0 || replicas > 0 && amountDown > 0 {
		return errors.New("invalid scale parameter: can't handle more than one parameter")
	}

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
			redisClient,
			clientset,
			scheduler,
			int(amountUp),
			timeoutScaleup,
			false,
		)
	} else if amountDown > 0 {
		logger.Infof("manually scaling down scheduler %s in %d GRUs", schedulerName, amountDown)
		err = ScaleDown(
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
		logger.Infof("manually scaling scheduler %s to  %d GRUs", schedulerName, replicas)
		var pods *v1.PodList
		err := mr.WithSegment(models.SegmentPod, func() error {
			var err error
			pods, err = clientset.CoreV1().Pods(schedulerName).List(metav1.ListOptions{
				LabelSelector: labels.Set{}.AsSelector().String(),
				FieldSelector: fields.Everything().String(),
			})
			return err
		})
		if err != nil {
			msg := fmt.Sprintf("error listing pods for scheduler %s", schedulerName)
			return maestroErrors.NewKubernetesError(msg, err)
		}

		nPods := uint(len(pods.Items))
		logger.Debugf("current number of pods: %d", nPods)

		if replicas > nPods {
			err = ScaleUp(
				logger,
				roomManager,
				mr,
				db,
				redisClient,
				clientset,
				scheduler,
				int(replicas-nPods),
				timeoutScaleup,
				false,
			)
		} else if replicas < nPods {
			err = ScaleDown(
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
	status string,
	config *viper.Viper,
	room *models.Room,
	schedulerCache *models.SchedulerCache,
) error {
	log := logger.WithFields(logrus.Fields{
		"operation": "SetRoomStatus",
	})

	cachedScheduler, err := schedulerCache.LoadScheduler(db, room.SchedulerName, true)
	if err != nil {
		return err
	}
	roomsCountByStatus, err := room.SetStatus(redisClient, db, mr, status, cachedScheduler.ConfigYAML, status == models.StatusOccupied)
	if err != nil {
		return err
	}

	if status != models.StatusOccupied {
		return nil
	}

	limitManager := models.NewLimitManager(logger, redisClient, config)

	limit := roomsCountByStatus.Total() * cachedScheduler.ConfigYAML.AutoScaling.Up.Trigger.Limit
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
