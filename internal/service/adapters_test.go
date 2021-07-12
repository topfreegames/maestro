//+build integration

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/orlangure/gnomock"
	predis "github.com/orlangure/gnomock/preset/redis"
	"github.com/stretchr/testify/require"
	configmock "github.com/topfreegames/maestro/internal/config/mock"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
)

func getRedisUrl(t *testing.T) string {
	redisContainer, err := gnomock.Start(predis.Preset())
	require.NoError(t, err)

	t.Cleanup(func() {
		gnomock.Stop(redisContainer)
	})

	return fmt.Sprintf("redis://%s", redisContainer.DefaultAddress())
}

func TestOperationStorageRedis(t *testing.T) {
	t.Parallel()
	clock := NewClockTime()

	t.Run("with valid redis", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(operationStorageRedisUrlPath).Return(getRedisUrl(t))
		opStorage, err := NewOperationStorageRedis(clock, config)
		require.NoError(t, err)

		_, _, err = opStorage.GetOperation(context.Background(), "", "")
		require.ErrorIs(t, err, errors.ErrNotFound)
	})

	t.Run("with invalid redis", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(operationStorageRedisUrlPath).Return("redis://somewhere-in-the-world:6379")
		opStorage, err := NewOperationStorageRedis(clock, config)
		require.NoError(t, err)

		_, _, err = opStorage.GetOperation(context.Background(), "", "")
		require.ErrorIs(t, err, errors.ErrUnexpected)
	})

	t.Run("with invalid configuration", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(operationStorageRedisUrlPath).Return("")
		_, err := NewOperationStorageRedis(clock, config)
		require.Error(t, err)
	})
}

func TestRoomStorageRedis(t *testing.T) {
	t.Parallel()

	t.Run("with valid redis", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(roomStorageRedisUrlPath).Return(getRedisUrl(t))
		roomStorage, err := NewRoomStorageRedis(config)
		require.NoError(t, err)

		_, err = roomStorage.GetRoom(context.Background(), "", "")
		require.ErrorIs(t, err, errors.ErrNotFound)
	})

	t.Run("with invalid redis", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(roomStorageRedisUrlPath).Return("redis://somewhere-in-the-world:6379")
		roomStorage, err := NewRoomStorageRedis(config)
		require.NoError(t, err)

		_, err = roomStorage.GetRoom(context.Background(), "", "")
		require.ErrorIs(t, err, errors.ErrUnexpected)
	})

	t.Run("with invalid configuration", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(roomStorageRedisUrlPath).Return("")
		_, err := NewRoomStorageRedis(config)
		require.Error(t, err)
	})
}

func TestInstanceStorageRedis(t *testing.T) {
	t.Parallel()

	t.Run("with valid redis", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(instanceStorageRedisUrlPath).Return(getRedisUrl(t))
		config.EXPECT().GetInt(instanceStorageRedisScanSizePath).Return(10)
		instanceStorage, err := NewGameRoomInstanceStorageRedis(config)
		require.NoError(t, err)

		_, err = instanceStorage.GetInstance(context.Background(), "", "")
		require.ErrorIs(t, err, errors.ErrNotFound)
	})

	t.Run("with invalid redis", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(instanceStorageRedisUrlPath).Return("redis://somewhere-in-the-world:6379")
		config.EXPECT().GetInt(instanceStorageRedisScanSizePath).Return(10)
		instanceStorage, err := NewGameRoomInstanceStorageRedis(config)
		require.NoError(t, err)

		_, err = instanceStorage.GetInstance(context.Background(), "", "")
		require.ErrorIs(t, err, errors.ErrUnexpected)
	})

	t.Run("with invalid configuration", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(instanceStorageRedisUrlPath).Return("")
		_, err := NewGameRoomInstanceStorageRedis(config)
		require.Error(t, err)
	})
}

func TestPortAllocatorRandom(t *testing.T) {
	t.Parallel()

	t.Run("with valid range", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(portAllocatorRandomRangePath).Return("1000-2000")
		portAllocator, err := NewPortAllocatorRandom(config)
		require.NoError(t, err)

		_, err = portAllocator.Allocate(nil, 1)
		require.NoError(t, err)
	})

	t.Run("with invalid range", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(portAllocatorRandomRangePath).Return("abc")
		_, err := NewPortAllocatorRandom(config)
		require.Error(t, err)
	})
}

// TODO(gabrielcorado): test running some command on PG
func TestSchedulerStoragePostgres(t *testing.T) {
	t.Parallel()

	t.Run("with valid configuration", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(schedulerStoragePostgresUrlPath).Return("postgres://somewhere:5432/db")
		_, err := NewSchedulerStoragePg(config)
		require.NoError(t, err)
	})

	t.Run("with invalid configuration", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(schedulerStoragePostgresUrlPath).Return("")
		_, err := NewSchedulerStoragePg(config)
		require.Error(t, err)
	})
}
