package service

import (
	"context"
	"fmt"

	"github.com/go-pg/pg"
	legacyredis "github.com/go-redis/redis"
	"github.com/go-redis/redis/v8"
	clocktime "github.com/topfreegames/maestro/internal/adapters/clock/time"
	instancestorageredis "github.com/topfreegames/maestro/internal/adapters/instance_storage/redis"
	operationstorageredis "github.com/topfreegames/maestro/internal/adapters/operation_storage/redis"
	roomstorageredis "github.com/topfreegames/maestro/internal/adapters/room_storage/redis"
	kubernetesruntime "github.com/topfreegames/maestro/internal/adapters/runtime/kubernetes"
	portallocatorrandom "github.com/topfreegames/maestro/internal/adapters/port_allocator/random"
	schedulerstoragepg "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/pg"
	"github.com/topfreegames/maestro/internal/config"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/entities"
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
	instanceStorageRedisUrlPath = "adapters.instanceStorage.redis.url"
	instanceStorageRedisScanSizePath = "adapters.instanceStorage.redis.scanSize"
	// Random port allocator
	portAllocatorRandomRangePath = "adapters.portallocator.random.range"
	// Postgres scheduler storage
	schedulerStoragePostgresUrlPath = "adapters.schedulerStorage.postgres.url"
)

func NewRuntimeKubernetes(ctx context.Context, c config.Config) (ports.Runtime, error) {
	clientset, err := createKubernetesClient(
		ctx,
		c.GetString(runtimeKubernetesMasterUrlPath),
		c.GetString(runtimeKubernetesKubeconfigPath),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Kubernetes runtime: %w", err)
	}

	return kubernetesruntime.New(clientset), nil
}

func NewOperationStorageRedis(ctx context.Context, clock ports.Clock, c config.Config) (ports.OperationStorage, error) {
	client, err := createRedisClient(ctx, c.GetString(operationStorageRedisUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis operation storage: %w", err)
	}

	return operationstorageredis.NewRedisOperationStorage(client, clock), nil
}

func NewRoomStorageRedis(ctx context.Context, c config.Config) (ports.RoomStorage, error) {
	client, err := createLegacyRedisClient(ctx, c.GetString(roomStorageRedisUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis room storage: %w", err)
	}

	return roomstorageredis.NewRedisStateStorage(client), nil
}

func NewGameRoomInstanceStorageRedis(ctx context.Context, c config.Config) (ports.GameRoomInstanceStorage, error) {
	client, err := createLegacyRedisClient(ctx, c.GetString(instanceStorageRedisUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis instance storage: %w", err)
	}

	return instancestorageredis.NewRedisInstanceStorage(client, c.GetInt(instanceStorageRedisScanSizePath)), nil
}

func NewClockTime() (ports.Clock, error) {
	return clocktime.NewClock(), nil
}

func NewPortAllocatorRandom(c config.Config) (ports.PortAllocator, error) {
	portRange, err := entities.ParsePortRange(c.GetString(portAllocatorRandomRangePath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize random port allocator: %w", err)
	}

	return portallocatorrandom.NewRandomPortAllocator(portRange), nil
}

func NewSchedulerStoragePg(ctx context.Context, c config.Config) (ports.SchedulerStorage, error) {
	opts, err := connectToPostgres(ctx, c.GetString(schedulerStoragePostgresUrlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize postgres scheduler storage: %w", err)
	}

	return schedulerstoragepg.NewSchedulerStorage(opts), nil
}

func createRedisClient(ctx context.Context, url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	return redis.NewClient(opts), nil
}

// NOTE: We need this because some of our adapters are still using the old
// version.
func createLegacyRedisClient(ctx context.Context, url string) (*legacyredis.Client, error) {
	opts, err := legacyredis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	return legacyredis.NewClient(opts), nil
}

func connectToPostgres(ctx context.Context, url string) (*pg.Options, error) {
	opts, err := pg.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid postgres URL: %w", err)
	}

	return opts, nil
}

func createKubernetesClient(ctx context.Context, masterUrl, kubeconfigPath string) (kubernetes.Interface, error) {
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
