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
	"strings"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/go-redis/redis/extra/redisotel/v8"
	"github.com/go-redis/redis/v8"
	schedulerredis "github.com/topfreegames/maestro/internal/adapters/cache/redis/scheduler"
	clockTime "github.com/topfreegames/maestro/internal/adapters/clock/time"
	eventsadapters "github.com/topfreegames/maestro/internal/adapters/events"
	"github.com/topfreegames/maestro/internal/adapters/flow/redis/operation"
	operation2 "github.com/topfreegames/maestro/internal/adapters/lease/redis/operation"
	portAllocatorRandom "github.com/topfreegames/maestro/internal/adapters/portallocator/random"
	kubernetesRuntime "github.com/topfreegames/maestro/internal/adapters/runtime/kubernetes"
	"github.com/topfreegames/maestro/internal/adapters/storage/postgres/scheduler"
	instanceStorageRedis "github.com/topfreegames/maestro/internal/adapters/storage/redis/instance"
	redis2 "github.com/topfreegames/maestro/internal/adapters/storage/redis/operation"
	roomStorageRedis "github.com/topfreegames/maestro/internal/adapters/storage/redis/room"
	"github.com/topfreegames/maestro/internal/adapters/tracing"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/entities/port"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"
	"github.com/topfreegames/maestro/internal/core/operations/rooms/add"
	"github.com/topfreegames/maestro/internal/core/operations/rooms/remove"
	"github.com/topfreegames/maestro/internal/core/operations/storagecleanup"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies/roomoccupancy"
	operationservice "github.com/topfreegames/maestro/internal/core/services/operations"
	"github.com/topfreegames/maestro/internal/core/services/rooms"
	"github.com/topfreegames/maestro/internal/core/services/schedulers"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc/keepalive"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// configurations paths for the adapters
const (
	// GRPC KeepAlive Configs
	grpcKeepAliveTimePath    = "adapters.grpc.keepalive.time"
	grpcKeepAliveTimeoutPath = "adapters.grpc.keepalive.timeout"
	// Kubernetes runtime
	runtimeKubernetesMasterURLPath                                 = "adapters.runtime.kubernetes.masterUrl"
	runtimeKubernetesKubeconfigPath                                = "adapters.runtime.kubernetes.kubeconfig"
	runtimeKubernetesInClusterPath                                 = "adapters.runtime.kubernetes.inCluster"
	runtimeKubernetesQPS                                           = "adapters.runtime.kubernetes.qps"
	runtimeKubernetesBurst                                         = "adapters.runtime.kubernetes.burst"
	runtimeKubernetesTopologySpreadEnabled                         = "adapters.runtime.kubernetes.topologySpreadConstraint.enabled"
	runtimeKubernetesTopologySpreadMaxSkew                         = "adapters.runtime.kubernetes.topologySpreadConstraint.maxSkew"
	runtimeKubernetesTopologySpreadTopologyKey                     = "adapters.runtime.kubernetes.topologySpreadConstraint.topologyKey"
	runtimeKubernetesTopologySpreadWhenUnsatisfiableScheduleAnyway = "adapters.runtime.kubernetes.topologySpreadConstraint.whenUnsatisfiableScheduleAnyway"
	// Redis operation storage
	operationStorageRedisURLPath      = "adapters.operationStorage.redis.url"
	operationLeaseStorageRedisURLPath = "adapters.operationLeaseStorage.redis.url"
	// Redis room storage
	roomStorageRedisURLPath = "adapters.roomStorage.redis.url"
	// Redis scheduler cache
	schedulerCacheRedisURLPath = "adapters.schedulerCache.redis.url"
	// Redis instance storage
	instanceStorageRedisURLPath      = "adapters.instanceStorage.redis.url"
	instanceStorageRedisScanSizePath = "adapters.instanceStorage.redis.scanSize"
	// Redis operation flow
	operationFlowRedisURLPath = "adapters.operationFlow.redis.url"
	// Redis configs
	redisPoolSizePath = "adapters.redis.poolSize"
	// Random port allocator
	portAllocatorRandomRangePath = "adapters.portAllocator.random.range"
	// Postgres scheduler storage
	schedulerStoragePostgresURLPath = "adapters.schedulerStorage.postgres.url"

	// operation TTL
	operationsTTLPath = "workers.redis.operationsTtl"
)

// NewSchedulerManager instantiates a new scheduler manager.
func NewSchedulerManager(schedulerStorage ports.SchedulerStorage, schedulerCache ports.SchedulerCache, operationManager ports.OperationManager, roomStorage ports.RoomStorage) ports.SchedulerManager {
	return schedulers.NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)
}

// NewOperationManager instantiates a new operation manager
func NewOperationManager(flow ports.OperationFlow, storage ports.OperationStorage, operationDefinitionConstructors map[string]operations.DefinitionConstructor, leaseStorage ports.OperationLeaseStorage, config operationservice.OperationManagerConfig, schedulerStorage ports.SchedulerStorage) ports.OperationManager {
	return operationservice.New(flow, storage, operationDefinitionConstructors, leaseStorage, config, schedulerStorage)
}

// NewRoomManager instantiates a room manager.
func NewRoomManager(clock ports.Clock, portAllocator ports.PortAllocator, roomStorage ports.RoomStorage, schedulerStorage ports.SchedulerStorage, instanceStorage ports.GameRoomInstanceStorage, runtime ports.Runtime, eventsService ports.EventsService, config rooms.RoomManagerConfig) ports.RoomManager {
	return rooms.New(clock, portAllocator, roomStorage, schedulerStorage, instanceStorage, runtime, eventsService, config)
}

// NewEventsForwarder instantiates GRPC as events forwarder.
func NewEventsForwarder(c config.Config) (ports.EventsForwarder, error) {
	keepAliveCfg := keepalive.ClientParameters{
		Time:    c.GetDuration(grpcKeepAliveTimePath),
		Timeout: c.GetDuration(grpcKeepAliveTimeoutPath),
	}
	forwarderGrpc := eventsadapters.NewForwarderClient(keepAliveCfg)
	return eventsadapters.NewEventsForwarder(forwarderGrpc), nil
}

// NewRuntimeKubernetes instantiates kubernetes as runtime.
func NewRuntimeKubernetes(c config.Config) (ports.Runtime, error) {
	var masterURL string
	var kubeConfigPath string
	var qps int
	var burst int

	inCluster := c.GetBool(runtimeKubernetesInClusterPath)
	if inCluster {
		qps = c.GetInt(runtimeKubernetesQPS)
		burst = c.GetInt(runtimeKubernetesBurst)
	} else {
		masterURL = c.GetString(runtimeKubernetesMasterURLPath)
		kubeConfigPath = c.GetString(runtimeKubernetesKubeconfigPath)
	}

	clientSet, err := createKubernetesClient(masterURL, kubeConfigPath, func(conf *rest.Config) {
		conf.QPS = float32(qps)
		conf.Burst = burst
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Kubernetes runtime: %w", err)
	}

	return kubernetesRuntime.New(clientSet, kubernetesRuntime.KubernetesConfig{
		TopologySpreadConstraintConfig: kubernetesRuntime.TopologySpreadConstraintConfig{
			Enabled:                         c.GetBool(runtimeKubernetesTopologySpreadEnabled),
			MaxSkew:                         c.GetInt(runtimeKubernetesTopologySpreadMaxSkew),
			TopologyKey:                     c.GetString(runtimeKubernetesTopologySpreadTopologyKey),
			WhenUnsatisfiableScheduleAnyway: c.GetBool(runtimeKubernetesTopologySpreadWhenUnsatisfiableScheduleAnyway),
		},
	}), nil
}

// NewOperationStorageRedis instantiates redis as operation storage.
func NewOperationStorageRedis(clock ports.Clock, operationDefinitionProviders map[string]operations.DefinitionConstructor, c config.Config) (ports.OperationStorage, error) {
	shouldRetryOperation := true
	client, err := createRedisClient(c, c.GetString(operationStorageRedisURLPath), shouldRetryOperation)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis operation storage: %w", err)
	}

	operationsTTLPathMap := map[redis2.Definition]time.Duration{
		healthcontroller.OperationName: c.GetDuration(operationsTTLPath),
		add.OperationName:              c.GetDuration(operationsTTLPath),
		remove.OperationName:           c.GetDuration(operationsTTLPath),
		storagecleanup.OperationName:   c.GetDuration(operationsTTLPath),
	}

	return redis2.NewRedisOperationStorage(client, clock, operationsTTLPathMap, operationDefinitionProviders), nil
}

// NewOperationLeaseStorageRedis instantiates redis as operation lease storage.
func NewOperationLeaseStorageRedis(clock ports.Clock, c config.Config) (ports.OperationLeaseStorage, error) {
	shouldRetryOperation := true
	client, err := createRedisClient(c, c.GetString(operationLeaseStorageRedisURLPath), shouldRetryOperation)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis operation lease storage: %w", err)
	}

	return operation2.NewRedisOperationLeaseStorage(client, clock), nil
}

// NewRoomStorageRedis instantiates redis as room storage.
func NewRoomStorageRedis(c config.Config) (ports.RoomStorage, error) {
	shouldRetryOperation := false
	client, err := createRedisClient(c, c.GetString(roomStorageRedisURLPath), shouldRetryOperation)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis room storage: %w", err)
	}

	return roomStorageRedis.NewRedisStateStorage(client), nil
}

// NewGameRoomInstanceStorageRedis instantiates redis as instance storage.
func NewGameRoomInstanceStorageRedis(c config.Config) (ports.GameRoomInstanceStorage, error) {
	shouldRetryOperation := true
	client, err := createRedisClient(c, c.GetString(instanceStorageRedisURLPath), shouldRetryOperation)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis instance storage: %w", err)
	}

	return instanceStorageRedis.NewRedisInstanceStorage(client, c.GetInt(instanceStorageRedisScanSizePath)), nil
}

// NewSchedulerCacheRedis instantiates redis as scheduler cache.
func NewSchedulerCacheRedis(c config.Config) (ports.SchedulerCache, error) {
	shouldRetryOperation := true
	client, err := createRedisClient(c, c.GetString(schedulerCacheRedisURLPath), shouldRetryOperation)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis scheduler cache: %w", err)
	}

	return schedulerredis.NewRedisSchedulerCache(client), nil
}

// NewClockTime instantiates a new clock.
func NewClockTime() ports.Clock {
	return clockTime.NewClock()
}

// NewPortAllocatorRandom instantiates a new port allocator.
func NewPortAllocatorRandom(c config.Config) (ports.PortAllocator, error) {
	portRange, err := port.ParsePortRange(c.GetString(portAllocatorRandomRangePath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize random port allocator: %w", err)
	}

	return portAllocatorRandom.NewRandomPortAllocator(portRange), nil
}

// NewSchedulerStoragePg instantiates a postgres connection as scheduler storage.
func NewSchedulerStoragePg(c config.Config) (ports.SchedulerStorage, error) {
	opts, err := connectToPostgres(GetSchedulerStoragePostgresURL(c))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize postgres scheduler storage: %w", err)
	}

	pgStorage := scheduler.NewSchedulerStorage(opts)

	if tracing.IsTracingEnabled(c) {
		pgStorage.EnableTracing()
	}

	return pgStorage, nil
}

// GetSchedulerStoragePostgresURL get scheduler storage postgres URL.
func GetSchedulerStoragePostgresURL(c config.Config) string {
	return c.GetString(schedulerStoragePostgresURLPath)
}

func createRedisClient(c config.Config, url string, shouldRetryOperation bool) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	opts.PoolSize = c.GetInt(redisPoolSizePath)
	if opts.TLSConfig != nil {
		opts.TLSConfig.InsecureSkipVerify = true
	}

	hostPort := strings.Split(opts.Addr, ":")

	if !shouldRetryOperation {
		opts.MaxRetries = -1
	}

	client := redis.NewClient(opts)

	if tracing.IsTracingEnabled(c) {
		client.AddHook(redisotel.NewTracingHook(redisotel.WithAttributes(
			semconv.NetPeerNameKey.String(hostPort[0]),
			semconv.NetPeerPortKey.String(hostPort[1])),
		))
	}

	return client, nil
}

// NewPolicyMap instantiates a new policy to be used by autoscaler expecting a room storage as parameter.
func NewPolicyMap(roomStorage ports.RoomStorage) autoscaler.PolicyMap {
	return autoscaler.PolicyMap{
		autoscaling.RoomOccupancy: roomoccupancy.NewPolicy(roomStorage),
	}
}

// NewAutoscaler instantiates  a new autoscaler expecting a Policy Map as parameter.
func NewAutoscaler(policies autoscaler.PolicyMap) ports.Autoscaler {
	return autoscaler.NewAutoscaler(policies)
}

// NewOperationFlowRedis instantiates a new operation flow using redis as backend.
func NewOperationFlowRedis(c config.Config) (ports.OperationFlow, error) {
	shouldRetryOperation := true
	client, err := createRedisClient(c, c.GetString(operationFlowRedisURLPath), shouldRetryOperation)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis operation storage: %w", err)
	}

	return operation.NewRedisOperationFlow(client), nil
}

func connectToPostgres(url string) (*pg.Options, error) {
	opts, err := pg.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid postgres URL: %w", err)
	}

	return opts, nil
}

func createKubernetesClient(masterURL, kubeconfigPath string, opts ...func(*rest.Config)) (kubernetes.Interface, error) {
	// NOTE: if neither masterURL or kubeconfigPath are not passed, this will
	// fallback to in cluster config.
	kubeconfig, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to construct kubernetes config: %w", err)
	}

	for _, opt := range opts {
		opt(kubeconfig)
	}

	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return client, nil
}
