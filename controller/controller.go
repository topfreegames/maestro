// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package controller

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	redisLock "github.com/bsm/redis-lock"
	goredis "github.com/go-redis/redis"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/viper"
	clockinterfaces "github.com/topfreegames/extensions/clock/interfaces"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/extensions/redis"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	maestroErrors "github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/models"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

var getOptions = metav1.GetOptions{}

// CreateScheduler creates a new scheduler from a yaml configuration
func CreateScheduler(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
	configYAML *models.ConfigYAML,
	timeoutSec int,
) error {
	configYAML.EnsureDefaultValues()

	configBytes, err := yaml.Marshal(configYAML)
	if err != nil {
		return err
	}
	yamlString := string(configBytes)

	// by creating the namespace first we ensure it is available in kubernetes
	namespace := models.NewNamespace(configYAML.Name)

	var exists bool
	err = mr.WithSegment(models.SegmentNamespace, func() error {
		exists, err = namespace.Exists(clientset)
		return err
	})
	if err != nil {
		return err
	}

	if exists {
		return fmt.Errorf(`namespace "%s" already exists`, namespace.Name)
	}

	err = mr.WithSegment(models.SegmentNamespace, func() error {
		return namespace.Create(clientset)
	})

	if err != nil {
		deleteErr := mr.WithSegment(models.SegmentNamespace, func() error {
			return namespace.Delete(clientset)
		})
		if deleteErr != nil {
			err = deleteErr
		}
		return err
	}

	scheduler := models.NewScheduler(configYAML.Name, configYAML.Game, yamlString)
	err = mr.WithSegment(models.SegmentInsert, func() error {
		return scheduler.Create(db)
	})

	if err != nil {
		deleteErr := deleteSchedulerHelper(logger, mr, db, redisClient, clientset, scheduler, namespace, timeoutSec)
		if deleteErr != nil {
			err = deleteErr
		}
		return err
	}

	err = ScaleUp(logger, mr, db, redisClient, clientset, scheduler, configYAML.AutoScaling.Min, timeoutSec, true)
	if err != nil {
		if scheduler.LastScaleOpAt == int64(0) {
			// this prevents error: null value in column \"last_scale_op_at\" violates not-null constraint
			scheduler.LastScaleOpAt = int64(1)
		}
		deleteErr := deleteSchedulerHelper(logger, mr, db, redisClient, clientset, scheduler, namespace, timeoutSec)
		if deleteErr != nil {
			return deleteErr
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
			err = deleteErr
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
	mr *models.MixedMetricsReporter,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
	schedulerName, gameName string,
	roomNames []string,
	reason string,
) error {
	if len(roomNames) == 0 {
		logger.Debug("no rooms need to be deleted")
		return nil
	}
	for _, roomName := range roomNames {
		err := deletePod(logger, mr, clientset, redisClient, schedulerName, gameName, roomName, reason)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			logger.WithFields(logrus.Fields{"roomName": roomName, "function": "DeleteUnavailableRooms"}).WithError(err).Error("error deleting room")
		} else {
			room := models.NewRoom(roomName, schedulerName)
			err = room.ClearAll(redisClient)
			if err != nil {
				logger.WithField("roomName", roomName).WithError(err).Error("error removing room info from redis")
			}
		}
	}

	return nil
}

func pendingPods(
	clientset kubernetes.Interface,
	namespace string,
) (bool, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set{}.AsSelector().String(),
		FieldSelector: fields.Everything().String(),
	}
	pods, err := clientset.CoreV1().Pods(namespace).List(listOptions)
	if err != nil {
		return false, maestroErrors.NewKubernetesError("error when listing pods", err)
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodPending {
			return true, nil
		}
	}

	return false, nil
}

// ScaleUp scales up a scheduler using its config
func ScaleUp(
	logger logrus.FieldLogger,
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

	existPendingPods, err := pendingPods(clientset, scheduler.Name)
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
		pod, err := createPod(l, mr, redisClient, db, clientset, configYAML)
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
func ScaleDown(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, redisClient redisinterfaces.RedisClient, clientset kubernetes.Interface, scheduler *models.Scheduler, amount, timeoutSec int) error {
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
	_, err := pipe.Exec()
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
		err := deletePod(logger, mr, clientset, redisClient, scheduler.Name, scheduler.Game, roomName, reportersConstants.ReasonScaleDown)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			logger.WithField("roomName", roomName).WithError(err).Error("error deleting room")
			deletionErr = err
		} else {
			room := models.NewRoom(roomName, scheduler.Name)
			err = room.ClearAll(redisClient)
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
				_, err := clientset.CoreV1().Pods(scheduler.Name).Get(name, metav1.GetOptions{})
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
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	clientset kubernetes.Interface,
	configYAML *models.ConfigYAML,
	maxSurge int,
	clock clockinterfaces.Clock,
	schedulerOrNil *models.Scheduler,
	config *viper.Viper,
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
			redisClient.Client,
			lockKey,
			time.Duration(lockTimeoutMS)*time.Millisecond,
			0, 0,
		)
		select {
		case <-timeout.C:
			return errors.New("timeout while wating for redis lock")
		case <-ticker.C:
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
			redisClient.LeaveCriticalSection(lock)
		}
	}()

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

	if MustUpdatePods(&oldConfig, configYAML) {
		l.Info("pods must be recreated, starting process")

		kubePods, err := clientset.CoreV1().Pods(schedulerName).List(metav1.ListOptions{
			LabelSelector: labels.Set{}.AsSelector().String(),
			FieldSelector: fields.Everything().String(),
		})
		if err != nil {
			return maestroErrors.NewKubernetesError("error when listing pods", err)
		}

		podChunks := segmentPods(kubePods.Items, maxSurge)
		createdPods := []v1.Pod{}
		deletedPods := []v1.Pod{}

		for i, chunk := range podChunks {
			l.Debugf("deleting chunk %d: %v", i, names(chunk))

			newlyCreatedPods, newlyDeletedPods, timedout := replacePodsAndWait(
				l, mr, clientset, db, redisClient.Client,
				willTimeoutAt, clock, configYAML, chunk,
			)
			createdPods = append(createdPods, newlyCreatedPods...)
			deletedPods = append(deletedPods, newlyDeletedPods...)

			if timedout {
				l.Debug("update timed out, rolling back")
				rollErr := rollback(
					l, mr, db, redisClient.Client, clientset,
					&oldConfig, maxSurge, 2*timeoutDur, createdPods, deletedPods)
				if rollErr != nil {
					l.WithError(rollErr).Debug("error during update roll back")
					err = rollErr
				}
				return errors.New("timedout waiting rooms to be replaced, rolled back")
			}
		}
	} else {
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
	err = mr.WithSegment(models.SegmentNamespace, func() error {
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
			pods, listErr := clientset.CoreV1().Pods(scheduler.Name).List(metav1.ListOptions{})
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

func createPod(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	redisClient redisinterfaces.RedisClient,
	db pginterfaces.DB,
	clientset kubernetes.Interface,
	configYAML *models.ConfigYAML,
) (*v1.Pod, error) {
	randID := strings.SplitN(uuid.NewV4().String(), "-", 2)[0]
	name := fmt.Sprintf("%s-%s", configYAML.Name, randID)
	room := models.NewRoom(name, configYAML.Name)
	err := mr.WithSegment(models.SegmentInsert, func() error {
		return room.Create(redisClient, db, mr, configYAML)
	})
	if err != nil {
		return nil, err
	}
	namesEnvVars := []*models.EnvVar{
		{
			Name:  "MAESTRO_SCHEDULER_NAME",
			Value: configYAML.Name,
		},
		{
			Name:  "MAESTRO_ROOM_ID",
			Value: name,
		},
	}

	var pod *models.Pod
	if configYAML.Version() == "v1" {
		env := append(configYAML.Env, namesEnvVars...)
		pod, err = models.NewPod(
			configYAML.Game,
			configYAML.Image,
			name,
			configYAML.Name,
			configYAML.Limits,
			configYAML.Requests, // TODO: requests should be <= limits calculate it
			configYAML.ShutdownTimeout,
			configYAML.Ports,
			configYAML.Cmd,
			env,
			clientset,
			redisClient,
		)
	} else if configYAML.Version() == "v2" {
		containers := make([]*models.Container, len(configYAML.Containers))
		for i, container := range configYAML.Containers {
			containers[i] = container.NewWithCopiedEnvs()
			containers[i].Env = append(containers[i].Env, namesEnvVars...)
		}

		pod, err = models.NewPodWithContainers(
			configYAML.Game,
			name,
			configYAML.Name,
			configYAML.ShutdownTimeout,
			containers,
			clientset,
			redisClient,
		)
	}
	if err != nil {
		return nil, err
	}

	if configYAML.NodeAffinity != "" {
		pod.SetAffinity(configYAML.NodeAffinity)
	}
	if configYAML.NodeToleration != "" {
		pod.SetToleration(configYAML.NodeToleration)
	}

	var kubePod *v1.Pod
	err = mr.WithSegment(models.SegmentPod, func() error {
		kubePod, err = pod.Create(clientset)
		return err
	})
	if err != nil {
		return nil, err
	}
	nodeName := kubePod.Spec.NodeName
	logger.WithFields(logrus.Fields{
		"node": nodeName,
		"name": name,
	}).Info("Created GRU (pod) successfully.")

	return kubePod, nil
}

func deletePod(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
	schedulerName, gameName, name, reason string,
) error {
	pod, err := models.NewPod(gameName, "", name, schedulerName, nil, nil, 0, []*models.Port{}, []string{}, []*models.EnvVar{}, clientset, redisClient)
	err = mr.WithSegment(models.SegmentPod, func() error {
		return pod.Delete(clientset, redisClient, reason)
	})
	if err != nil {
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

func waitForPods(
	timeout time.Duration,
	clientset kubernetes.Interface,
	namespace string,
	pods []*v1.Pod,
	l logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
) error {
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		exit := true
		select {
		case <-timeoutTimer.C:
			return errors.New("timeout waiting for rooms to be created")
		case <-ticker.C:
			for i := range pods {
				if pods[i] != nil {
					pod, err := clientset.CoreV1().Pods(namespace).Get(pods[i].GetName(), metav1.GetOptions{})
					if err != nil {
						//The pod does not exist (not even on Pending or ContainerCreating state), so create again
						exit = false
						l.WithError(err).Infof("error creating pod %s, recreating...", pods[i].GetName())
						pods[i].ResourceVersion = ""
						_, err = clientset.CoreV1().Pods(namespace).Create(pods[i])
						if err != nil {
							l.WithError(err).Errorf("error recreating pod %s", pods[i].GetName())
						}
					} else {
						if len(pod.Status.Phase) == 0 {
							break // TODO: HACK!!!  Trying to detect if we are running unit tests
						}
						if pod.Status.Phase != v1.PodRunning {
							exit = false
							break
						}
						for _, containerStatus := range pod.Status.ContainerStatuses {
							if !containerStatus.Ready {
								exit = false
								break
							}
						}
					}
				}
			}
			l.Debug("scaling scheduler...")
		}
		if exit {
			l.Info("finished scaling scheduler")
			break
		}
	}

	return nil
}

// UpdateSchedulerImage is a UpdateSchedulerConfig sugar that updates only the image
func UpdateSchedulerImage(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	clientset kubernetes.Interface,
	schedulerName string,
	imageParams *models.SchedulerImageParams,
	maxSurge int,
	clock clockinterfaces.Clock,
	config *viper.Viper,
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
		logger,
		mr,
		db,
		redisClient,
		clientset,
		&configYaml,
		maxSurge,
		clock,
		scheduler,
		config,
	)
}

// UpdateSchedulerMin is a UpdateSchedulerConfig sugar that updates only the image
func UpdateSchedulerMin(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	schedulerName string,
	schedulerMin int,
	clock clockinterfaces.Clock,
	config *viper.Viper,
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
		logger,
		mr,
		db,
		redisClient,
		nil,
		&configYaml,
		100,
		clock,
		scheduler,
		config,
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
		pods, err := clientset.CoreV1().Pods(schedulerName).List(metav1.ListOptions{
			LabelSelector: labels.Set{}.AsSelector().String(),
			FieldSelector: fields.Everything().String(),
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

	if status == models.StatusOccupied {
		if roomsCountByStatus.Total()*cachedScheduler.ConfigYAML.AutoScaling.Up.Trigger.Limit < 100*roomsCountByStatus.Occupied {
			scaleUpLog := log.WithFields(logrus.Fields{
				"scheduler":  room.SchedulerName,
				"readyRooms": roomsCountByStatus.Ready,
				"id":         uuid.NewV4().String(),
			})
			scaleUpLog.Info("few ready rooms, scaling up")
			go func() {
				err := ScaleUp(
					logger,
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
