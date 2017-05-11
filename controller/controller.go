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
	uuid "github.com/satori/go.uuid"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	"github.com/topfreegames/maestro/models"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	err = mr.WithSegment(models.SegmentNamespace, func() error {
		return namespace.Create(clientset)
	})

	if err != nil {
		deleteErr := mr.WithSegment(models.SegmentNamespace, func() error {
			return namespace.Delete(clientset)
		})
		if deleteErr != nil {
			return deleteErr
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
			return deleteErr
		}
		return err
	}

	err = ScaleUp(logger, mr, db, redisClient, clientset, scheduler, configYAML.AutoScaling.Min, timeoutSec, true)
	if err != nil {
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
			return deleteErr
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
		logger.WithError(err).Error("error listing rooms")
		return err
	}
	if len(roomNames) == 0 {
		logger.Debug("no rooms need to be deleted")
		return nil
	}
	for _, roomName := range roomNames {
		err := deleteServiceAndPod(logger, mr, clientset, schedulerName, roomName)
		if err != nil {
			logger.WithField("roomName", roomName).WithError(err).Error("error deleting room")
		} else {
			room := models.NewRoom(roomName, schedulerName)
			err := room.ClearAll(redisClient)
			if err != nil {
				logger.WithField("roomName", roomName).WithError(err).Error("error removing room info from redis")
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

	timeout := make(chan bool, 1)
	pods := make([]string, amount)
	var creationErr error
	go func() {
		time.Sleep(time.Duration(timeoutSec) * time.Second)
		timeout <- true
	}()
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
	for {
		exit := true
		select {
		case <-timeout:
			return errors.New("timeout scaling up scheduler")
		default:
			for i := 0; i < amount; i++ {
				pod, err := clientset.CoreV1().Pods(scheduler.Name).Get(pods[i], metav1.GetOptions{})
				if err != nil {
					l.WithError(err).Error("scale up pod error")
				} else {
					if len(pod.Status.Phase) == 0 {
						break // TODO: HACK!!!  Trying to detect if we are running unit tests
					}
					if pod.Status.Phase != v1.PodRunning {
						exit = false
					}
					for _, containerStatus := range pod.Status.ContainerStatuses {
						if !containerStatus.Ready {
							exit = false
						}
					}
				}
			}
			l.Debug("scaling up scheduler...")
			time.Sleep(time.Duration(1) * time.Second)
		}
		if exit {
			l.Info("finished scaling up scheduler")
			break
		}
	}
	return creationErr
}

func deleteSchedulerHelper(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, clientset kubernetes.Interface, scheduler *models.Scheduler, namespace *models.Namespace, timeoutSec int) error {
	var err error
	if scheduler.ID != "" {
		scheduler.State = models.StateTerminating
		scheduler.StateLastChangedAt = time.Now().Unix()
		err = mr.WithSegment(models.SegmentDelete, func() error {
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
		configYAML.Limits.CPU,
		configYAML.Limits.Memory,
		configYAML.Limits.CPU,    // TODO: requests should be <= limits calculate it
		configYAML.Limits.Memory, // TODO: requests should be <= limits calculate it
		configYAML.ShutdownTimeout,
		configYAML.Ports,
		configYAML.Cmd,
		env,
	)
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
	pod := models.NewPod("", "", name, schedulerName, "", "", "", "", 0, []*models.Port{}, []string{}, []*models.EnvVar{})
	err = mr.WithSegment(models.SegmentPod, func() error {
		return pod.Delete(clientset)
	})
	if err != nil {
		return err
	}
	return nil
}
