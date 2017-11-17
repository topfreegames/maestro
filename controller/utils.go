// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package controller

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"

	clockinterfaces "github.com/topfreegames/extensions/clock/interfaces"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
)

func replacePodsAndWait(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	clientset kubernetes.Interface,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	willTimeoutAt time.Time,
	clock clockinterfaces.Clock,
	configYAML *models.ConfigYAML,
	podsToDelete []v1.Pod,
) (createdPods []v1.Pod, deletedPods []v1.Pod, err error) {
	createdPods = []v1.Pod{}
	deletedPods = []v1.Pod{}

	for _, pod := range podsToDelete {
		logger.Debugf("deleting pod %s", pod.GetName())

		err = deletePodAndRoom(
			logger, mr, clientset, redisClient,
			configYAML.Name, configYAML.Game,
			pod.GetName(), reportersConstants.ReasonUpdate)
		if err == nil || strings.Contains(err.Error(), "redis") {

			deletedPods = append(deletedPods, pod)
		}
		if err != nil {
			logger.WithError(err).Debugf("error deleting pod %s", pod.GetName())
			return createdPods, deletedPods, err
		}
	}

	timeout := willTimeoutAt.Sub(clock.Now())
	createdPods, err = createPodsAsTheyAreDeleted(
		logger, mr, clientset, db, redisClient, timeout, configYAML,
		deletedPods,
	)
	if err != nil {
		return createdPods, deletedPods, err
	}

	timeout = willTimeoutAt.Sub(clock.Now())
	err = waitCreatingPods(
		logger, clientset, timeout, configYAML.Name,
		createdPods,
	)
	if err != nil {
		return createdPods, deletedPods, err
	}

	return createdPods, deletedPods, err
}

// In rollback, it must delete newly created pod and
// restore old deleted pods to come back to previous state
func rollback(
	l logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	clientset kubernetes.Interface,
	configYAML *models.ConfigYAML,
	maxSurge int,
	timeout time.Duration,
	createdPods, deletedPods []v1.Pod,
) error {
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

	for i := 0; i < len(deletedPodChunks) || i < len(createdPodChunks); i++ {
		if i < len(createdPodChunks) {
			logger.Debugf("deleting chunk %#v", names(createdPodChunks[i]))
			for j := 0; j < len(createdPodChunks[i]); {
				pod := createdPodChunks[i][j]
				logger.Debugf("deleting pod %s", pod.GetName())

				err = deletePodAndRoom(
					logger, mr, clientset, redisClient,
					configYAML.Name, configYAML.Game,
					pod.GetName(), reportersConstants.ReasonUpdate)
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

				newPod, err := createPod(logger, mr, redisClient,
					db, clientset, configYAML)
				if err != nil {
					logger.WithError(err).Debug("error creating new pod")
					time.Sleep(1 * time.Second)
					continue
				}

				j = j + 1
				newlyCreatedPods = append(newlyCreatedPods, *newPod)
			}

			waitTimeout := willTimeoutAt.Sub(time.Now())
			err = waitCreatingPods(
				logger, clientset, waitTimeout, configYAML.Name,
				newlyCreatedPods,
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func createPodsAsTheyAreDeleted(
	l logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	clientset kubernetes.Interface,
	db pginterfaces.DB,
	redisClient redisinterfaces.RedisClient,
	timeout time.Duration,
	configYAML *models.ConfigYAML,
	deletedPods []v1.Pod,
) (createdPods []v1.Pod, err error) {
	logger := l.WithFields(logrus.Fields{
		"operation": "controller.waitTerminatingPods",
		"scheduler": configYAML.Name,
	})

	createdPods = []v1.Pod{}
	logger.Debugf("waiting for pods to terminate: %#v", names(deletedPods))

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	i := 0
	for {
		exit := true
		select {
		case <-ticker.C:
			for j := i; j < len(deletedPods); j++ {
				pod := deletedPods[i]
				_, err := clientset.CoreV1().Pods(configYAML.Name).Get(
					pod.GetName(), getOptions,
				)

				if err == nil || !strings.Contains(err.Error(), "not found") {
					logger.WithField("pod", pod.GetName()).Debugf("pod still exists")
					exit = false
					break
				}

				newPod, err := createPod(logger, mr, redisClient,
					db, clientset, configYAML)
				if err != nil {
					exit = false
					logger.
						WithError(err).
						WithField("pod", newPod.GetName()).
						Info("error creating pod")
					break
				}

				i = j + 1

				createdPods = append(createdPods, *newPod)
			}
		case <-timeoutTimer.C:
			err := errors.New("timeout waiting for rooms to be removed")
			logger.WithError(err).Error("stopping scale")
			return createdPods, err
		}

		if exit {
			logger.Info("terminating pods were successfully removed")
			break
		}
	}

	return createdPods, err
}

func waitTerminatingPods(
	l logrus.FieldLogger,
	clientset kubernetes.Interface,
	timeout time.Duration,
	namespace string,
	deletedPods []v1.Pod,
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
				_, err := clientset.CoreV1().Pods(namespace).Get(
					pod.GetName(), getOptions,
				)

				if err == nil || !strings.Contains(err.Error(), "not found") {
					logger.WithField("pod", pod.GetName()).Debugf("pod still exists")
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
) error {
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
			for _, pod := range createdPods {
				createdPod, err := clientset.CoreV1().Pods(namespace).Get(
					pod.GetName(), getOptions,
				)
				if err != nil && strings.Contains(err.Error(), "not found") {
					exit = false
					logger.
						WithError(err).
						WithField("pod", pod.GetName()).
						Info("error creating pod, recreating...")

					pod.ResourceVersion = ""
					_, err = clientset.CoreV1().Pods(namespace).Create(&pod)
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

				if !v1.IsPodReady(createdPod) {
					exit = false
					break
				}
			}
		case <-timeoutTimer.C:
			return errors.New("timeout waiting for rooms to be created")
		}

		if exit {
			logger.Info("creating pods are successfully running")
			break
		}
	}

	return nil
}

func deletePodAndRoom(
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
	schedulerName, gameName, name, reason string,
) error {
	pod, err := models.NewDefaultPod(gameName, name, schedulerName, clientset, redisClient)
	if err != nil {
		return err
	}

	err = deletePod(
		logger, mr, clientset, redisClient, schedulerName, gameName,
		pod.Name, reportersConstants.ReasonUpdate)
	if err != nil {
		logger.
			WithField("roomName", pod.Name).
			WithError(err).
			Error("error removing room info from redis")
		return err
	}

	room := models.NewRoom(pod.Name, schedulerName)
	err = room.ClearAll(redisClient)
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
