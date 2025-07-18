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
	"container/ring"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/worker/config"

	"github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/core/worker"
)

func TestMetricsReporterWorker_StartProduceMetrics(t *testing.T) {
	t.Run("produce metrics for pods and game rooms with success when no error occurs", func(t *testing.T) {
		resetMetricsCollectors()

		mockCtl := gomock.NewController(t)
		roomStorage := mock.NewMockRoomStorage(mockCtl)
		instanceStorage := mock.NewMockGameRoomInstanceStorage(mockCtl)
		schedulerCache := mock.NewMockSchedulerCache(mockCtl)
		ctx, cancelFunc := context.WithCancel(context.Background())
		scheduler := &entities.Scheduler{Name: "random-scheduler"}
		instances := newInstancesList(40)

		workerOpts := &worker.WorkerOptions{
			RoomStorage:           roomStorage,
			InstanceStorage:       instanceStorage,
			MetricsReporterConfig: &config.MetricsReporterConfig{MetricsReporterIntervalMillis: 500},
			SchedulerCache:        schedulerCache,
		}

		worker := NewMetricsReporterWorker(scheduler, workerOpts)

		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusReady).
			Return(11, nil).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusPending).
			Return(22, nil).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusTerminating).
			Return(33, nil).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusOccupied).
			Return(44, nil).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusUnready).
			Return(55, nil).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusError).
			Return(66, nil).MinTimes(3)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusActive).
			Return(77, nil).MinTimes(3)
		roomStorage.EXPECT().GetRunningMatchesCount(gomock.Any(), scheduler.Name).
			Return(88, nil).MinTimes(3)
		instanceStorage.EXPECT().GetAllInstances(gomock.Any(), scheduler.Name).Return(instances, nil).MinTimes(3)
		schedulerCache.EXPECT().GetScheduler(gomock.Any(), scheduler.Name).Return(scheduler, nil).MinTimes(3)

		go func() {
			err := worker.Start(ctx)
			assert.NoError(t, err)
		}()

		time.Sleep(time.Second * 2)
		assert.True(t, worker.IsRunning())
		cancelFunc()
		// Wait for the worker to properly stop
		time.Sleep(time.Millisecond * 100)
		assert.False(t, worker.IsRunning())

		// assert metrics were collected
		assert.Equal(t, float64(11), testutil.ToFloat64(gameRoomReadyGaugeMetric))
		assert.Equal(t, float64(22), testutil.ToFloat64(gameRoomPendingGaugeMetric))
		assert.Equal(t, float64(33), testutil.ToFloat64(gameRoomTerminatingGaugeMetric))
		assert.Equal(t, float64(44), testutil.ToFloat64(gameRoomOccupiedGaugeMetric))
		assert.Equal(t, float64(55), testutil.ToFloat64(gameRoomUnreadyGaugeMetric))
		assert.Equal(t, float64(66), testutil.ToFloat64(gameRoomErrorGaugeMetric))
		assert.Equal(t, float64(77), testutil.ToFloat64(gameRoomActiveGaugeMetric))

		assert.Equal(t, float64(8), testutil.ToFloat64(instanceReadyGaugeMetric))
		assert.Equal(t, float64(8), testutil.ToFloat64(instancePendingGaugeMetric))
		assert.Equal(t, float64(8), testutil.ToFloat64(instanceUnknownGaugeMetric))
		assert.Equal(t, float64(8), testutil.ToFloat64(instanceTerminatingGaugeMetric))
		assert.Equal(t, float64(8), testutil.ToFloat64(instanceErrorGaugeMetric))

		assert.Equal(t, float64(88), testutil.ToFloat64(runningMatchesGaugeMetric))
	})
}

func TestMetricsReporterWorker_StartDoNotProduceMetrics(t *testing.T) {
	t.Run("don't produce metrics, log errors but doesn't stop worker when some error occurs", func(t *testing.T) {
		// Reset metrics before test to ensure clean state
		resetMetricsCollectors()

		mockCtl := gomock.NewController(t)
		defer mockCtl.Finish()

		roomStorage := mock.NewMockRoomStorage(mockCtl)
		instanceStorage := mock.NewMockGameRoomInstanceStorage(mockCtl)
		schedulerCache := mock.NewMockSchedulerCache(mockCtl)
		ctx, cancelFunc := context.WithCancel(context.Background())

		scheduler := &entities.Scheduler{Name: "random-scheduler-2"}

		workerOpts := &worker.WorkerOptions{
			RoomStorage:           roomStorage,
			InstanceStorage:       instanceStorage,
			MetricsReporterConfig: &config.MetricsReporterConfig{MetricsReporterIntervalMillis: 500},
			SchedulerCache:        schedulerCache,
		}
		worker := NewMetricsReporterWorker(scheduler, workerOpts)

		// Set up all mocks to return errors
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
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), scheduler.Name, game_room.GameStatusActive).
			Return(0, errors.New("some_error")).MinTimes(3)
		roomStorage.EXPECT().GetRunningMatchesCount(gomock.Any(), scheduler.Name).
			Return(0, errors.New("some_error")).MinTimes(3)
		instanceStorage.EXPECT().GetAllInstances(gomock.Any(), scheduler.Name).
			Return([]*game_room.Instance{}, errors.New("some_error")).MinTimes(3)
		schedulerCache.EXPECT().GetScheduler(gomock.Any(), scheduler.Name).
			Return(scheduler, errors.New("some_error")).MinTimes(3)

		go func() {
			err := worker.Start(ctx)
			assert.NoError(t, err)
		}()

		// Wait for the worker to start and run at least 3 cycles (500ms * 3 = 1500ms)
		time.Sleep(time.Millisecond * 1600) // Wait for 3 ticks (1500ms) plus some buffer
		assert.True(t, worker.IsRunning())

		cancelFunc()
		// Wait for the worker to properly stop
		time.Sleep(time.Millisecond * 100)
		assert.False(t, worker.IsRunning())

		// assert metrics were set to 0 for room and instance metrics (due to errors)
		// but scheduler metrics may still be collected since they don't depend on storage
		assert.Equal(t, float64(0), testutil.ToFloat64(gameRoomReadyGaugeMetric))
		assert.Equal(t, float64(0), testutil.ToFloat64(gameRoomPendingGaugeMetric))
		assert.Equal(t, float64(0), testutil.ToFloat64(gameRoomTerminatingGaugeMetric))
		assert.Equal(t, float64(0), testutil.ToFloat64(gameRoomOccupiedGaugeMetric))
		assert.Equal(t, float64(0), testutil.ToFloat64(gameRoomUnreadyGaugeMetric))
		assert.Equal(t, float64(0), testutil.ToFloat64(gameRoomErrorGaugeMetric))

		assert.Equal(t, float64(0), testutil.ToFloat64(instanceReadyGaugeMetric))
		assert.Equal(t, float64(0), testutil.ToFloat64(instancePendingGaugeMetric))
		assert.Equal(t, float64(0), testutil.ToFloat64(instanceUnknownGaugeMetric))
		assert.Equal(t, float64(0), testutil.ToFloat64(instanceTerminatingGaugeMetric))
		assert.Equal(t, float64(0), testutil.ToFloat64(instanceErrorGaugeMetric))
	})
}

func newInstancesList(numberOfInstances int) (instances []*game_room.Instance) {
	possibleInstanceStatus := []game_room.InstanceStatusType{
		game_room.InstanceReady,
		game_room.InstanceUnknown,
		game_room.InstanceError,
		game_room.InstanceTerminating,
		game_room.InstancePending,
	}
	statusRing := ring.New(len(possibleInstanceStatus))
	for i := 0; i < statusRing.Len(); i++ {
		statusRing.Value = possibleInstanceStatus[i]
		statusRing = statusRing.Next()
	}

	for i := 0; i < numberOfInstances; i++ {
		status, _ := statusRing.Move(i).Value.(game_room.InstanceStatusType)
		instances = append(instances, &game_room.Instance{Status: game_room.InstanceStatus{Type: status}})
	}
	return instances
}
func resetMetricsCollectors() {
	// Reset all game room metrics
	gameRoomReadyGaugeMetric.Reset()
	gameRoomPendingGaugeMetric.Reset()
	gameRoomTerminatingGaugeMetric.Reset()
	gameRoomOccupiedGaugeMetric.Reset()
	gameRoomUnreadyGaugeMetric.Reset()
	gameRoomErrorGaugeMetric.Reset()
	gameRoomActiveGaugeMetric.Reset()

	// Reset all instance metrics
	instanceReadyGaugeMetric.Reset()
	instancePendingGaugeMetric.Reset()
	instanceUnknownGaugeMetric.Reset()
	instanceTerminatingGaugeMetric.Reset()
	instanceErrorGaugeMetric.Reset()

	// Reset other metrics that might be set
	runningMatchesGaugeMetric.Reset()
	schedulerMaxMatchesGaugeMetric.Reset()
	schedulerAutoscalePolicyReadyTargetGaugeMetric.Reset()
	schedulerAutoscalePolicyFixedBufferGaugeMetric.Reset()
}
