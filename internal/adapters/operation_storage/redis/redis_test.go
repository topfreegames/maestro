package redis

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/go-redis/redis"
	"github.com/orlangure/gnomock"
	predis "github.com/orlangure/gnomock/preset/redis"
	"github.com/stretchr/testify/require"
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
		client.FlushDB()
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

// TODO(gabrielcorado): consider moving this "test" operation as a NoopOperation
// inside the operations.
type testOperationDefinition struct{}

func (d *testOperationDefinition) Marshal() []byte            { return []byte{} }
func (d *testOperationDefinition) Unmarshal(raw []byte) error { return nil }
func (d *testOperationDefinition) Name() string               { return "testOperationDefinition" }

func TestCreateOperation(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		client := getRedisConnection(t)
		storage := NewRedisOperationStorage(client)

		op := &operation.Operation{
			ID:            "some-op-id",
			SchedulerName: "test-scheduler",
			Status:        operation.StatusPending,
			Definition:    &testOperationDefinition{},
		}

		err := storage.CreateOperation(context.Background(), op)
		require.NoError(t, err)

		opID, err := client.LPop(storage.buildSchedulerPendingOperationsKey(op.SchedulerName)).Result()
		require.NoError(t, err)
		require.Equal(t, op.ID, opID)

		operationStored, err := client.HGetAll(storage.buildSchedulerOperationKey(op.SchedulerName, opID)).Result()
		require.NoError(t, err)
		require.Equal(t, op.ID, operationStored[idRedisKey])
		require.Equal(t, op.SchedulerName, operationStored[schedulerNameRedisKey])
		require.Equal(t, op.Definition.Name(), operationStored[definitionNameRedisKey])

		intStatus, err := strconv.Atoi(operationStored[statusRedisKey])
		require.NoError(t, err)
		require.Equal(t, op.Status, operation.Status(intStatus))
	})

	t.Run("fails on redis", func(t *testing.T) {
		client := getRedisConnection(t)
		storage := NewRedisOperationStorage(client)

		op := &operation.Operation{
			ID:            "some-op-id",
			SchedulerName: "test-scheduler",
			Status:        operation.StatusPending,
			Definition:    &testOperationDefinition{},
		}

		// "drop" redis connection
		client.Close()

		err := storage.CreateOperation(context.Background(), op)
		require.ErrorIs(t, errors.ErrUnexpected, err)
	})
}
