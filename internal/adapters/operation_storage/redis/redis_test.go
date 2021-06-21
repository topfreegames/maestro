package redis

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/orlangure/gnomock"
	predis "github.com/orlangure/gnomock/preset/redis"
	"github.com/stretchr/testify/require"
	clockmock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
)

var dbNumber int32 = 0
var redisContainer *gnomock.Container

func getRedisConnection(t *testing.T) *redis.Client {
	db := atomic.AddInt32(&dbNumber, 1)

	client := redis.NewClient(&redis.Options{
		Addr: redisContainer.DefaultAddress(),
		DB:   int(db),
	})

	t.Cleanup(func() {
		client.FlushDB(context.Background())
	})

	return client
}

func TestMain(m *testing.M) {
	var err error
	redisContainer, err = gnomock.Start(predis.Preset())

	if err != nil {
		panic(fmt.Sprintf("error creating redis docker instance: %s\n", err))
	}

	code := m.Run()

	_ = gnomock.Stop(redisContainer)
	os.Exit(code)
}

func TestCreateOperation(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		client := getRedisConnection(t)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: "test-definition",
		}

		contents := []byte("hello test")
		err := storage.CreateOperation(context.Background(), op, contents)
		require.NoError(t, err)

		opID, err := client.LPop(context.Background(), storage.buildSchedulerPendingOperationsKey(op.SchedulerName)).Result()
		require.NoError(t, err)
		require.Equal(t, op.ID, opID)

		operationStored, err := client.HGetAll(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, opID)).Result()
		require.NoError(t, err)
		require.Equal(t, op.ID, operationStored[idRedisKey])
		require.Equal(t, op.SchedulerName, operationStored[schedulerNameRedisKey])
		require.Equal(t, op.DefinitionName, operationStored[definitionNameRedisKey])
		require.Equal(t, string(contents), operationStored[definitionContentsRedisKey])

		intStatus, err := strconv.Atoi(operationStored[statusRedisKey])
		require.NoError(t, err)
		require.Equal(t, op.Status, operation.Status(intStatus))
	})

	t.Run("fails on redis", func(t *testing.T) {
		client := getRedisConnection(t)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: "test-definition",
		}

		// "drop" redis connection
		client.Close()

		err := storage.CreateOperation(context.Background(), op, []byte{})
		require.ErrorIs(t, errors.ErrUnexpected, err)
	})
}

func TestGetOperation(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		client := getRedisConnection(t)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: "test-definition",
		}

		contents := []byte("hello test")
		err := storage.CreateOperation(context.Background(), op, contents)
		require.NoError(t, err)

		operationStored, definitionContents, err := storage.GetOperation(context.Background(), op.SchedulerName, op.ID)
		require.NoError(t, err)
		require.Equal(t, contents, definitionContents)
		require.Equal(t, op.ID, operationStored.ID)
		require.Equal(t, op.SchedulerName, operationStored.SchedulerName)
		require.Equal(t, op.Status, operationStored.Status)
		require.Equal(t, op.DefinitionName, operationStored.DefinitionName)
	})

	t.Run("not found", func(t *testing.T) {
		client := getRedisConnection(t)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)

		operationStored, definitionContents, err := storage.GetOperation(context.Background(), "test-scheduler", "inexistent-id")
		require.ErrorIs(t, errors.ErrNotFound, err)
		require.Nil(t, operationStored)
		require.Nil(t, definitionContents)
	})

	t.Run("fails on redis", func(t *testing.T) {
		client := getRedisConnection(t)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)

		// "drop" redis connection
		client.Close()

		operationStored, definitionContents, err := storage.GetOperation(context.Background(), "test-scheduler", "some-op-id")
		require.ErrorIs(t, errors.ErrUnexpected, err)
		require.Nil(t, operationStored)
		require.Nil(t, definitionContents)
	})
}

func TestNextSchedulerOperationID(t *testing.T) {
	t.Run("successfully receives the operation", func(t *testing.T) {
		client := getRedisConnection(t)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusPending,
			DefinitionName: "test-definition",
		}

		contents := []byte("hello test")
		err := storage.CreateOperation(context.Background(), op, contents)
		require.NoError(t, err)

		opID, err := storage.NextSchedulerOperationID(context.Background(), op.SchedulerName)
		require.NoError(t, err)
		require.Equal(t, op.ID, opID)
	})

	t.Run("failed with context canceled", func(t *testing.T) {
		client := getRedisConnection(t)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)

		ctx, cancel := context.WithCancel(context.Background())
		nextWait := make(chan error)

		go func() {
			_, err := storage.NextSchedulerOperationID(ctx, "some-random-scheduler")
			nextWait <- err
		}()

		cancel()

		require.Eventually(t, func() bool {
			select {
			case err := <-nextWait:
				require.Error(t, err)
				return true
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("failed redis connection", func(t *testing.T) {
		client := getRedisConnection(t)
		clock := clockmock.NewFakeClock(time.Now())
		storage := NewRedisOperationStorage(client, clock)

		nextWait := make(chan error)
		go func() {
			_, err := storage.NextSchedulerOperationID(context.Background(), "some-random-scheduler")
			nextWait <- err
		}()

		client.Close()

		require.Eventually(t, func() bool {
			select {
			case err := <-nextWait:
				require.Error(t, err)
				return true
			default:
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)
	})
}

func TestSetActiveOperation(t *testing.T) {
	t.Run("set operation as active", func (t *testing.T) {
		client := getRedisConnection(t)
		now := time.Now()
		clock := clockmock.NewFakeClock(now)
		storage := NewRedisOperationStorage(client, clock)

		op := &operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "test-scheduler",
			Status:         operation.StatusInProgress,
		}

		err := storage.SetOperationActive(context.Background(), op)
		require.NoError(t, err)

		operationStored, err := client.HGetAll(context.Background(), storage.buildSchedulerOperationKey(op.SchedulerName, op.ID)).Result()
		require.NoError(t, err)

		intStatus, err := strconv.Atoi(operationStored[statusRedisKey])
		require.NoError(t, err)
		require.Equal(t, op.Status, operation.Status(intStatus))

		score, err := client.ZScore(context.Background(), storage.buildSchedulerActiveOperationsKey(op.SchedulerName), op.ID).Result()
		require.NoError(t, err)
		require.Equal(t, float64(now.Unix()), score)
	})
}
