// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"github.com/sirupsen/logrus"
	pginterfaces "github.com/topfreegames/extensions/pg/interfaces"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

//MetricsReporter is a contract for reporters of metrics
type MetricsReporter interface {
	StartSegment(string) map[string]interface{}
	EndSegment(map[string]interface{}, string)

	StartDatastoreSegment(datastore, collection, operation string) map[string]interface{}
	EndDatastoreSegment(map[string]interface{})

	StartExternalSegment(string) map[string]interface{}
	EndExternalSegment(map[string]interface{})
}

// ContainerIface returns container properties
type ContainerIface interface {
	GetImage() string
	GetName() string
	GetPorts() []*Port
	GetLimits() *Resources
	GetRequests() *Resources
	GetCmd() []string
	GetEnv() []*EnvVar
}

// AddrGetter return IP and ports of a room
type AddrGetter interface {
	Get(room *Room, kubernetesClient kubernetes.Interface) (*RoomAddresses, error)
}

// RoomManager should create and delete a game room
type RoomManager interface {
	Create(
		logger logrus.FieldLogger,
		mr *MixedMetricsReporter,
		redisClient redisinterfaces.RedisClient,
		db pginterfaces.DB,
		clientset kubernetes.Interface,
		configYAML *ConfigYAML,
		scheduler *Scheduler) (*v1.Pod, error)
	Delete(
		logger logrus.FieldLogger,
		mr *MixedMetricsReporter,
		clientset kubernetes.Interface,
		redisClient redisinterfaces.RedisClient,
		configYaml *ConfigYAML,
		name, reason string,
	) error
}
