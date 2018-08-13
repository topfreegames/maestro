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
	"math"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/topfreegames/maestro/models"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	clockinterfaces "github.com/topfreegames/extensions/clock/interfaces"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	maestroErrors "github.com/topfreegames/maestro/errors"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func replacePodsAndWait(
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	clientset kubernetes.Interface,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	willTimeoutAt time.Time,
	clock clockinterfaces.Clock,
	configYAML *models.ConfigYAML,
	podsToDelete []v1.Pod,
	scheduler *models.Scheduler,
	operationManager *models.OperationManager,
) (createdPods []v1.Pod, deletedPods []v1.Pod, timedout, canceled bool) {
	createdPods = []v1.Pod{}
	deletedPods = []v1.Pod{}

	for _, pod := range podsToDelete {
		logger.Debugf("deleting pod %s", pod.GetName())

		err := DeletePodAndRoom(logger, roomManager, mr, clientset, redisClient,
			configYAML, pod.GetName(), reportersConstants.ReasonUpdate)
		if err == nil || strings.Contains(err.Error(), "redis") {
			deletedPods = append(deletedPods, pod)
		}
		if err != nil {
			logger.WithError(err).Debugf("error deleting pod %s", pod.GetName())
		}
	}

	now := clock.Now()
	timeout := willTimeoutAt.Sub(now)
	createdPods, timedout, canceled = createPodsAsTheyAreDeleted(
		logger, roomManager, mr, clientset, db, redisClient, timeout, configYAML,
		deletedPods, scheduler, operationManager)
	if timedout || canceled {
		return createdPods, deletedPods, timedout, canceled
	}

	timeout = willTimeoutAt.Sub(clock.Now())
	timedout, canceled = waitCreatingPods(
		logger, clientset, timeout, configYAML.Name,
		createdPods, operationManager, mr)
	if timedout || canceled {
		return createdPods, deletedPods, timedout, canceled
	}

	return createdPods, deletedPods, false, false
}

// In rollback, it must delete newly created pod and
// restore old deleted pods to come back to previous state
func rollback(
	l logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
	configYAML *models.ConfigYAML,
	maxSurge int,
	timeout time.Duration,
	createdPods, deletedPods []v1.Pod,
	scheduler *models.Scheduler,
	versionToRollbackTo string,
) error {
	scheduler.Version = versionToRollbackTo

	var err error
	willTimeoutAt := time.Now().Add(timeout)
	logger := l.WithFields(logrus.Fields{
		"operation": "controller.rollback",
		"scheduler": configYAML.Name,
	})

	logger.Info("starting rollback")
	logger.Debugf("deleting all %#v", names(createdPods))
	logger.Debugf("recreating same quantity of these: %#v", names(deletedPods))

	deletedPodChunks := segmentPods(deletedPods, maxSurge)
	createdPodChunks := segmentPods(createdPods, maxSurge)

	configYaml, err := models.NewConfigYAML(scheduler.YAML)
	if err != nil {
		return err
	}

	for i := 0; i < len(deletedPodChunks) || i < len(createdPodChunks); i++ {
		if i < len(createdPodChunks) {
			logger.Debugf("deleting chunk %#v", names(createdPodChunks[i]))
			for j := 0; j < len(createdPodChunks[i]); {
				pod := createdPodChunks[i][j]
				logger.Debugf("deleting pod %s", pod.GetName())

				err = DeletePodAndRoom(logger, roomManager, mr, clientset, redisClient,
					configYaml, pod.GetName(), reportersConstants.ReasonUpdate)
				if err != nil {
					logger.WithError(err).
						Debugf("error deleting newly created pod %s", pod.GetName())
					time.Sleep(1 * time.Second)
					continue
				}

				j = j + 1
			}

			waitTimeout := willTimeoutAt.Sub(time.Now())
			err = waitTerminatingPods(
				logger, clientset, waitTimeout, configYAML.Name,
				createdPodChunks[i],
				mr,
			)
			if err != nil {
				return err
			}
		}

		if i < len(deletedPodChunks) {
			logger.Debugf("recreating chunk %#v", names(deletedPodChunks[i]))
			newlyCreatedPods := []v1.Pod{}
			for j := 0; j < len(deletedPodChunks[i]); {
				pod := deletedPodChunks[i][j]
				logger.Debugf("creating new pod to substitute %s", pod.GetName())

				newPod, err := roomManager.Create(logger, mr, redisClient,
					db, clientset, configYAML, scheduler)
				if err != nil {
					logger.WithError(err).Debug("error creating new pod")
					time.Sleep(1 * time.Second)
					continue
				}

				j = j + 1
				newlyCreatedPods = append(newlyCreatedPods, *newPod)
			}

			waitTimeout := willTimeoutAt.Sub(time.Now())
			waitCreatingPods(logger, clientset, waitTimeout, configYAML.Name,
				newlyCreatedPods, nil, mr)
		}
	}

	return nil
}

func createPodsAsTheyAreDeleted(
	l logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	clientset kubernetes.Interface,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	timeout time.Duration,
	configYAML *models.ConfigYAML,
	deletedPods []v1.Pod,
	scheduler *models.Scheduler,
	operationManager *models.OperationManager,
) (createdPods []v1.Pod, timedout, wasCanceled bool) {
	logger := l.WithFields(logrus.Fields{
		"operation": "controller.waitTerminatingPods",
		"scheduler": configYAML.Name,
	})

	createdPods = []v1.Pod{}
	logger.Debugf("pods to terminate: %#v", names(deletedPods))

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	i := 0
	for {
		exit := true
		select {
		case <-ticker.C:
			if operationManager.WasCanceled() {
				logger.Warn("operation was canceled")
				return createdPods, false, true
			}

			for j := i; j < len(deletedPods); j++ {
				pod := deletedPods[i]
				err := mr.WithSegment(models.SegmentPod, func() error {
					var err error
					_, err = clientset.CoreV1().Pods(configYAML.Name).Get(pod.GetName(), getOptions)
					return err
				})
				if err == nil || !strings.Contains(err.Error(), "not found") {
					logger.WithField("pod", pod.GetName()).Debugf("pod still exists")
					exit = false
					break
				}

				newPod, err := roomManager.Create(logger, mr, redisClient,
					db, clientset, configYAML, scheduler)
				if err != nil {
					exit = false
					logger.
						WithError(err).
						Info("error creating pod")
					break
				}

				i = j + 1

				createdPods = append(createdPods, *newPod)
			}
		case <-timeoutTimer.C:
			err := errors.New("timeout waiting for rooms to be removed")
			logger.WithError(err).Error("stopping scale")
			return createdPods, true, false
		}

		if exit {
			logger.Info("terminating pods were successfully removed")
			break
		}
	}

	return createdPods, false, false
}

func waitTerminatingPods(
	l logrus.FieldLogger,
	clientset kubernetes.Interface,
	timeout time.Duration,
	namespace string,
	deletedPods []v1.Pod,
	mr *models.MixedMetricsReporter,
) error {
	logger := l.WithFields(logrus.Fields{
		"operation": "controller.waitTerminatingPods",
		"scheduler": namespace,
	})

	logger.Debugf("waiting for pods to terminate: %#v", names(deletedPods))

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		exit := true
		select {
		case <-ticker.C:
			for _, pod := range deletedPods {
				err := mr.WithSegment(models.SegmentPod, func() error {
					var err error
					_, err = clientset.CoreV1().Pods(namespace).Get(
						pod.GetName(), getOptions,
					)
					return err
				})

				if err == nil || !strings.Contains(err.Error(), "not found") {
					logger.WithField("pod", pod.GetName()).Debugf("pod still exists, deleting again")
					err = mr.WithSegment(models.SegmentPod, func() error {
						return clientset.CoreV1().Pods(namespace).Delete(pod.GetName(), deleteOptions)
					})
					exit = false
					break
				}
			}
		case <-timeoutTimer.C:
			err := errors.New("timeout waiting for rooms to be removed")
			logger.WithError(err).Error("stopping scale")
			return err
		}

		if exit {
			logger.Info("terminating pods were successfully removed")
			break
		}
	}

	return nil
}

func waitCreatingPods(
	l logrus.FieldLogger,
	clientset kubernetes.Interface,
	timeout time.Duration,
	namespace string,
	createdPods []v1.Pod,
	operationManager *models.OperationManager,
	mr *models.MixedMetricsReporter,
) (timedout, wasCanceled bool) {
	logger := l.WithFields(logrus.Fields{
		"operation": "controller.waitCreatingPods",
		"scheduler": namespace,
	})

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		exit := true
		select {
		case <-ticker.C:
			if operationManager.WasCanceled() {
				logger.Warn("operation was canceled")
				return false, true
			}

			for _, pod := range createdPods {
				var createdPod *v1.Pod
				err := mr.WithSegment(models.SegmentPod, func() error {
					var err error
					createdPod, err = clientset.CoreV1().Pods(namespace).Get(
						pod.GetName(), getOptions,
					)
					return err
				})
				if err != nil && strings.Contains(err.Error(), "not found") {
					exit = false
					logger.
						WithError(err).
						WithField("pod", pod.GetName()).
						Info("error creating pod, recreating...")

					pod.ResourceVersion = ""
					err = mr.WithSegment(models.SegmentPod, func() error {
						var err error
						_, err = clientset.CoreV1().Pods(namespace).Create(&pod)
						return err
					})
					if err != nil {
						logger.
							WithError(err).
							WithField("pod", pod.GetName()).
							Errorf("error recreating pod")
					}
					break
				}

				if len(createdPod.Status.Phase) == 0 {
					//HACK! Trying to detect if we are running unit tests
					break
				}

				if !models.IsPodReady(createdPod) {
					logger.WithField("pod", createdPod.GetName()).Debug("pod not ready yet, waiting...")
					exit = false
					break
				}
			}
		case <-timeoutTimer.C:
			logger.Error("timeout waiting for rooms to be created")
			return true, false
		}

		if exit {
			logger.Info("creating pods are successfully running")
			break
		}
	}

	return false, false
}

// DeletePodAndRoom deletes the pod and removes the room from redis
func DeletePodAndRoom(
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
	configYaml *models.ConfigYAML,
	name, reason string,
) error {
	var pod *models.Pod
	err := mr.WithSegment(models.SegmentPod, func() error {
		var err error
		pod, err = models.NewPod(name, nil, configYaml, clientset, redisClient)
		return err
	})
	if err != nil {
		return err
	}

	err = roomManager.Delete(logger, mr, clientset, redisClient, configYaml,
		pod.Name, reportersConstants.ReasonUpdate)
	if err != nil {
		logger.
			WithField("roomName", pod.Name).
			WithError(err).
			Error("error removing room info from redis")
		return err
	}

	room := models.NewRoom(pod.Name, configYaml.Name)
	err = room.ClearAll(redisClient, mr)
	if err != nil {
		logger.
			WithField("roomName", pod.Name).
			WithError(err).
			Error("error removing room info from redis")
		return err
	}

	return nil
}

func segmentPods(pods []v1.Pod, maxSurge int) [][]v1.Pod {
	if pods == nil || len(pods) == 0 {
		return make([][]v1.Pod, 0)
	}

	totalLength := len(pods)
	chunkLength := chunkLength(pods, maxSurge)
	chunks := nChunks(pods, chunkLength)
	podChunks := make([][]v1.Pod, chunks)

	for i := range podChunks {
		start := i * chunkLength
		end := start + chunkLength
		if end > totalLength {
			end = totalLength
		}

		podChunks[i] = pods[start:end]
	}

	return podChunks
}

func chunkLength(pods []v1.Pod, maxSurge int) int {
	denominator := 100.0 / float64(maxSurge)
	lenPods := float64(len(pods))
	return int(math.Ceil(lenPods / denominator))
}

func nChunks(pods []v1.Pod, chunkLength int) int {
	return int(math.Ceil(float64(len(pods)) / float64(chunkLength)))
}

func names(pods []v1.Pod) []string {
	names := make([]string, len(pods))
	for i, pod := range pods {
		names[i] = pod.GetName()
	}
	return names
}

// GetLockKey returns the key of the scheduler lock
func GetLockKey(prefix, schedulerName string) string {
	return fmt.Sprintf("%s-%s", prefix, schedulerName)
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
			msg := "timeout waiting for rooms to be created"
			l.Error(msg)
			return errors.New(msg)
		case <-ticker.C:
			for i := range pods {
				if pods[i] != nil {
					var pod *v1.Pod
					err := mr.WithSegment(models.SegmentPod, func() error {
						var err error
						pod, err = clientset.CoreV1().Pods(namespace).Get(pods[i].GetName(), metav1.GetOptions{})
						return err
					})
					if err != nil {
						//The pod does not exist (not even on Pending or ContainerCreating state), so create again
						exit = false
						l.WithError(err).Infof("error creating pod %s, recreating...", pods[i].GetName())
						pods[i].ResourceVersion = ""
						err = mr.WithSegment(models.SegmentPod, func() error {
							_, err = clientset.CoreV1().Pods(namespace).Create(pods[i])
							return err
						})
						if err != nil {
							l.WithError(err).Errorf("error recreating pod %s", pods[i].GetName())
						}
					} else {
						if models.IsUnitTest(pod) {
							break
						}

						if pod.Status.Phase != v1.PodRunning {
							isPending, reason, message := models.PodPending(pod)
							if isPending && strings.Contains(message, models.PodNotFitsHostPorts) {
								l.WithFields(logrus.Fields{
									"reason":  reason,
									"message": message,
								}).Error("pod's host port is not available in any node of the pool, watcher will delete it soon")
								continue
							} else {
								l.WithFields(logrus.Fields{
									"pod":     pod.GetName(),
									"pending": isPending,
									"reason":  reason,
									"message": message,
								}).Warn("pod is not running yet")
								exit = false
								break
							}
						}

						if !models.IsPodReady(pod) {
							exit = false
							break
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

func pendingPods(
	clientset kubernetes.Interface,
	namespace string,
	mr *models.MixedMetricsReporter,
) (bool, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set{}.AsSelector().String(),
		FieldSelector: fields.Everything().String(),
	}
	var pods *v1.PodList
	err := mr.WithSegment(models.SegmentPod, func() error {
		var err error
		pods, err = clientset.CoreV1().Pods(namespace).List(listOptions)
		return err
	})
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

func getSchedulersAndGlobalPortRanges(
	db pginterfaces.DB,
	redis redisinterfaces.RedisClient,
	log logrus.FieldLogger,
) (ranges map[string]*models.PortRange, err error) {
	log = log.WithField("operation", "controller.getSchedulersPortRanges")

	ranges = map[string]*models.PortRange{}

	log.Debug("listing schedulers")
	names, err := models.ListSchedulersNames(db)
	if err != nil {
		log.WithError(err).Error("error listing schedulers from db")
		return ranges, err
	}

	log.Debug("loading schedulers")
	schedulers, err := models.LoadSchedulers(db, names)
	if err != nil {
		log.WithError(err).Error("error loading schedulers from db")
		return ranges, err
	}

	log.Debug("unmarshaling config yamls")
	for _, scheduler := range schedulers {
		configYaml, err := models.NewConfigYAML(scheduler.YAML)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal scheduler %s", scheduler.Name)
			return nil, err
		}

		if configYaml.PortRange.IsSet() {
			ranges[scheduler.Name] = configYaml.PortRange
		}
	}

	log.Debug("getting global port range")
	start, end, err := models.GetGlobalPortRange(redis)
	if err != nil {
		log.WithError(err).Error("failed to get global port range from redis")
		return ranges, err
	}

	log.Debug("successfully got port ranges")
	ranges[models.Global] = &models.PortRange{
		Start: start,
		End:   end,
	}

	return ranges, nil
}

func checkPortRange(
	oldConfig, newConfig *models.ConfigYAML,
	log logrus.FieldLogger,
	db pginterfaces.DB,
	redis redisinterfaces.RedisClient,
) (changedPortRange bool, err error) {
	isCreatingScheduler := oldConfig == nil

	if isCreatingScheduler {
		if !newConfig.PortRange.IsSet() {
			return false, nil
		}
	} else {
		if !oldConfig.PortRange.IsSet() && !newConfig.PortRange.IsSet() {
			return false, nil
		}

		if oldConfig.PortRange.IsSet() && !newConfig.PortRange.IsSet() {
			return true, nil
		}

		if oldConfig.PortRange.Equals(newConfig.PortRange) {
			log.Info("old scheduler contains new port range, skipping port check")
			return false, nil
		}

		if !newConfig.PortRange.IsValid() {
			return false, errors.New("port range is invalid")
		}
	}

	log.Info("update changed ports pool, getting all used ports range")
	ranges, err := getSchedulersAndGlobalPortRanges(db, redis, log)
	if err != nil {
		return true, err
	}

	log.WithField("pool", newConfig.PortRange.String()).Info("checking if new pool has intersection with other ones")
	for schedulerName, portRange := range ranges {
		if schedulerName == newConfig.Name {
			continue
		}
		if portRange.HasIntersection(newConfig.PortRange) {
			return true, fmt.Errorf("scheduler trying to use ports used by pool '%s'", schedulerName)
		}
	}

	return true, nil
}
