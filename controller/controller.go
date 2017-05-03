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
		deleteErr := mr.WithSegment(models.SegmentNamespace, func() error {
			return namespace.Delete(clientset)
		})
		if deleteErr != nil {
			return deleteErr
		}
		return err
	}

	if err != nil {
		deleteErr := deleteSchedulerHelper(logger, mr, db, clientset, scheduler, namespace)
		if deleteErr != nil {
			return deleteErr
		}
		return err
	}

	err = ScaleUp(logger, mr, db, redisClient, clientset, scheduler.Name, configYAML.AutoScaling.Min, timeoutSec, true)
	if err != nil {
		deleteErr := deleteSchedulerHelper(logger, mr, db, clientset, scheduler, namespace)
		if deleteErr != nil {
			return deleteErr
		}
		return err
	}

	state := models.NewSchedulerState(scheduler.Name, "in-sync", time.Now().Unix(), time.Now().Unix())
	err = SaveSchedulerStateInfo(logger, mr, redisClient, state)
	if err != nil {
		deleteErr := deleteSchedulerHelper(logger, mr, db, clientset, scheduler, namespace)
		if deleteErr != nil {
			return deleteErr
		}
		return err
	}
	return nil
}

// DeleteScheduler deletes a scheduler from a yaml configuration
func DeleteScheduler(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, clientset kubernetes.Interface, schedulerName string) error {
	scheduler := models.NewScheduler(schedulerName, "", "")
	err := mr.WithSegment(models.SegmentSelect, func() error {
		return scheduler.Load(db)
	})
	if err != nil {
		return err
	}
	namespace := models.NewNamespace(scheduler.Name)
	return deleteSchedulerHelper(logger, mr, db, clientset, scheduler, namespace)
}

// GetSchedulerScalingInfo returns the scheduler scaling policies and room count by status
func GetSchedulerScalingInfo(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, client redisinterfaces.RedisClient, schedulerName string) (*models.AutoScaling, *models.RoomsStatusCount, error) {
	scheduler := models.NewScheduler(schedulerName, "", "")
	err := mr.WithSegment(models.SegmentSelect, func() error {
		return scheduler.Load(db)
	})
	if err != nil {
		return nil, nil, err
	}
	scalingPolicy := scheduler.GetAutoScalingPolicy()
	var roomCountByStatus *models.RoomsStatusCount
	err = mr.WithSegment(models.SegmentGroupBy, func() error {
		roomCountByStatus, err = models.GetRoomsCountByStatus(client, scheduler.Name)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return scalingPolicy, roomCountByStatus, nil
}

// GetSchedulerStateInfo returns the scheduler state information
func GetSchedulerStateInfo(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, client redisinterfaces.RedisClient, schedulerName string) (*models.SchedulerState, error) {
	state := models.NewSchedulerState(schedulerName, "", 0, 0)
	err := mr.WithSegment(models.SegmentHGetAll, func() error {
		return state.Load(client)
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}

// SaveSchedulerStateInfo updates the scheduler state information
func SaveSchedulerStateInfo(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, client redisinterfaces.RedisClient, state *models.SchedulerState) error {
	return mr.WithSegment(models.SegmentHMSet, func() error {
		return state.Save(client)
	})
}

// ScaleUp scales up a scheduler using its config
func ScaleUp(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, redisClient redisinterfaces.RedisClient, clientset kubernetes.Interface, schedulerName string, amount, timeoutSec int, initalOp bool) error {
	l := logger.WithFields(logrus.Fields{
		"source":    "scaleUp",
		"scheduler": schedulerName,
		"amount":    amount,
	})
	l.Info("scaling scheduler up")
	scheduler := models.NewScheduler(schedulerName, "", "")
	err := mr.WithSegment(models.SegmentSelect, func() error {
		return scheduler.Load(db)
	})
	if err != nil {
		return err
	}
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
	select {
	case <-timeout:
		return errors.New("timeout scaling up scheduler")
	default:
		exit := true
		for i := 0; i < amount; i++ {
			pod, err := clientset.CoreV1().Pods(scheduler.Name).Get(pods[i], metav1.GetOptions{})
			if err != nil {
				l.WithError(err).Error("scale up pod error")
			} else {
				for _, containerStatus := range pod.Status.ContainerStatuses {
					if !containerStatus.Ready {
						exit = false
					}
				}
			}
			if exit {
				l.Info("finished scaling up scheduler")
				break
			}
		}
		l.Debug("scaling up scheduler...")
		time.Sleep(time.Duration(1) * time.Second)
	}
	// TODO: set SchedulerState.State back to "in-sync"
	return creationErr
}

func deleteSchedulerHelper(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db pginterfaces.DB, clientset kubernetes.Interface, scheduler *models.Scheduler, namespace *models.Namespace) error {
	err := mr.WithSegment(models.SegmentNamespace, func() error {
		return namespace.Delete(clientset) // TODO: we're assuming that deleting a namespace gracefully terminates all its pods
	})
	if err != nil {
		logger.WithError(err).Error("Failed to delete namespace while rolling back cluster creation.")
		return err
	}
	err = mr.WithSegment(models.SegmentDelete, func() error {
		return scheduler.Delete(db)
	})
	if err != nil {
		logger.WithError(err).Error("Failed to delete scheduler while rolling back cluster creation.")
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
		&models.EnvVar{
			Name:  "MAESTRO_SCHEDULER_NAME",
			Value: configYAML.Name,
		},
		&models.EnvVar{
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
	// TODO WIP guardar ip
	// node, err := clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	// if err != nil {
	// 	return "", err
	// }
	return name, nil
}
