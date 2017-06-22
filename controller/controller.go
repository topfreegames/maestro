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
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	redisLock "github.com/bsm/redis-lock"
	goredis "github.com/go-redis/redis"
	uuid "github.com/satori/go.uuid"
	clockinterfaces "github.com/topfreegames/extensions/clock/interfaces"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/extensions/redis"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	maestroErrors "github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/models"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

// CreateScheduler creates a new scheduler from a yaml configuration
func CreateScheduler(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, redisClient redisinterfaces.RedisClient, clientset kubernetes.Interface, configYAML *models.ConfigYAML, timeoutSec int) error {
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
		deleteErr := deleteSchedulerHelper(logger, mr, db, clientset, scheduler, namespace, timeoutSec)
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
		deleteErr := deleteSchedulerHelper(logger, mr, db, clientset, scheduler, namespace, timeoutSec)
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
		deleteErr := deleteSchedulerHelper(logger, mr, db, clientset, scheduler, namespace, timeoutSec)
		if deleteErr != nil {
			err = deleteErr
		}
		return err
	}
	return nil
}

// CreateNamespaceIfNecessary creates a namespace in kubernetes if it does not exist
func CreateNamespaceIfNecessary(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, clientset kubernetes.Interface, schedulerName string) error {
	namespace := models.NewNamespace(schedulerName)
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
		logger.WithField("name", schedulerName).Debug("namespace already exists")
		return nil
	}
	logger.WithField("name", schedulerName).Info("creating namespace in kubernetes")
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
func DeleteScheduler(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, clientset kubernetes.Interface, schedulerName string, timeoutSec int) error {
	scheduler := models.NewScheduler(schedulerName, "", "")
	err := mr.WithSegment(models.SegmentSelect, func() error {
		return scheduler.Load(db)
	})
	if err != nil {
		return err
	}
	namespace := models.NewNamespace(scheduler.Name)
	return deleteSchedulerHelper(logger, mr, db, clientset, scheduler, namespace, timeoutSec)
}

// GetSchedulerScalingInfo returns the scheduler scaling policies and room count by status
func GetSchedulerScalingInfo(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, client redisinterfaces.RedisClient, schedulerName string) (*models.Scheduler, *models.AutoScaling, *models.RoomsStatusCount, error) {
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

// DeleteRoomsNoPingSince delete rooms where lastPing < since
func DeleteRoomsNoPingSince(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, redisClient redisinterfaces.RedisClient, clientset kubernetes.Interface, schedulerName string, since int64) error {
	var roomNames []string
	err := mr.WithSegment(models.SegmentZRangeBy, func() error {
		var err error
		roomNames, err = models.GetRoomsNoPingSince(redisClient, schedulerName, since)
		return err
	})
	if err != nil {
		logger.WithError(err).Error("error listing rooms no ping since")
		return err
	}
	if len(roomNames) == 0 {
		logger.Debug("no rooms need to be deleted")
		return nil
	}
	for _, roomName := range roomNames {
		err := deleteServiceAndPod(logger, mr, clientset, schedulerName, roomName)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			logger.WithFields(logrus.Fields{"roomName": roomName, "function": "DeleteRoomsNoPingSince"}).WithError(err).Error("error deleting room")
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

// DeleteRoomsOccupiedTimeout delete rooms that have occupied status for more than timeout time
func DeleteRoomsOccupiedTimeout(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
	schedulerName string,
	since int64,
) error {
	var roomNames []string
	var err error
	err = mr.WithSegment(models.SegmentZRangeBy, func() error {
		roomNames, err = models.GetRoomsOccupiedTimeout(redisClient, schedulerName, since)
		return err
	})
	if err != nil {
		logger.WithError(err).Error("error listing rooms occupied timeout")
		return err
	}
	if len(roomNames) == 0 {
		return nil
	}
	for _, roomName := range roomNames {
		err := deleteServiceAndPod(logger, mr, clientset, schedulerName, roomName)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			logger.WithFields(logrus.Fields{"roomName": roomName, "function": "DeleteRoomsOccupiedTimeout"}).WithError(err).Error("error deleting room")
		} else {
			room := models.NewRoom(roomName, schedulerName)
			err = room.ClearAll(redisClient)
			if err != nil {
				logger.WithFields(logrus.Fields{"roomName": roomName, "function": "DeleteRoomsOccupiedTimeout"}).WithError(err).Error("error removing room info from redis")
			}
		}
	}
	return nil
}

// ScaleUp scales up a scheduler using its config
func ScaleUp(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, redisClient redisinterfaces.RedisClient, clientset kubernetes.Interface, scheduler *models.Scheduler, amount, timeoutSec int, initalOp bool) error {
	l := logger.WithFields(logrus.Fields{
		"source":    "scaleUp",
		"scheduler": scheduler.Name,
		"amount":    amount,
	})
	configYAML, _ := models.NewConfigYAML(scheduler.YAML)

	pods := make([]string, amount)
	var creationErr error

	willTimeoutAt := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	for i := 0; i < amount; i++ {
		podName, err := createServiceAndPod(l, mr, redisClient, clientset, configYAML, scheduler.Name)
		if err != nil {
			l.WithError(err).Error("scale up error")
			if initalOp {
				return err
			}
			creationErr = err
		}
		pods[i] = podName
	}

	if willTimeoutAt.Sub(time.Now()) < 0 {
		return errors.New("timeout scaling up scheduler")
	}

	err := waitForPods(
		willTimeoutAt.Sub(time.Now()),
		clientset,
		amount,
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
		err := deleteServiceAndPod(logger, mr, clientset, scheduler.Name, roomName)
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
	timeout := time.NewTimer(willTimeoutAt.Sub(time.Now())).C

	for {
		exit := true
		select {
		case <-timeout:
			return errors.New("timeout scaling down scheduler")
		default:
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
			time.Sleep(time.Duration(1) * time.Second)
		}
		if exit {
			l.Info("finished scaling down scheduler")
			break
		}
	}

	return deletionErr
}

// UpdateSchedulerConfig updates a Scheduler with new image, scale info, etc
// Old pods and services are deleted and recreated with new config
// Scale up and down are locked
func UpdateSchedulerConfig(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	clientset kubernetes.Interface,
	configYAML *models.ConfigYAML,
	timeoutSec, lockTimeoutMS int,
	lockKey string,
	clock clockinterfaces.Clock,
) error {
	schedulerName := configYAML.Name
	l := logger.WithFields(logrus.Fields{
		"source":    "updateSchedulerConfig",
		"scheduler": schedulerName,
	})

	// Check if scheduler to Update exists indeed
	scheduler := models.NewScheduler(schedulerName, "", "")
	err := mr.WithSegment(models.SegmentSelect, func() error {
		return scheduler.Load(db)
	})
	if err != nil {
		l.Debug("update error", err)
		return maestroErrors.NewDatabaseError(err)
	}
	if scheduler.YAML == "" {
		l.Debug("scheduler not found")
		msg := fmt.Sprintf("scheduler %s not found, create it first", schedulerName)
		return maestroErrors.NewValidationFailedError(errors.New(msg))
	}

	var oldConfig models.ConfigYAML
	err = yaml.Unmarshal([]byte(scheduler.YAML), &oldConfig)
	if err != nil {
		l.Debug("invalid config saved on db", err)
		return err
	}

	var lock *redisLock.Lock
	if MustUpdatePods(&oldConfig, configYAML) {
		// Lock watchers so they don't scale up and down
		timeoutDur := time.Duration(timeoutSec) * time.Second
		willTimeoutAt := clock.Now().Add(timeoutDur)
		lock, err = redisClient.EnterCriticalSection(redisClient.Client, lockKey, time.Duration(lockTimeoutMS)*time.Millisecond, 0, 0)
		ticker := time.NewTicker(500 * time.Millisecond)
		timeout := time.NewTimer(timeoutDur)
	waitForLock:
		for {
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
		ticker.Stop()
		defer func() {
			if lock != nil {
				redisClient.LeaveCriticalSection(lock)
			}
		}()

		// Delete old rooms
		listOptions := metav1.ListOptions{
			LabelSelector: labels.Set{}.AsSelector().String(),
			FieldSelector: fields.Everything().String(),
		}
		svcs, err := clientset.CoreV1().Services(schedulerName).List(listOptions)
		if err != nil {
			return maestroErrors.NewKubernetesError("error when listing  services", err)
		}

		timeout = time.NewTimer(willTimeoutAt.Sub(clock.Now()))
		time.Sleep(1 * time.Millisecond) // This sleep avoids race condition between select goroutine and timer goroutine
		numberOldPods := len(svcs.Items)
		logger.WithField("numberOfRooms", numberOldPods).Info("deleting old rooms")
		i := 0
		for i < numberOldPods {
			svc := svcs.Items[i]
			select {
			case <-timeout.C:
				err = errors.New("timeout during room deletion")
				return maestroErrors.NewKubernetesError("error when deleting old rooms. Maestro will scale up, if necessary, with previous room configuration.", err)
			default:
				err := deleteServiceAndPod(logger, mr, clientset, schedulerName, svc.GetName())
				if err != nil {
					logger.WithField("roomName", svc.GetName()).WithError(err).Error("error deleting room")
					time.Sleep(1 * time.Second)
				} else {
					i = i + 1
					room := models.NewRoom(svc.GetName(), schedulerName)
					err := room.ClearAll(redisClient.Client)
					if err != nil {
						//TODO: try again so Redis doesn't hold trash data
						logger.WithField("roomName", svc.GetName()).WithError(err).Error("error removing room info from redis")
					}
				}
			}
		}

		if numberOldPods < configYAML.AutoScaling.Min {
			numberOldPods = configYAML.AutoScaling.Min
		}
		logger.WithField("numberOfRooms", numberOldPods).Info("recreating rooms")

		pods := make([]string, numberOldPods)
		numberNewPods := 0
		timeout = time.NewTimer(willTimeoutAt.Sub(clock.Now()))
		ticker = time.NewTicker(1 * time.Second)

		// Creates pods as they are deleted
		for numberNewPods < numberOldPods {
			select {
			case <-timeout.C:
				for _, podToDelete := range pods {
					deleteServiceAndPod(logger, mr, clientset, schedulerName, podToDelete)
				}
				err = errors.New("timeout during new room creation")
				return maestroErrors.NewKubernetesError("error when creating new rooms. Maestro will scale up, if necessary, with previous room configuration.", err)
			case <-ticker.C:
				k8sPods, err := clientset.CoreV1().Pods(schedulerName).List(listOptions)
				if err != nil {
					logger.WithError(err).Error("error when getting services")
					continue
				}

				numberCurrentPods := len(k8sPods.Items)
				numberPodsToCreate := numberOldPods - numberCurrentPods

				for i := 0; i < numberPodsToCreate; i++ {
					name, err := createServiceAndPod(l, mr, redisClient.Client, clientset, configYAML, schedulerName)
					if err != nil {
						logger.WithError(err).Error("error when creating pod and service")
						continue
					}
					pods[numberNewPods] = name
					numberNewPods = numberNewPods + 1
				}
			}
		}

		ticker.Stop()
		if willTimeoutAt.Sub(clock.Now()) < 0 {
			return errors.New("timeout scaling up scheduler")
		}

		// Wait for pods to be created
		err = waitForPods(
			willTimeoutAt.Sub(clock.Now()),
			clientset,
			numberNewPods,
			configYAML.Name,
			pods,
			l,
			mr,
		)
		if err != nil {
			logger.WithError(err).Error("error when updating scheduler and waiting for new pods to run")
			for _, podToDelete := range pods {
				deleteServiceAndPod(l, mr, clientset, configYAML.Name, podToDelete)
			}
			return err
		}
	}

	// Update new config on DB
	configBytes, err := yaml.Marshal(configYAML)
	if err != nil {
		return err
	}
	yamlString := string(configBytes)
	scheduler.YAML = yamlString
	err = mr.WithSegment(models.SegmentUpdate, func() error {
		return scheduler.Update(db)
	})

	return err
}

func deleteSchedulerHelper(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, clientset kubernetes.Interface, scheduler *models.Scheduler, namespace *models.Namespace, timeoutSec int) error {
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
		return namespace.DeletePods(clientset)
	})
	if err != nil {
		logger.WithError(err).Error("failed to delete namespace pods")
		return err
	}
	timeoutPods := make(chan bool, 1)
	go func() {
		time.Sleep(time.Duration(2*configYAML.ShutdownTimeout) * time.Second)
		timeoutPods <- true
	}()
	time.Sleep(10 * time.Nanosecond) //This negligible sleep avoids race condition
	exit := false
	for !exit {
		select {
		case <-timeoutPods:
			return errors.New("timeout deleting scheduler pods")
		default:
			pods, listErr := clientset.CoreV1().Pods(scheduler.Name).List(metav1.ListOptions{})
			if listErr != nil {
				logger.WithError(listErr).Error("error listing pods")
			} else if len(pods.Items) == 0 {
				exit = true
			}
			logger.Debug("deleting scheduler pods")
			time.Sleep(time.Duration(1) * time.Second)
		}
	}

	err = mr.WithSegment(models.SegmentNamespace, func() error {
		return namespace.Delete(clientset)
	})
	if err != nil {
		logger.WithError(err).Error("failed to delete namespace while deleting scheduler")
		return err
	}
	timeoutNamespace := make(chan bool, 1)
	go func() {
		time.Sleep(time.Duration(timeoutSec) * time.Second)
		timeoutNamespace <- true
	}()
	time.Sleep(10 * time.Nanosecond) //This negligible sleep avoids race condition
	exit = false
	for !exit {
		select {
		case <-timeoutNamespace:
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
	// if kubernetes failed to delete service and pods, watcher will recreate
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

func buildNodePortEnvVars(ports []v1.ServicePort) []*models.EnvVar {
	nodePortEnvVars := make([]*models.EnvVar, len(ports))
	for idx, port := range ports {
		nodePortEnvVars[idx] = &models.EnvVar{
			Name:  fmt.Sprintf("MAESTRO_NODE_PORT_%d_%s", port.Port, port.Protocol),
			Value: strconv.FormatInt(int64(port.NodePort), 10),
		}
	}
	return nodePortEnvVars
}

func createServiceAndPod(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, redisClient redisinterfaces.RedisClient, clientset kubernetes.Interface, configYAML *models.ConfigYAML, schedulerName string) (string, error) {
	randID := strings.SplitN(uuid.NewV4().String(), "-", 2)[0]
	name := fmt.Sprintf("%s-%s", configYAML.Name, randID)
	room := models.NewRoom(name, schedulerName)
	err := mr.WithSegment(models.SegmentInsert, func() error {
		return room.Create(redisClient)
	})
	if err != nil {
		return "", err
	}
	service := models.NewService(name, configYAML.Name, configYAML.Ports)
	var kubeService *v1.Service
	err = mr.WithSegment(models.SegmentService, func() error {
		kubeService, err = service.Create(clientset)
		return err
	})
	if err != nil {
		return "", err
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
	env := append(configYAML.Env, namesEnvVars...)
	nodePortEnvVars := buildNodePortEnvVars(kubeService.Spec.Ports)
	env = append(env, nodePortEnvVars...)
	pod := models.NewPod(
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
	)
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
		return "", err
	}
	nodeName := kubePod.Spec.NodeName
	logger.WithFields(logrus.Fields{
		"node": nodeName,
		"name": name,
	}).Info("Created GRU (service and pod) successfully.")
	return name, nil
}

func deleteServiceAndPod(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, clientset kubernetes.Interface, schedulerName, name string) error {
	service := models.NewService(name, schedulerName, []*models.Port{})
	err := mr.WithSegment(models.SegmentService, func() error {
		return service.Delete(clientset)
	})
	if err != nil {
		return err
	}
	pod := models.NewPod("", "", name, schedulerName, nil, nil, 0, []*models.Port{}, []string{}, []*models.EnvVar{})
	err = mr.WithSegment(models.SegmentPod, func() error {
		return pod.Delete(clientset)
	})
	if err != nil {
		return err
	}
	return nil
}

// MustUpdatePods returns true if it's necessary to delete old pod and create a new one
//  so this have the new configuration.
func MustUpdatePods(old, new *models.ConfigYAML) bool {
	samePorts := func(ps1, ps2 []*models.Port) bool {
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

	switch {
	case old.Image != new.Image:
		return true
	case !samePorts(old.Ports, new.Ports):
		return true
	case old.Limits != nil && new.Limits != nil && *old.Limits != *new.Limits:
		return true
	case old.Requests != nil && new.Requests != nil && *old.Requests != *new.Requests:
		return true
	case !sameCmd(old.Cmd, new.Cmd):
		return true
	case !sameEnv(old.Env, new.Env):
		return true
	default:
		return false
	}
}

func waitForPods(
	timeout time.Duration,
	clientset kubernetes.Interface,
	numberOfPods int,
	namespace string,
	pods []string,
	l logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
) error {
	timeoutChan := time.NewTimer(timeout).C
	tickerChan := time.NewTicker(1 * time.Second).C

	for {
		exit := true
		select {
		case <-timeoutChan:
			return errors.New("timeout waiting for rooms to be created")
		case <-tickerChan:
			for i := 0; i < numberOfPods; i++ {
				if pods[i] != "" {
					pod, err := clientset.CoreV1().Pods(namespace).Get(pods[i], metav1.GetOptions{})
					if err != nil {
						l.WithError(err).Error("scale up pod error")
						exit = false
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
