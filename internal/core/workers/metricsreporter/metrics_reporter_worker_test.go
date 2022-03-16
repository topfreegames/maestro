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

//go:build unit
// +build unit

package metricsreporter

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	mockinstance "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/core/workers"
)

func TestMetricsReporterWorker_Start(t *testing.T) {
	t.Run("produce metrics for pods and game rooms with success when no error occurs", func(t *testing.T) {
		mockCtl := gomock.NewController(t)
		roomStorage := mock.NewMockRoomStorage(mockCtl)
		instanceStorage := mockinstance.NewMockGameRoomInstanceStorage(mockCtl)
		ctx, cancelFunc := context.WithCancel(context.Background())

		scheduler := &entities.Scheduler{Name: "random-scheduler"}
		instances := []*game_room.Instance{
			{
				Status: game_room.InstanceStatus{Type: 0},
			},
			{
				Status: game_room.InstanceStatus{Type: 1},
			},
			{
				Status: game_room.InstanceStatus{Type: 2},
			},
			{
				Status: game_room.InstanceStatus{Type: 3},
			},
			{
				Status: game_room.InstanceStatus{Type: 4},
			},
		}

		workerOpts := &workers.WorkerOptions{
			RoomStorage:           roomStorage,
			InstanceStorage:       instanceStorage,
			MetricsReporterPeriod: 500}

		worker := NewMetricsReporterWorker(scheduler, workerOpts)

		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusReady).
			Return(1, nil).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusPending).
			Return(1, nil).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusTerminating).
			Return(1, nil).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).
			Return(1, nil).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusUnready).
			Return(1, nil).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusError).
			Return(1, nil).MinTimes(3)

		instanceStorage.EXPECT().GetAllInstances(gomock.Any(), scheduler.Name).Return(instances, nil).MinTimes(3)

		go func() {
			err := worker.Start(ctx)
			require.NoError(t, err)
		}()

		time.Sleep(time.Second * 2)
		assert.True(t, worker.IsRunning())
		cancelFunc()
		assert.False(t, worker.IsRunning())
	})

	t.Run("log errors but doesn't stop worker when some error occurs", func(t *testing.T) {
		mockCtl := gomock.NewController(t)
		roomStorage := mock.NewMockRoomStorage(mockCtl)
		instanceStorage := mockinstance.NewMockGameRoomInstanceStorage(mockCtl)
		ctx, cancelFunc := context.WithCancel(context.Background())

		scheduler := &entities.Scheduler{Name: "random-scheduler"}

		workerOpts := &workers.WorkerOptions{
			RoomStorage:           roomStorage,
			InstanceStorage:       instanceStorage,
			MetricsReporterPeriod: 500}

		worker := NewMetricsReporterWorker(scheduler, workerOpts)

		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusReady).
			Return(0, errors.New("some_error")).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusPending).
			Return(0, errors.New("some_error")).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusTerminating).
			Return(0, errors.New("some_error")).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).
			Return(0, errors.New("some_error")).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusUnready).
			Return(0, errors.New("some_error")).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusError).
			Return(0, errors.New("some_error")).MinTimes(3)

		instanceStorage.EXPECT().GetAllInstances(gomock.Any(), scheduler.Name).
			Return([]*game_room.Instance{}, errors.New("some_error")).MinTimes(3)

		go func() {
			err := worker.Start(ctx)
			require.NoError(t, err)
		}()
		time.Sleep(time.Second * 2)
		assert.True(t, worker.IsRunning())
		cancelFunc()
		assert.False(t, worker.IsRunning())
	})

}
