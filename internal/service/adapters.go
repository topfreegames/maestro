// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package service

import (
	"fmt"
	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"
	"time"

	operationadapters "github.com/topfreegames/maestro/internal/adapters/operation"

	eventsadapters "github.com/topfreegames/maestro/internal/adapters/events"

	scheduleradapters "github.com/topfreegames/maestro/internal/adapters/scheduler"

	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"

	"github.com/go-pg/pg"
	"github.com/go-redis/redis/v8"
	clockTime "github.com/topfreegames/maestro/internal/adapters/clock/time"
	instanceStorageRedis "github.com/topfreegames/maestro/internal/adapters/instance_storage/redis"
	portAllocatorRandom "github.com/topfreegames/maestro/internal/adapters/port_allocator/random"
	roomStorageRedis "github.com/topfreegames/maestro/internal/adapters/room_storage/redis"
	kubernetesRuntime "github.com/topfreegames/maestro/internal/adapters/runtime/kubernetes"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// configurations paths for the adapters
const (
	// Kubernetes runtime
	runtimeKubernetesMasterUrlPath  = "adapters.runtime.kubernetes.masterUrl"
	runtimeKubernetesKubeconfigPath = "adapters.runtime.kubernetes.kubeconfig"
	runtimeKubernetesInCluster      = "adapters.runtime.kubernetes.inCluster"
	// Redis operation storage
	operationStorageRedisUrlPath      = "adapters.operationStorage.redis.url"
	operationLeaseStorageRedisUrlPath = "adapters.operationLeaseStorage.redis.url"
	// Redis room storage
	roomStorageRedisUrlPath = "adapters.roomStorage.redis.url"
	// Redis scheduler cache
	schedulerCacheRedisUrlPath = "adapters.schedulerCache.redis.url"
	// Redis instance storage
	instanceStorageRedisUrlPath      = "adapters.instanceStorage.redis.url"
	instanceStorageRedisScanSizePath = "adapters.instanceStorage.redis.scanSize"
	// Random port allocator
	portAllocatorRandomRangePath = "adapters.portAllocator.random.range"
	// Postgres scheduler storage
	schedulerStoragePostgresUrlPath = "adapters.schedulerStorage.postgres.url"
	// Redis operation flow
	operationFlowRedisUrlPath = "adapters.operationFlow.redis.url"
	// Health Controller operation TTL
	healthControllerOperationTTL = "adapters.operationStorage.redis.operationsTtl.healthController"
)

func NewOperationManager(flow ports.OperationFlow, storage ports.OperationStorage, operationDefinitionConstructors map[string]operations.DefinitionConstructor, leaseStorage ports.OperationLeaseStorage, config operation_manager.OperationManagerConfig, schedulerStorage ports.SchedulerStorage) ports.OperationManager {
	return operation_manager.New(flow, storage, operationDefinitionConstructors, leaseStorage, config, schedulerStorage)
}

func NewRoomManager(clock ports.Clock, portAllocator ports.PortAllocator, roomStorage ports.RoomStorage, instanceStorage ports.GameRoomInstanceStorage, runtime ports.Runtime, eventsService ports.EventsService, config room_manager.RoomManagerConfig) ports.RoomManager {
	return room_manager.New(clock, portAllocator, roomStorage, instanceStorage, runtime, eventsService, config)
}

func NewEventsForwarder(c config.Config) (ports.EventsForwarder, error) {
	forwarderGrpc := eventsadapters.NewForwarderClient()
	return eventsadapters.NewEventsForwarder(forwarderGrpc), nil
}

func NewRuntimeKubernetes(c config.Config) (ports.Runtime, error) {
	var masterUrl string
	var kubeConfigPath string

	inCluster := c.GetBool(runtimeKubernetesInCluster)
	if !inCluster {
		masterUrl = c.GetString(runtimeKubernetesMasterUrlPath)
		kubeConfigPath = c.GetString(runtimeKubernetesKubeconfigPath)
	}

	clientSet, err := createKubernetesClient(masterUrl, kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Kubernetes runtime: %w", err)
	}

	return kubernetesRuntime.New(clientSet), nil
}

func NewOperationStorageRedis(clock ports.Clock, c config.Config) (ports.OperationStorage, error) {
	client, err := createRedisClient(c.GetString(operationStorageRedisUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis operation storage: %w", err)
	}

	operationsTTlMap := map[operationadapters.Definition]time.Duration{
		healthcontroller.OperationName: c.GetDuration(healthControllerOperationTTL),
	}

	return operationadapters.NewRedisOperationStorage(client, clock, operationsTTlMap), nil
}

func NewOperationLeaseStorageRedis(clock ports.Clock, c config.Config) (ports.OperationLeaseStorage, error) {
	client, err := createRedisClient(c.GetString(operationLeaseStorageRedisUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis operation lease storage: %w", err)
	}

	return operationadapters.NewRedisOperationLeaseStorage(client, clock), nil
}

func NewRoomStorageRedis(c config.Config) (ports.RoomStorage, error) {
	client, err := createRedisClient(c.GetString(roomStorageRedisUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis room storage: %w", err)
	}

	return roomStorageRedis.NewRedisStateStorage(client), nil
}

func NewGameRoomInstanceStorageRedis(c config.Config) (ports.GameRoomInstanceStorage, error) {
	client, err := createRedisClient(c.GetString(instanceStorageRedisUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis instance storage: %w", err)
	}

	return instanceStorageRedis.NewRedisInstanceStorage(client, c.GetInt(instanceStorageRedisScanSizePath)), nil
}

func NewSchedulerCacheRedis(c config.Config) (ports.SchedulerCache, error) {
	client, err := createRedisClient(c.GetString(schedulerCacheRedisUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis scheduler cache: %w", err)
	}

	return scheduleradapters.NewRedisSchedulerCache(client), nil
}

func NewClockTime() ports.Clock {
	return clockTime.NewClock()
}

func NewPortAllocatorRandom(c config.Config) (ports.PortAllocator, error) {
	portRange, err := entities.ParsePortRange(c.GetString(portAllocatorRandomRangePath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize random port allocator: %w", err)
	}

	return portAllocatorRandom.NewRandomPortAllocator(portRange), nil
}

func GetSchedulerStoragePostgresUrl(c config.Config) string {
	return c.GetString(schedulerStoragePostgresUrlPath)
}

func NewSchedulerStoragePg(c config.Config) (ports.SchedulerStorage, error) {
	opts, err := connectToPostgres(GetSchedulerStoragePostgresUrl(c))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize postgres scheduler storage: %w", err)
	}

	return scheduleradapters.NewSchedulerStorage(opts), nil
}

func createRedisClient(url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	return redis.NewClient(opts), nil
}

func NewOperationFlowRedis(c config.Config) (ports.OperationFlow, error) {
	client, err := createRedisClient(c.GetString(operationFlowRedisUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis operation storage: %w", err)
	}

	return operationadapters.NewRedisOperationFlow(client), nil
}

func connectToPostgres(url string) (*pg.Options, error) {
	opts, err := pg.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid postgres URL: %w", err)
	}

	return opts, nil
}

func createKubernetesClient(masterUrl, kubeconfigPath string) (kubernetes.Interface, error) {
	// NOTE: if neither masterUrl or kubeconfigPath are not passed, this will
	// fallback to in cluster config.
	kubeconfig, err := clientcmd.BuildConfigFromFlags(masterUrl, kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to construct kubernetes config: %w", err)
	}

	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return client, nil
}
