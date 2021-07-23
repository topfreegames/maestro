package service

import (
	"fmt"

	"github.com/go-pg/pg"
	legacyRedis "github.com/go-redis/redis"
	"github.com/go-redis/redis/v8"
	clockTime "github.com/topfreegames/maestro/internal/adapters/clock/time"
	instanceStorageRedis "github.com/topfreegames/maestro/internal/adapters/instance_storage/redis"
	operationFlowRedis "github.com/topfreegames/maestro/internal/adapters/operation_flow/redis"
	operationStorageRedis "github.com/topfreegames/maestro/internal/adapters/operation_storage/redis"
	portAllocatorRandom "github.com/topfreegames/maestro/internal/adapters/port_allocator/random"
	roomStorageRedis "github.com/topfreegames/maestro/internal/adapters/room_storage/redis"
	kubernetesRuntime "github.com/topfreegames/maestro/internal/adapters/runtime/kubernetes"
	schedulerStoragePg "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/pg"
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
	// Redis operation storage
	operationStorageRedisUrlPath = "adapters.operationStorage.redis.url"
	// Redis room storage
	roomStorageRedisUrlPath = "adapters.roomStorage.redis.url"
	// Redis instance storage
	instanceStorageRedisUrlPath      = "adapters.instanceStorage.redis.url"
	instanceStorageRedisScanSizePath = "adapters.instanceStorage.redis.scanSize"
	// Random port allocator
	portAllocatorRandomRangePath = "adapters.portAllocator.random.range"
	// Postgres scheduler storage
	schedulerStoragePostgresUrlPath = "adapters.schedulerStorage.postgres.url"
	// Redis operation flow
	operationFlowRedisUrlPath = "adapters.operationFlow.redis.url"
)

func NewRuntimeKubernetes(c config.Config) (ports.Runtime, error) {
	clientset, err := createKubernetesClient(
		c.GetString(runtimeKubernetesMasterUrlPath),
		c.GetString(runtimeKubernetesKubeconfigPath),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Kubernetes runtime: %w", err)
	}

	return kubernetesRuntime.New(clientset), nil
}

func NewOperationStorageRedis(clock ports.Clock, c config.Config) (ports.OperationStorage, error) {
	client, err := createRedisClient(c.GetString(operationStorageRedisUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis operation storage: %w", err)
	}

	return operationStorageRedis.NewRedisOperationStorage(client, clock), nil
}

func NewRoomStorageRedis(c config.Config) (ports.RoomStorage, error) {
	client, err := createLegacyRedisClient(c.GetString(roomStorageRedisUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis room storage: %w", err)
	}

	return roomStorageRedis.NewRedisStateStorage(client), nil
}

func NewGameRoomInstanceStorageRedis(c config.Config) (ports.GameRoomInstanceStorage, error) {
	client, err := createLegacyRedisClient(c.GetString(instanceStorageRedisUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis instance storage: %w", err)
	}

	return instanceStorageRedis.NewRedisInstanceStorage(client, c.GetInt(instanceStorageRedisScanSizePath)), nil
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

func NewSchedulerStoragePg(c config.Config) (ports.SchedulerStorage, error) {
	opts, err := connectToPostgres(c.GetString(schedulerStoragePostgresUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize postgres scheduler storage: %w", err)
	}

	return schedulerStoragePg.NewSchedulerStorage(opts), nil
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

	return operationFlowRedis.NewRedisOperationFlow(client), nil
}

// NOTE: We need this because some of our adapters are still using the old
// version.
func createLegacyRedisClient(url string) (*legacyRedis.Client, error) {
	opts, err := legacyRedis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	return legacyRedis.NewClient(opts), nil
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
