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

	"github.com/Sirupsen/logrus"

	"github.com/topfreegames/extensions/interfaces"
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

// CreateScheduler creates a new scheduler from a yaml configuration
func CreateScheduler(logger *logrus.Logger, db interfaces.DB, clientset kubernetes.Interface, yamlString string) error {
	configYAML, err := models.NewConfigYAML(yamlString)
	if err != nil {
		return err
	}
	config := models.NewConfig(configYAML.Name, configYAML.Game, yamlString)
	// TODO: new relic support
	err = config.Create(db)
	if err != nil {
		return err
	}

	namespace := models.NewNamespace(config.Name)
	err = namespace.Create(clientset)
	if err != nil {
		deleteSchedulerHelper(logger, db, clientset, config, namespace)
		return err
	}

	// TODO: optimize creation (in parallel?)
	for i := 0; i < configYAML.AutoScaling.Min; i++ {
		name := fmt.Sprintf("%s-%d", configYAML.Name, i)
		err := createServiceAndPod(logger, db, clientset, configYAML, config.ID, name)
		if err != nil {
			deleteSchedulerHelper(logger, db, clientset, config, namespace)
			return err
		}
	}
	return nil
}

// DeleteScheduler deletes a scheduler from a yaml configuration
func DeleteScheduler(logger *logrus.Logger, db interfaces.DB, clientset kubernetes.Interface, yamlString string) error {
	configYAML, err := models.NewConfigYAML(yamlString)
	if err != nil {
		return err
	}
	config := models.NewConfig(configYAML.Name, configYAML.Game, yamlString)
	namespace := models.NewNamespace(config.Name)
	return deleteSchedulerHelper(logger, db, clientset, config, namespace)
}

// GetSchedulerScalingInfo returns the scheduler scaling policies and room count by status
func GetSchedulerScalingInfo(logger *logrus.Logger, db interfaces.DB, configName string) (*models.AutoScaling, map[string]int, error) {
	config := models.NewConfig(configName, "", "")
	// TODO: new relic support
	err := config.Load(db)
	if err != nil {
		return nil, nil, err
	}
	scalingPolicy := config.GetAutoScalingPolicy()
	roomCountByStatus, err := models.GetRoomsCountByStatus(db, config.ID)
	if err != nil {
		return nil, nil, err
	}
	return scalingPolicy, roomCountByStatus, nil
}

func deleteSchedulerHelper(logger *logrus.Logger, db interfaces.DB, clientset kubernetes.Interface, config *models.Config, namespace *models.Namespace) error {
	err := namespace.Delete(clientset) // TODO: we're assuming that deleting a namespace gracefully terminates all its pods
	if err != nil {
		logger.WithError(err).Error("Failed to delete namespace while rolling back config creation.")
		return err
	}
	err = config.Delete(db)
	if err != nil {
		logger.WithError(err).Error("Failed to delete config while rolling back config creation.")
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

func createServiceAndPod(logger *logrus.Logger, db interfaces.DB, clientset kubernetes.Interface, configYAML *models.ConfigYAML, configID, name string) error {
	room := models.NewRoom(name, configID)
	err := room.Create(db)
	if err != nil {
		return err
	}
	service := models.NewService(name, configYAML.Name, configYAML.Ports)
	kubeService, err := service.Create(clientset)
	if err != nil {
		return err
	}
	namespaceEnvVar := &models.EnvVar{
		Name:  "MAESTRO_NAMESPACE",
		Value: configYAML.Name,
	}
	env := append(configYAML.Env, namespaceEnvVar)
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
	kubePod, err := pod.Create(clientset)
	if err != nil {
		return err
	}
	nodeName := kubePod.Spec.NodeName
	logger.WithFields(logrus.Fields{
		"node": nodeName,
		"name": name,
	}).Info("Created service and pod successfully.")
	// TODO: also create rooms with status pending
	return nil
}
