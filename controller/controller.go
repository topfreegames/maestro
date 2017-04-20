// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/extensions/interfaces"
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

// CreateScheduler creates a new scheduler from a yaml configuration
func CreateScheduler(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db interfaces.DB, clientset kubernetes.Interface, yamlString string) error {
	var configYAML *models.ConfigYAML
	var err error
	err = mr.WithSegment(models.SegmentYaml, func() error {
		configYAML, err = models.NewConfigYAML(yamlString)
		return err
	})
	if err != nil {
		return err
	}

	config := models.NewConfig(configYAML.Name, configYAML.Game, yamlString)
	err = mr.WithSegment(models.SegmentInsert, func() error {
		return config.Create(db)
	})
	if err != nil {
		return err
	}

	namespace := models.NewNamespace(config.Name)
	err = mr.WithSegment(models.SegmentNamespace, func() error {
		return namespace.Create(clientset)
	})

	if err != nil {
		deleteErr := deleteSchedulerHelper(logger, mr, db, clientset, config, namespace)
		if deleteErr != nil {
			return deleteErr
		}
		return err
	}

	// TODO: optimize creation (in parallel?)
	for i := 0; i < configYAML.AutoScaling.Min; i++ {
		err := createServiceAndPod(logger, mr, db, clientset, configYAML, config.ID)
		if err != nil {
			deleteErr := deleteSchedulerHelper(logger, mr, db, clientset, config, namespace)
			if deleteErr != nil {
				return deleteErr
			}
			return err
		}
	}
	return nil
}

// DeleteScheduler deletes a scheduler from a yaml configuration
func DeleteScheduler(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db interfaces.DB, clientset kubernetes.Interface, configName string) error {
	config := models.NewConfig(configName, "", "")
	err := mr.WithSegment(models.SegmentSelect, func() error {
		return config.Load(db)
	})
	if err != nil {
		return err
	}
	namespace := models.NewNamespace(config.Name)
	return deleteSchedulerHelper(logger, mr, db, clientset, config, namespace)
}

// GetSchedulerScalingInfo returns the scheduler scaling policies and room count by status
func GetSchedulerScalingInfo(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db interfaces.DB, configName string) (*models.AutoScaling, *models.RoomsStatusCount, error) {
	config := models.NewConfig(configName, "", "")
	err := mr.WithSegment(models.SegmentSelect, func() error {
		return config.Load(db)
	})
	if err != nil {
		return nil, nil, err
	}
	scalingPolicy := config.GetAutoScalingPolicy()
	var roomCountByStatus *models.RoomsStatusCount
	err = mr.WithSegment(models.SegmentGroupBy, func() error {
		roomCountByStatus, err = models.GetRoomsCountByStatus(db, config.ID)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return scalingPolicy, roomCountByStatus, nil
}

// GetSchedulerStateInfo returns the scheduler state information
func GetSchedulerStateInfo(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, client interfaces.RedisClient, configName string) (*models.SchedulerState, error) {
	state := models.NewSchedulerState(configName, "", 0, 0)
	err := mr.WithSegment(models.SegmentHGetAll, func() error {
		return state.Load(client)
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}

// SaveSchedulerStateInfo updates the scheduler state information
func SaveSchedulerStateInfo(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, client interfaces.RedisClient, state *models.SchedulerState) error {
	return mr.WithSegment(models.SegmentHMSet, func() error {
		return state.Save(client)
	})
}

// ScaleUp scales up a scheduler using its config
func ScaleUp(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db interfaces.DB, clientset kubernetes.Interface, configName string) error {
	config := models.NewConfig(configName, "", "")
	err := mr.WithSegment(models.SegmentSelect, func() error {
		return config.Load(db)
	})
	if err != nil {
		return err
	}
	configYAML, _ := models.NewConfigYAML(config.YAML)

	// TODO: optimize creation (in parallel?)
	for i := 0; i < configYAML.AutoScaling.Up.Delta; i++ {
		err := createServiceAndPod(logger, mr, db, clientset, configYAML, config.ID)
		if err != nil {
			return err
		}
	}
	// TODO: set SchedulerState.State back to "in-sync"
	return nil
}

func deleteSchedulerHelper(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db interfaces.DB, clientset kubernetes.Interface, config *models.Config, namespace *models.Namespace) error {
	err := mr.WithSegment(models.SegmentNamespace, func() error {
		return namespace.Delete(clientset) // TODO: we're assuming that deleting a namespace gracefully terminates all its pods
	})
	if err != nil {
		logger.WithError(err).Error("Failed to delete namespace while rolling back cluster creation.")
		return err
	}
	err = mr.WithSegment(models.SegmentDelete, func() error {
		return config.Delete(db)
	})
	if err != nil {
		logger.WithError(err).Error("Failed to delete config while rolling back cluster creation.")
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

func createServiceAndPod(logger logrus.FieldLogger, mr *models.MixedMetricsReporter, db interfaces.DB, clientset kubernetes.Interface, configYAML *models.ConfigYAML, configID string) error {
	randID := strings.SplitN(uuid.NewV4().String(), "-", 2)[0]
	name := fmt.Sprintf("%s-%s", configYAML.Name, randID)
	room := models.NewRoom(name, configID)
	err := mr.WithSegment(models.SegmentInsert, func() error {
		return room.Create(db)
	})
	if err != nil {
		return err
	}
	service := models.NewService(name, configYAML.Name, configYAML.Ports)
	var kubeService *v1.Service
	err = mr.WithSegment(models.SegmentService, func() error {
		kubeService, err = service.Create(clientset)
		return err
	})
	if err != nil {
		return err
	}
	namesEnvVars := []*models.EnvVar{
		&models.EnvVar{
			Name:  "MAESTRO_SCHEDULER_NAME",
			Value: configYAML.Name,
		},
		&models.EnvVar{
			Name:  "MAESTRO_ROOM_NAME",
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
		configYAML.Limits.CPU,    // TODO: requests should be < limits calculate it
		configYAML.Limits.Memory, // TODO: requests should be < limits calculate it
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
		return err
	}
	nodeName := kubePod.Spec.NodeName
	logger.WithFields(logrus.Fields{
		"node": nodeName,
		"name": name,
	}).Info("Created GRU (service and pod) successfully.")
	return nil
}
