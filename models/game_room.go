// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"fmt"
	"strings"

	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// GameRoom is the struct that manages game rooms on production environment
type GameRoom struct{}

// Create creates a new pod
func (g *GameRoom) Create(
	logger logrus.FieldLogger,
	mr *MixedMetricsReporter,
	redisClient redisinterfaces.RedisClient,
	db pginterfaces.DB,
	clientset kubernetes.Interface,
	configYAML *ConfigYAML,
	scheduler *Scheduler,
) (*v1.Pod, error) {
	return createPod(
		logger,
		mr,
		redisClient,
		db,
		clientset,
		configYAML,
		scheduler,
	)
}

// Delete removes a pod
func (g *GameRoom) Delete(
	logger logrus.FieldLogger,
	mr *MixedMetricsReporter,
	clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
	configYaml *ConfigYAML,
	name, reason string,
) error {
	return deletePod(
		logger,
		mr,
		clientset,
		redisClient,
		configYaml,
		name,
		reason,
	)
}

// GameRoomWithService is the struct that manages game rooms on development environment
type GameRoomWithService struct{}

// Create creates a new pod with a NodePort service exposing it
func (g *GameRoomWithService) Create(
	logger logrus.FieldLogger,
	mr *MixedMetricsReporter,
	redisClient redisinterfaces.RedisClient,
	db pginterfaces.DB,
	clientset kubernetes.Interface,
	configYAML *ConfigYAML,
	scheduler *Scheduler,
) (*v1.Pod, error) {
	pod, err := createPod(
		logger,
		mr,
		redisClient,
		db,
		clientset,
		configYAML,
		scheduler,
	)
	if err != nil {
		return nil, err
	}

	svc := NewService(pod.Name, configYAML)
	_, err = svc.Create(clientset)
	if err != nil {
		deletePod(
			logger,
			mr,
			clientset,
			redisClient,
			configYAML,
			pod.Name,
			"failed_to_create_service_for_pod",
		)
		return nil, err
	}

	return pod, nil
}

// Delete removes a pod and the service that exposes it
func (g *GameRoomWithService) Delete(
	logger logrus.FieldLogger,
	mr *MixedMetricsReporter,
	clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
	configYAML *ConfigYAML,
	name, reason string,
) error {
	err := deletePod(
		logger,
		mr,
		clientset,
		redisClient,
		configYAML,
		name,
		reason,
	)
	if err != nil {
		return err
	}
	svc := NewService(name, configYAML)
	return svc.Delete(clientset, reason, configYAML)
}

func createPod(
	logger logrus.FieldLogger,
	mr *MixedMetricsReporter,
	redisClient redisinterfaces.RedisClient,
	db pginterfaces.DB,
	clientset kubernetes.Interface,
	configYAML *ConfigYAML,
	scheduler *Scheduler,
) (*v1.Pod, error) {
	randID := strings.SplitN(uuid.NewV4().String(), "-", 2)[0]
	name := fmt.Sprintf("%s-%s", configYAML.Name, randID)
	room := NewRoom(name, configYAML.Name)
	err := mr.WithSegment(SegmentInsert, func() error {
		return room.Create(redisClient, db, mr, configYAML)
	})
	if err != nil {
		return nil, err
	}
	namesEnvVars := []*EnvVar{
		{
			Name:  "MAESTRO_SCHEDULER_NAME",
			Value: configYAML.Name,
		},
		{
			Name:  "MAESTRO_ROOM_ID",
			Value: name,
		},
	}

	var pod *Pod
	if configYAML.Version() == "v1" {
		env := append(configYAML.Env, namesEnvVars...)
		err = mr.WithSegment(SegmentPod, func() error {
			var err error
			pod, err = NewPod(name, env, configYAML, clientset, redisClient)
			return err
		})
	} else if configYAML.Version() == "v2" {
		containers := make([]*Container, len(configYAML.Containers))
		for i, container := range configYAML.Containers {
			containers[i] = container.NewWithCopiedEnvs()
			containers[i].Env = append(containers[i].Env, namesEnvVars...)
		}

		pod, err = NewPodWithContainers(name, containers, configYAML, clientset, redisClient)
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

	pod.SetVersion(scheduler.Version)

	var kubePod *v1.Pod
	err = mr.WithSegment(SegmentPod, func() error {
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
	mr *MixedMetricsReporter,
	clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
	configYaml *ConfigYAML,
	name, reason string,
) error {
	pod := &Pod{
		Name:      name,
		Game:      configYaml.Game,
		Namespace: configYaml.Name,
	}
	err := mr.WithSegment(SegmentPod, func() error {
		return pod.Delete(clientset, redisClient, reason, configYaml)
	})
	if err != nil {
		return err
	}
	return nil
}
