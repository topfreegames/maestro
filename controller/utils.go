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
	"math"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/reporters"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	redisLock "github.com/bsm/redis-lock"
	clockinterfaces "github.com/topfreegames/extensions/clock/interfaces"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/extensions/redis"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	maestroErrors "github.com/topfreegames/maestro/errors"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SegmentAndReplacePods acts when a scheduler rolling update is needed.
// It segment the list of current pods in chunks of size maxSurge and replace them with new ones
func SegmentAndReplacePods(
	ctx context.Context,
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	clientset kubernetes.Interface,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	willTimeoutAt time.Time,
	configYAML *models.ConfigYAML,
	pods []v1.Pod,
	scheduler *models.Scheduler,
	operationManager *models.OperationManager,
	maxSurge int,
	clock clockinterfaces.Clock,
) (timeoutErr, cancelErr, err error) {
	schedulerName := scheduler.Name
	l := logger.WithFields(logrus.Fields{
		"source":    "SegmentAndReplacePods",
		"scheduler": schedulerName,
	})

	// segment pods in chunks
	podChunks := segmentPods(pods, maxSurge)

	for i, chunk := range podChunks {
		l.Debugf("updating chunk %d: %v", i, names(chunk))

		// replace chunk
		timedout, canceled, errored := replacePodsAndWait(
			l,
			roomManager,
			mr,
			clientset,
			db,
			redisClient,
			willTimeoutAt,
			configYAML,
			chunk,
			scheduler,
			operationManager,
			clock,
		)

		if timedout {
			timeoutErr = errors.New("timedout waiting rooms to be replaced, rolled back")
			l.WithError(timeoutErr).Error("error replacing chunk of pods")
			break
		}

		if canceled {
			cancelErr = errors.New("operation was canceled, rolled back")
			l.WithError(cancelErr).Error("error replacing chunk of pods")
			break
		}

		if errored != nil {
			err = errored
			l.WithError(errored).Error("error replacing chunk of pods")
			break
		}
	}

	return timeoutErr, cancelErr, err
}

func replacePodsAndWait(
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	clientset kubernetes.Interface,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	willTimeoutAt time.Time,
	configYAML *models.ConfigYAML,
	podsChunk []v1.Pod,
	scheduler *models.Scheduler,
	operationManager *models.OperationManager,
	clock clockinterfaces.Clock,
) (timedout, canceled bool, err error) {
	timedout = false
	canceled = false
	var wg sync.WaitGroup
	var mutex = &sync.Mutex{}

	// create a chunk of pods (chunkSize = maxSurge) and remove a chunk of old ones
	wg.Add(len(podsChunk))
	for _, pod := range podsChunk {
		go func(pod v1.Pod) {
			defer wg.Done()
			localTimedout, localCanceled, localErr := createNewRemoveOldPod(
				logger,
				roomManager,
				mr,
				clientset,
				db,
				redisClient,
				willTimeoutAt,
				configYAML,
				scheduler,
				operationManager,
				mutex,
				pod,
				clock,
			)
			// if a routine is timedout or canceled,
			// rolling update should stop
			if localTimedout {
				mutex.Lock()
				timedout = localTimedout
				mutex.Unlock()
			}
			if localCanceled {
				mutex.Lock()
				canceled = localCanceled
				mutex.Unlock()
			}
			if localErr != nil {
				mutex.Lock()
				err = localErr
				mutex.Unlock()
			}
		}(pod)
	}
	wg.Wait()

	return timedout, canceled, err
}

func createNewRemoveOldPod(
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	clientset kubernetes.Interface,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	willTimeoutAt time.Time,
	configYAML *models.ConfigYAML,
	scheduler *models.Scheduler,
	operationManager *models.OperationManager,
	mutex *sync.Mutex,
	pod v1.Pod,
	clock clockinterfaces.Clock,
) (timedout, canceled bool, err error) {
	logger.Debug("creating pod")

	// create new pod
	newPod, err := roomManager.Create(logger, mr, redisClient,
		db, clientset, configYAML, scheduler)

	if err != nil {
		logger.WithError(err).Debug("error creating pod")
		return false, false, err
	}

	// wait for new pod to be created
	timeout := willTimeoutAt.Sub(clock.Now())
	timedout, canceled, err = waitCreatingPods(
		logger, clientset, timeout, configYAML.Name,
		[]v1.Pod{*newPod}, operationManager, mr)
	if timedout || canceled || err != nil {
		return timedout, canceled, err
	}

	// delete old pod
	logger.Debugf("deleting pod %s", pod.GetName())
	err = DeletePodAndRoom(logger, roomManager, mr, clientset, redisClient,
		configYAML, pod.GetName(), reportersConstants.ReasonUpdate)
	if err != nil && !strings.Contains(err.Error(), "redis") {
		logger.WithError(err).Errorf("error deleting pod %s during rolling update", pod.GetName())
		return false, false, nil
	}

	// wait for old pods to be deleted
	// we assume that maxSurge == maxUnavailable as we can't set maxUnavailable yet
	// so for every pod created in a chunk one is deleted right after it
	timeout = willTimeoutAt.Sub(clock.Now())
	timedout, canceled = waitTerminatingPods(
		logger, clientset, timeout, configYAML.Name,
		[]v1.Pod{pod}, operationManager, mr)
	if timedout || canceled {
		return timedout, canceled, nil
	}

	return false, false, nil
}

// DBRollback perform a rollback on a scheduler config in the database
func DBRollback(
	ctx context.Context,
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient *redis.Client,
	failedConfigYAML *models.ConfigYAML,
	oldConfigYAML *models.ConfigYAML,
	clock clockinterfaces.Clock,
	scheduler *models.Scheduler,
	config *viper.Viper,
	oldVersion string,
) (err error) {
	eventRollbackTags := map[string]interface{}{
		"name": scheduler.Name,
		"game": scheduler.Game,
	}
	err = scheduler.UpdateVersionStatus(db)
	if err != nil {
		reporters.Report(reportersConstants.EventSchedulerRollbackError, eventRollbackTags)
		return err
	}

	// create new major version to rollback
	scheduler.NextMajorVersion()
	scheduler.RollingUpdateStatus = rollbackStatus(oldVersion)
	err = saveConfigYAML(
		ctx,
		logger,
		mr,
		db,
		oldConfigYAML,
		scheduler,
		*failedConfigYAML,
		config,
		oldVersion,
	)
	if err != nil {
		reporters.Report(reportersConstants.EventSchedulerRollbackError, eventRollbackTags)
		return err
	}

	err = scheduler.UpdateVersionStatus(db)
	if err != nil {
		reporters.Report(reportersConstants.EventSchedulerRollbackError, eventRollbackTags)
		return err
	}

	return nil
}

func waitTerminatingPods(
	l logrus.FieldLogger,
	clientset kubernetes.Interface,
	timeout time.Duration,
	namespace string,
	deletedPods []v1.Pod,
	operationManager *models.OperationManager,
	mr *models.MixedMetricsReporter,
) (timedout, wasCanceled bool) {
	logger := l.WithFields(logrus.Fields{
		"source":    "controller.waitTerminatingPods",
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
			// operationManger is nil when rolling back (rollback can't be canceled)
			if operationManager != nil && operationManager.WasCanceled() {
				logger.Warn("operation was canceled")
				return false, true
			}

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

				if err != nil && !strings.Contains(err.Error(), "not found") {
					logger.
						WithError(err).
						WithField("pod", pod.GetName()).
						Info("error getting pod")
					exit = false
					break
				}

				if err != nil && !strings.Contains(err.Error(), "not found") {
					logger.
						WithError(err).
						WithField("pod", pod.GetName()).
						Info("error getting pod")
					exit = false
					break
				}
			}
		case <-timeoutTimer.C:
			logger.Error("timeout waiting for rooms to be removed")
			return true, false
		}

		if exit {
			logger.Info("terminating pods were successfully removed")
			break
		}
	}

	return false, false
}

func waitCreatingPods(
	l logrus.FieldLogger,
	clientset kubernetes.Interface,
	timeout time.Duration,
	namespace string,
	createdPods []v1.Pod,
	operationManager *models.OperationManager,
	mr *models.MixedMetricsReporter,
) (timedout, wasCanceled bool, err error) {
	logger := l.WithFields(logrus.Fields{
		"source":    "controller.waitCreatingPods",
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
			// operationManger is nil when rolling back (rollback can't be canceled)
			if operationManager != nil && operationManager.WasCanceled() {
				logger.Warn("operation was canceled")
				return false, true, nil
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
						Error("error creating pod, recreating...")

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

				if err != nil && !strings.Contains(err.Error(), "not found") {
					logger.
						WithError(err).
						WithField("pod", pod.GetName()).
						Error("error getting pod")
					exit = false
					break
				}

				if !models.IsPodReady(createdPod) {
					logger.WithField("pod", createdPod.GetName()).Debug("pod not ready yet, waiting...")
					err = models.ValidatePodWaitingState(createdPod)

					if err != nil {
						logger.WithField("pod", pod.GetName()).WithError(err).Error("invalid pod waiting state")
						return false, false, err
					}

					exit = false
					break
				}
			}
		case <-timeoutTimer.C:
			logger.Error("timeout waiting for rooms to be created")
			return true, false, nil
		}

		if exit {
			logger.Info("creating pods are successfully running")
			break
		}
	}

	return false, false, nil
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
			log.WithError(err).Errorf("failed to unmarshal scheduler %s", scheduler.Name)
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

// SetScalingAmount check the max and min limits and adjust the amount to scale accordingly
func SetScalingAmount(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	scheduler *models.Scheduler,
	max, min, amount int,
	isScaleDown bool,
) (int, error) {
	currentRooms, err := models.GetRoomsCountByStatus(redisClient, scheduler.Name)
	if err != nil {
		return 0, err
	}

	if isScaleDown == true {
		return setScaleDownAmount(logger, amount, currentRooms.Available(), max, min), nil
	}

	return setScaleUpAmount(logger, amount, currentRooms.Available(), max, min), nil
}

func setScaleUpAmount(logger logrus.FieldLogger, amount, currentRooms, max, min int) int {
	if max > 0 {
		if currentRooms >= max {
			logger.Warn("scale already at max. Not scaling up any rooms")
			return 0
		}

		if currentRooms+amount > max {
			logger.Warnf("amount to scale is higher than max. Maestro will scale up to the max of %d", max)
			return max - currentRooms
		}
	}

	if currentRooms+amount < min {
		logger.Warnf("amount to scale is lower than min. Maestro will scale up to the min of %d", min)
		return min - currentRooms
	}

	return amount
}

func setScaleDownAmount(logger logrus.FieldLogger, amount, currentRooms, max, min int) int {
	if min > 0 {
		if currentRooms <= min {
			logger.Warn("scale already at min. Not scaling down any rooms")
			return 0
		}

		if currentRooms-amount < min {
			logger.Warnf("amount to scale is lower than min. Maestro will scale down to the min of %d", min)
			return currentRooms - min
		}
	}

	if max > 0 && currentRooms-amount > max {
		logger.Warnf("amount to scale is lower than max. Maestro will scale down to the max of %d", max)
		return currentRooms - max
	}

	return amount
}

func validateMetricsTrigger(configYAML *models.ConfigYAML, logger logrus.FieldLogger) error {
	for _, trigger := range configYAML.AutoScaling.Up.MetricsTrigger {
		if trigger.Type == models.CPUAutoScalingPolicyType {
			if (configYAML.Requests == nil || configYAML.Requests.CPU == "") && len(configYAML.Containers) == 0 {
				logger.Error("must set requests.cpu in order to use cpu autoscaling")
				return fmt.Errorf("must set requests.cpu in order to use cpu autoscaling")
			}
			for _, container := range configYAML.Containers {
				if container.Requests == nil || container.Requests.CPU == "" {
					logger.Error("must set requests.cpu in order to use cpu autoscaling")
					return fmt.Errorf("must set requests.cpu in order to use cpu autoscaling")
				}
			}
		}

		if trigger.Type == models.MemAutoScalingPolicyType {
			if (configYAML.Requests == nil || configYAML.Requests.Memory == "") && len(configYAML.Containers) == 0 {
				logger.Error("must set requests.memory in order to use mem autoscaling")
				return fmt.Errorf("must set requests.memory in order to use mem autoscaling")
			}
			for _, container := range configYAML.Containers {
				if container.Requests == nil || container.Requests.Memory == "" {
					logger.Error("must set requests.memory in order to use mem autoscaling")
					return fmt.Errorf("must set requests.memory in order to use mem autoscaling")
				}
			}
		}
	}

	for _, trigger := range configYAML.AutoScaling.Down.MetricsTrigger {
		if trigger.Type == models.CPUAutoScalingPolicyType {
			if (configYAML.Requests == nil || configYAML.Requests.CPU == "") && len(configYAML.Containers) == 0 {
				logger.Error("must set requests.cpu in order to use cpu autoscaling")
				return fmt.Errorf("must set requests.cpu in order to use cpu autoscaling")
			}
			for _, container := range configYAML.Containers {
				if container.Requests == nil || container.Requests.CPU == "" {
					logger.Error("must set requests.cpu in order to use cpu autoscaling")
					return fmt.Errorf("must set requests.cpu in order to use cpu autoscaling")
				}
			}
		}

		if trigger.Type == models.MemAutoScalingPolicyType {
			if (configYAML.Requests == nil || configYAML.Requests.Memory == "") && len(configYAML.Containers) == 0 {
				logger.Error("must set requests.memory in order to use mem autoscaling")
				return fmt.Errorf("must set requests.memory in order to use mem autoscaling")
			}
			for _, container := range configYAML.Containers {
				if container.Requests == nil || container.Requests.Memory == "" {
					logger.Error("must set requests.memory in order to use mem autoscaling")
					return fmt.Errorf("must set requests.memory in order to use mem autoscaling")
				}
			}
		}
	}
	return nil
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

func validateConfig(logger logrus.FieldLogger, configYAML *models.ConfigYAML, maxSurge int) error {
	if maxSurge <= 0 {
		return errors.New("invalid parameter: maxsurge must be greater than 0")
	}

	if configYAML.AutoScaling.Max > 0 && configYAML.AutoScaling.Min > configYAML.AutoScaling.Max {
		return errors.New("invalid parameter: autoscaling max must be greater than min")
	}

	// if using resource scaling (cpu, mem) requests must be set
	return validateMetricsTrigger(configYAML, logger)
}

// LoadScheduler loads a scheduler from DB with its YAML config
func LoadScheduler(
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	schedulerOrNil *models.Scheduler,
	schedulerName string,
) (*models.Scheduler, models.ConfigYAML, error) {
	var scheduler *models.Scheduler
	var configYAML models.ConfigYAML
	var err error
	if schedulerOrNil != nil {
		scheduler = schedulerOrNil
		err = yaml.Unmarshal([]byte(scheduler.YAML), &configYAML)
		if err != nil {
			return scheduler, configYAML, err
		}
	} else {
		scheduler, configYAML, err = schedulerAndConfigFromName(mr, db, schedulerName)
		if err != nil {
			return scheduler, configYAML, err
		}
	}
	return scheduler, configYAML, nil
}

// AcquireLock acquires a lock defined by its lockKey
func AcquireLock(
	ctx context.Context,
	logger logrus.FieldLogger,
	redisClient *redis.Client,
	config *viper.Viper,
	operationManager *models.OperationManager,
	lockKey, schedulerName string,
) (lock *redisLock.Lock, canceled bool, err error) {
	timeoutSec := config.GetInt("updateTimeoutSeconds")
	lockTimeoutMS := config.GetInt("watcher.lockTimeoutMs")
	timeoutDur := time.Duration(timeoutSec) * time.Second
	ticker := time.NewTicker(2 * time.Second)

	// guarantee that downScaling and config locks doesn't timeout before update times out.
	// otherwise it can result in all pods dying during a rolling update that is destined to timeout
	if (lockKey == models.GetSchedulerDownScalingLockKey(config.GetString("watcher.lockKey"), schedulerName) ||
		lockKey == models.GetSchedulerConfigLockKey(config.GetString("watcher.lockKey"), schedulerName)) &&
		lockTimeoutMS < timeoutSec*1000 {
		lockTimeoutMS = (timeoutSec + 1) * 1000
	}

	defer ticker.Stop()
	timeout := time.NewTimer(timeoutDur)
	defer timeout.Stop()

	l := logger.WithFields(logrus.Fields{
		"source":    "AcquireLock",
		"scheduler": schedulerName,
	})

	for {
		exit := false
		lock, err = redisClient.EnterCriticalSection(
			redisClient.Trace(ctx),
			lockKey,
			time.Duration(lockTimeoutMS)*time.Millisecond,
			0, 0,
		)
		select {
		case <-timeout.C:
			l.Warn("timeout while wating for redis lock")
			return nil, false, errors.New("timeout while wating for redis lock")
		case <-ticker.C:
			if operationManager != nil && operationManager.WasCanceled() {
				l.Warn("operation was canceled")
				return nil, true, nil
			}

			if err != nil {
				l.WithError(err).Error("error getting watcher lock")
				return nil, false, err
			}

			if lock == nil {
				l.Warnf("unable to get watcher %s lock %s, maybe some other process has it", schedulerName, lockKey)
				break
			}

			if lock.IsLocked() {
				exit = true
				break
			}
		}
		if exit {
			l.Debugf("acquired lock %s", lockKey)
			break
		}
	}

	return lock, false, err
}

// AcquireLockOnce tries to acquire a lock defined by its lockKey only once
// If lock is already acquired by another process it just returns an error
func AcquireLockOnce(
	ctx context.Context,
	logger logrus.FieldLogger,
	redisClient *redis.Client,
	config *viper.Viper,
	lockKey string,
	schedulerName string,
) (*redisLock.Lock, error) {
	l := logger.WithFields(logrus.Fields{
		"source":    "AcquireLockOnce",
		"scheduler": schedulerName,
	})

	lockTimeoutMS := config.GetInt("watcher.lockTimeoutMs")

	lock, err := redisClient.EnterCriticalSection(
		redisClient.Trace(ctx),
		lockKey,
		time.Duration(lockTimeoutMS)*time.Millisecond,
		0, 0,
	)

	if err != nil {
		l.WithError(err).Error("error acquiring lock")
		return nil, err
	}

	if lock == nil {
		l.Warnf("unable to acquire scheduler %s lock %s, maybe some other process has it", schedulerName, lockKey)
		return nil, fmt.Errorf("unable to acquire scheduler %s lock %s, maybe some other process has it", schedulerName, lockKey)
	}

	if lock.IsLocked() {
		l.Debugf("acquired lock %s", lockKey)
	}

	return lock, err
}

// ReleaseLock releases a lock defined by its lockKey
func ReleaseLock(
	logger logrus.FieldLogger,
	redisClient *redis.Client,
	lock *redisLock.Lock,
	schedulerName string,
) {
	l := logger.WithFields(logrus.Fields{
		"source":    "ReleaseLock",
		"scheduler": schedulerName,
	})

	if lock != nil {
		err := redisClient.LeaveCriticalSection(lock)
		if err != nil {
			l.WithError(err).Error("error releasing lock. Either wait or remove it manually from redis")
		} else {
			l.Debug("lock released")
		}
	} else {
		l.Debug("lock is nil. No lock to release")
	}
}

func saveConfigYAML(
	ctx context.Context,
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	configYAML *models.ConfigYAML,
	scheduler *models.Scheduler,
	oldConfig models.ConfigYAML,
	config *viper.Viper,
	oldVersion string,
) error {
	l := logger.WithFields(logrus.Fields{
		"source":    "saveConfigYAML",
		"scheduler": configYAML.Name,
	})
	maxVersions := config.GetInt("schedulers.versions.toKeep")

	l.Debug("updating configYAML on database")

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
			created, err := scheduler.UpdateVersion(db, maxVersions, oldVersion)
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

// ListCurrentPods returns a list of kubernetes pods
func ListCurrentPods(
	mr *models.MixedMetricsReporter,
	clientset kubernetes.Interface,
	schedulerName string,
) (*v1.PodList, error) {
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
		return nil, maestroErrors.NewKubernetesError("error when listing pods", err)
	}
	return kubePods, nil
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
			logger.Debug("deleting scheduler namespace")
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

	reporters.Report(reportersConstants.EventSchedulerDelete, map[string]interface{}{
		"name": scheduler.Name,
		"game": scheduler.Game,
	})

	return nil
}
