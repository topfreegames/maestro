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
func CreateScheduler(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
	configYAML *models.ConfigYAML,
	timeoutSec int,
) error {
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
	namespace := models.NewNamespace(scheduler.Name)
	return deleteSchedulerHelper(logger, mr, db, redisClient, clientset, scheduler, namespace, timeoutSec)
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

// DeleteUnavailableRooms delete rooms that are not available
func DeleteUnavailableRooms(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
	schedulerName string,
	roomNames []string,
) error {
	if len(roomNames) == 0 {
		logger.Debug("no rooms need to be deleted")
		return nil
	}
	for _, roomName := range roomNames {
		err := deletePod(logger, mr, clientset, redisClient, schedulerName, roomName)
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
func ScaleUp(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, redisClient redisinterfaces.RedisClient, clientset kubernetes.Interface, scheduler *models.Scheduler, amount, timeoutSec int, initalOp bool) error {
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
		pod, err := createPod(l, mr, redisClient, clientset, configYAML, scheduler.Name)
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
		err := deletePod(logger, mr, clientset, redisClient, scheduler.Name, roomName)
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
// Old pods are deleted and recreated with new config
// Scale up and down are locked
func UpdateSchedulerConfig(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	clientset kubernetes.Interface,
	configYAML *models.ConfigYAML,
	timeoutSec, lockTimeoutMS, maxSurge int,
	lockKey string,
	clock clockinterfaces.Clock,
	schedulerOrNil *models.Scheduler,
) error {
	schedulerName := configYAML.Name
	l := logger.WithFields(logrus.Fields{
		"source":    "updateSchedulerConfig",
		"scheduler": schedulerName,
	})

	if maxSurge <= 0 {
		return errors.New("invalid parameter: maxsurge must be greater than 0")
	}

	var scheduler *models.Scheduler
	var oldConfig models.ConfigYAML
	var err error
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

	denominator := 100 / maxSurge
	sum := func(start, total, denominator int) int {
		delta := total / denominator
		if delta == 0 {
			delta = 1
		}
		start = start + delta
		if start > total {
			start = total
		}
		return start
	}

	var lock *redisLock.Lock
	if MustUpdatePods(&oldConfig, configYAML) {
		// Lock watchers so they don't scale up and down
		timeoutDur := time.Duration(timeoutSec) * time.Second
		willTimeoutAt := clock.Now().Add(timeoutDur)
		ticker := time.NewTicker(2 * time.Second)
		timeout := time.NewTimer(timeoutDur)
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
		ticker.Stop()
		defer func() {
			if lock != nil {
				redisClient.LeaveCriticalSection(lock)
			}
		}()

		kubePods, err := clientset.CoreV1().Pods(schedulerName).List(metav1.ListOptions{
			LabelSelector: labels.Set{}.AsSelector().String(),
			FieldSelector: fields.Everything().String(),
		})
		if err != nil {
			return maestroErrors.NewKubernetesError("error when listing pods", err)
		}

		totalPodsToDelete := len(kubePods.Items)
		logger.WithField("numberOfRooms", totalPodsToDelete).Info("recreating rooms")

		totalPodsToCreate := totalPodsToDelete
		if totalPodsToCreate < configYAML.AutoScaling.Min {
			totalPodsToCreate = configYAML.AutoScaling.Min
		}

		startCreate := 0
		startDelete := 0

		endCreate := sum(startCreate, totalPodsToCreate, denominator)
		endDelete := sum(startDelete, totalPodsToDelete, denominator)

		newPods := []*v1.Pod{}
		timeout = time.NewTimer(willTimeoutAt.Sub(clock.Now()))

		for startDelete < totalPodsToDelete {
			// This sleep avoids race condition between select goroutine and timer goroutine
			time.Sleep(1 * time.Millisecond)
			podsToDelete := kubePods.Items[startDelete:endDelete]

			logger.WithField("numberOfRooms", endDelete-startDelete).Info("deleting old rooms")
			for _, pod := range podsToDelete {
				select {
				case <-timeout.C:
					err = errors.New("timeout during room deletion")
					msg := "error when deleting old rooms. Maestro will scale up, if necessary, with previous room configuration"
					return maestroErrors.NewKubernetesError(msg, err)
				default:
					err := deletePod(logger, mr, clientset, redisClient.Client, schedulerName, pod.GetName())
					if err != nil {
						logger.WithField("roomName", pod.GetName()).WithError(err).Error("error deleting room")
						time.Sleep(1 * time.Second)
					} else {
						room := models.NewRoom(pod.GetName(), schedulerName)
						err := room.ClearAll(redisClient.Client)
						if err != nil {
							//TODO: try again so Redis doesn't hold trash data
							logger.WithField("roomName", pod.GetName()).WithError(err).Error("error removing room info from redis")
						}
					}
				}
			}

			createdPods := 0
			numberOfPodsToCreate := endCreate - startCreate
			timeout = time.NewTimer(willTimeoutAt.Sub(clock.Now()))
			ticker = time.NewTicker(10 * time.Millisecond)

			// Creates pods as they are deleted
			for createdPods < numberOfPodsToCreate {
				select {
				case <-timeout.C:
					for _, podToDelete := range newPods {
						err := deletePod(logger, mr, clientset, redisClient.Client, schedulerName, podToDelete.GetName())
						if err != nil {
							logger.
								WithField("roomName", podToDelete.GetName()).
								WithError(err).
								Error("update scheduler timed out and room deletion failed")
						}
						room := models.NewRoom(podToDelete.GetName(), schedulerName)
						err = room.ClearAll(redisClient.Client)
						if err != nil {
							//TODO: try again so Redis doesn't hold trash data
							logger.WithField("roomName", podToDelete).WithError(err).Error("error removing room info from redis")
						}
					}
					err = errors.New("timeout during new room creation")
					return maestroErrors.
						NewKubernetesError(
							"error when creating new rooms. Maestro will scale up, if necessary, with previous room configuration.",
							err,
						)
				case <-ticker.C:
					for i := startCreate; i < endCreate; i++ {
						var podExists bool
						if i < len(podsToDelete) {
							podToDelete := podsToDelete[i]
							podExists, err = models.PodExists(podToDelete.GetName(), podToDelete.GetNamespace(), clientset)
							if err != nil {
								logger.WithError(err).Errorf("error getting pod %s", podToDelete.GetName())
							}
						}
						if i >= len(podsToDelete) || !podExists {
							pod, err := createPod(l, mr, redisClient.Client, clientset, configYAML, schedulerName)
							if err != nil {
								logger.WithError(err).Error("error when creating pod")
								continue
							}
							newPods = append(newPods, pod)
							createdPods = createdPods + 1
						}
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
				configYAML.Name,
				newPods[startCreate:endCreate],
				l,
				mr,
			)
			if err != nil {
				logger.WithError(err).Error("error when updating scheduler and waiting for new pods to run")
				for _, podToDelete := range newPods {
					deletePod(l, mr, clientset, redisClient.Client, configYAML.Name, podToDelete.GetName())
					room := models.NewRoom(podToDelete.GetName(), schedulerName)
					err := room.ClearAll(redisClient.Client)
					if err != nil {
						//TODO: try again so Redis doesn't hold trash data
						logger.WithField("roomName", podToDelete).WithError(err).Error("error removing room info from redis")
					}
				}
				return err
			}

			startDelete = endDelete
			endDelete = sum(endDelete, totalPodsToDelete, denominator)
			startCreate = endCreate
			endCreate = sum(endCreate, totalPodsToCreate, denominator)
			if endDelete == totalPodsToDelete {
				endCreate = totalPodsToCreate
			}
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
		return namespace.DeletePods(clientset, redisClient)
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
	clientset kubernetes.Interface,
	configYAML *models.ConfigYAML,
	schedulerName string,
) (*v1.Pod, error) {
	randID := strings.SplitN(uuid.NewV4().String(), "-", 2)[0]
	name := fmt.Sprintf("%s-%s", configYAML.Name, randID)
	room := models.NewRoom(name, schedulerName)
	err := mr.WithSegment(models.SegmentInsert, func() error {
		return room.Create(redisClient)
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
	env := append(configYAML.Env, namesEnvVars...)
	pod, err := models.NewPod(
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
	schedulerName,
	name string,
) error {
	pod, err := models.NewPod("", "", name, schedulerName, nil, nil, 0, []*models.Port{}, []string{}, []*models.EnvVar{}, clientset, redisClient)
	err = mr.WithSegment(models.SegmentPod, func() error {
		return pod.Delete(clientset, redisClient)
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
	case old.NodeAffinity != new.NodeAffinity:
		return true
	case old.NodeToleration != new.NodeToleration:
		return true
	default:
		return false
	}
}

func waitForPods(
	timeout time.Duration,
	clientset kubernetes.Interface,
	namespace string,
	pods []*v1.Pod,
	l logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
) error {
	timeoutChan := time.NewTimer(timeout).C
	tickerChan := time.NewTicker(10 * time.Millisecond).C

	for {
		exit := true
		select {
		case <-timeoutChan:
			return errors.New("timeout waiting for rooms to be created")
		case <-tickerChan:
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
	schedulerName, image, lockKey string,
	timeoutSec, lockTimeoutMS, maxSurge int,
	clock clockinterfaces.Clock,
) error {
	scheduler, configYaml, err := schedulerAndConfigFromName(mr, db, schedulerName)
	if err != nil {
		return err
	}
	if configYaml.Image == image {
		return nil
	}
	configYaml.Image = image
	return UpdateSchedulerConfig(
		logger,
		mr,
		db,
		redisClient,
		clientset,
		&configYaml,
		timeoutSec, lockTimeoutMS, maxSurge,
		lockKey,
		clock,
		scheduler,
	)
}

// UpdateSchedulerMin is a UpdateSchedulerConfig sugar that updates only the image
func UpdateSchedulerMin(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	schedulerName string,
	schedulerMin int,
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
		nil,
		nil,
		&configYaml,
		0, 0, 100,
		"",
		nil,
		scheduler,
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
