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

//go:build integration
// +build integration

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/orlangure/gnomock"
	predis "github.com/orlangure/gnomock/preset/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	configmock "github.com/topfreegames/maestro/internal/config/mock"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/core/services/autoscaler/policies/roomoccupancy"
)

func getRedisUrl(t *testing.T) string {
	redisContainer, err := gnomock.Start(predis.Preset(predis.WithVersion("6.2.0")))
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = gnomock.Stop(redisContainer)
	})

	return fmt.Sprintf("redis://%s", redisContainer.DefaultAddress())
}

func TestOperationStorageRedis(t *testing.T) {
	t.Parallel()
	clock := NewClockTime()

	t.Run("with valid redis", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(operationStorageRedisUrlPath).Return(getRedisUrl(t))
		config.EXPECT().GetInt(redisPoolSize).Return(500)
		config.EXPECT().GetDuration(healthControllerOperationTTL).Return(time.Minute)
		opStorage, err := NewOperationStorageRedis(clock, config)
		require.NoError(t, err)

		_, err = opStorage.GetOperation(context.Background(), "", "")
		require.ErrorIs(t, err, errors.ErrNotFound)
	})

	t.Run("with invalid redis", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(operationStorageRedisUrlPath).Return("redis://somewhere-in-the-world:6379")
		config.EXPECT().GetDuration(healthControllerOperationTTL).Return(time.Minute)
		config.EXPECT().GetInt(redisPoolSize).Return(500)
		opStorage, err := NewOperationStorageRedis(clock, config)
		require.NoError(t, err)

		_, err = opStorage.GetOperation(context.Background(), "", "")
		require.ErrorIs(t, err, errors.ErrUnexpected)
	})

	t.Run("with invalid configuration", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

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

		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(roomStorageRedisUrlPath).Return(getRedisUrl(t))
		config.EXPECT().GetInt(redisPoolSize).Return(500)
		roomStorage, err := NewRoomStorageRedis(config)
		require.NoError(t, err)

		_, err = roomStorage.GetRoom(context.Background(), "", "")
		require.ErrorIs(t, err, errors.ErrNotFound)
	})

	t.Run("with invalid redis", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(roomStorageRedisUrlPath).Return("redis://somewhere-in-the-world:6379")
		config.EXPECT().GetInt(redisPoolSize).Return(500)
		roomStorage, err := NewRoomStorageRedis(config)
		require.NoError(t, err)

		_, err = roomStorage.GetRoom(context.Background(), "", "")
		require.ErrorIs(t, err, errors.ErrUnexpected)
	})

	t.Run("with invalid configuration", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

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

		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(instanceStorageRedisUrlPath).Return(getRedisUrl(t))
		config.EXPECT().GetInt(instanceStorageRedisScanSizePath).Return(10)
		config.EXPECT().GetInt(redisPoolSize).Return(500)
		instanceStorage, err := NewGameRoomInstanceStorageRedis(config)
		require.NoError(t, err)

		_, err = instanceStorage.GetInstance(context.Background(), "", "")
		require.ErrorIs(t, err, errors.ErrNotFound)
	})

	t.Run("with invalid redis", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(instanceStorageRedisUrlPath).Return("redis://somewhere-in-the-world:6379")
		config.EXPECT().GetInt(instanceStorageRedisScanSizePath).Return(10)
		config.EXPECT().GetInt(redisPoolSize).Return(500)
		instanceStorage, err := NewGameRoomInstanceStorageRedis(config)
		require.NoError(t, err)

		_, err = instanceStorage.GetInstance(context.Background(), "", "")
		require.ErrorIs(t, err, errors.ErrUnexpected)
	})

	t.Run("with invalid configuration", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

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

		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(schedulerStoragePostgresUrlPath).Return("postgres://somewhere:5432/db")
		_, err := NewSchedulerStoragePg(config)
		require.NoError(t, err)
	})

	t.Run("with invalid configuration", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(schedulerStoragePostgresUrlPath).Return("")
		_, err := NewSchedulerStoragePg(config)
		require.Error(t, err)
	})
}

func TestOperationFlowRedis(t *testing.T) {
	t.Parallel()

	t.Run("with valid redis", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(operationFlowRedisUrlPath).Return(getRedisUrl(t))
		config.EXPECT().GetInt(redisPoolSize).Return(500)
		operationFlow, err := NewOperationFlowRedis(config)
		require.NoError(t, err)

		err = operationFlow.InsertOperationID(context.Background(), "", "")
		require.NoError(t, err)
	})

	t.Run("with invalid redis", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(operationFlowRedisUrlPath).Return("redis://somewhere-in-the-world:6379")
		config.EXPECT().GetInt(redisPoolSize).Return(500)
		operationFlow, err := NewOperationFlowRedis(config)
		require.NoError(t, err)

		err = operationFlow.InsertOperationID(context.Background(), "", "")
		require.ErrorIs(t, err, errors.ErrUnexpected)
	})

	t.Run("with invalid configuration", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)

		config := configmock.NewMockConfig(mockCtrl)

		config.EXPECT().GetString(operationFlowRedisUrlPath).Return("")
		_, err := NewOperationFlowRedis(config)
		require.Error(t, err)
	})
}

func TestNewPolicyMap(t *testing.T) {
	t.Run("Should return RoomOccupancy policy", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		roomStorageMock := mock.NewMockRoomStorage(mockCtrl)

		policyMap := NewPolicyMap(roomStorageMock)
		assert.IsType(t, policyMap[autoscaling.RoomOccupancy], &roomoccupancy.Policy{})
	})
}
