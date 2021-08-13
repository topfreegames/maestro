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

//+build unit

package workers_manager

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	schedulerStorageMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	configMock "github.com/topfreegames/maestro/internal/config/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/workers"
	workerMock "github.com/topfreegames/maestro/internal/core/workers/mock"
)

var (
	recorded *observer.ObservedLogs
	mockCtrl *gomock.Controller
)

func BeforeTest(t *testing.T) {
	core, observer := observer.New(zap.InfoLevel)
	zl := zap.New(core)
	zap.ReplaceGlobals(zl)
	recorded = observer

	mockCtrl = gomock.NewController(t)
	defer mockCtrl.Finish()
}

func TestStart(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		BeforeTest(t)

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		workerStopCh := make(chan struct{})
		workerBuilder := func(_ *entities.Scheduler, _ *workers.WorkerOptions) workers.Worker {
			return &workerMock.MockWorker{Run: false, StopCh: workerStopCh}
		}

		ctx, cancelFn := context.WithCancel(context.Background())
		configs.EXPECT().GetDuration(syncWorkersIntervalPath).Return(time.Second)
		configs.EXPECT().GetDuration(workersStopTimeoutDurationPath).Return(10 * time.Second)
		schedulerStorage.EXPECT().GetAllSchedulers(ctx).Return([]*entities.Scheduler{
			{
				Name:            "zooba-us",
				Game:            "zooba",
				State:           entities.StateCreating,
				RollbackVersion: "1.0.0",
				PortRange: &entities.PortRange{
					Start: 1,
					End:   10000,
				},
			},
		}, nil)

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, nil)

		done := make(chan struct{})
		go func() {
			err := workersManager.Start(ctx)
			require.NoError(t, err)
			done <- struct{}{}
		}()

		require.Eventually(t, func() bool {
			if len(workersManager.CurrentWorkers) > 0 {
				require.Contains(t, workersManager.CurrentWorkers, "zooba-us")
				return true
			}

			return false
		}, time.Second, 100*time.Millisecond)

		assertLogMessages(t, recorded, map[zapcore.Level][]string{
			zap.InfoLevel: {"new operation worker running"},
		})

		// guarantees we finish the process.
		cancelFn()
		require.Eventually(t, func() bool {
			select {
			case <-done:
				return true
			default:
			}

			return false
		}, time.Second, 100*time.Millisecond)
	})

	t.Run("fails when schedulerStorage fails to list all schedulers", func(t *testing.T) {
		BeforeTest(t)

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)

		configs.EXPECT().GetDuration(syncWorkersIntervalPath).Return(time.Second)
		configs.EXPECT().GetDuration(workersStopTimeoutDurationPath).Return(10 * time.Second)
		schedulerStorage.EXPECT().GetAllSchedulers(context.Background()).Return(nil, errors.ErrUnexpected)
		workerBuilder := func(_ *entities.Scheduler, _ *workers.WorkerOptions) workers.Worker {
			return &workerMock.MockWorker{Run: false}
		}

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, nil)

		done := make(chan struct{})
		go func() {
			err := workersManager.Start(context.Background())
			require.Error(t, err)
			done <- struct{}{}
		}()

		require.Eventually(t, func() bool {
			select {
			case <-done:
				return true
			default:
			}

			return false
		}, time.Second, 100*time.Millisecond)

		require.Empty(t, workersManager.CurrentWorkers)
	})

	t.Run("stops when context stops with no error", func(t *testing.T) {
		BeforeTest(t)

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)

		workerStopCh := make(chan struct{})
		workerBuilder := func(_ *entities.Scheduler, _ *workers.WorkerOptions) workers.Worker {
			return &workerMock.MockWorker{
				Run:    false,
				StopCh: workerStopCh,
			}
		}

		ctx, cancelFn := context.WithCancel(context.Background())
		configs.EXPECT().GetDuration(syncWorkersIntervalPath).Return(time.Second)
		configs.EXPECT().GetDuration(workersStopTimeoutDurationPath).Return(10 * time.Second)
		schedulerStorage.EXPECT().GetAllSchedulers(ctx).AnyTimes().Return([]*entities.Scheduler{
			{
				Name:            "zooba-us",
				Game:            "zooba",
				State:           entities.StateCreating,
				RollbackVersion: "1.0.0",
				PortRange: &entities.PortRange{
					Start: 1,
					End:   10000,
				},
			},
		}, nil)

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, nil)

		done := make(chan struct{})
		go func() {
			err := workersManager.Start(ctx)
			require.NoError(t, err)
			done <- struct{}{}
		}()

		// guarantees the
		require.Eventually(t, func() bool {
			return len(workersManager.CurrentWorkers) > 0
		}, time.Second, 100*time.Millisecond)

		// guarantees we finish the process.
		cancelFn()
		require.Eventually(t, func() bool {
			select {
			case <-done:
				return true
			default:
			}

			return false
		}, time.Second, 100*time.Millisecond)

		require.Empty(t, workersManager.CurrentWorkers)

		// Checks if the workersWaitGroup is empty (means all workers are done)
		require.Eventually(t, func() bool {
			workersManager.workersWaitGroup.Wait()
			return true
		}, 10*time.Millisecond, time.Millisecond)
	})

	t.Run("with success when scheduler added after initial sync", func(t *testing.T) {
		BeforeTest(t)

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)

		workerStopCh := make(chan struct{})
		workerBuilder := func(_ *entities.Scheduler, _ *workers.WorkerOptions) workers.Worker {
			return &workerMock.MockWorker{Run: false, StopCh: workerStopCh}
		}

		ctx, cancelFn := context.WithCancel(context.Background())
		configs.EXPECT().GetDuration(syncWorkersIntervalPath).Return(time.Second)
		configs.EXPECT().GetDuration(workersStopTimeoutDurationPath).Return(10 * time.Second)

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, nil)
		require.Empty(t, workersManager.CurrentWorkers)

		// first call channel handler
		firstCycle := make(chan struct{})
		schedulerStorage.EXPECT().GetAllSchedulers(ctx).DoAndReturn(func(_ context.Context) ([]*entities.Scheduler, error) {
			firstCycle <- struct{}{}
			return []*entities.Scheduler{}, nil
		})

		done := make(chan struct{})
		go func() {
			err := workersManager.Start(ctx)
			require.NoError(t, err)
			done <- struct{}{}
		}()

		// waits until the first sync happens.
		require.Eventually(t, func() bool {
			select {
			case <-firstCycle:
				return true
			default:
			}

			return false
		}, time.Second, 100*time.Millisecond)

		schedulerStorage.EXPECT().GetAllSchedulers(ctx).Return([]*entities.Scheduler{
			{
				Name:            "zooba-us",
				Game:            "zooba",
				State:           entities.StateCreating,
				RollbackVersion: "1.0.0",
				PortRange: &entities.PortRange{
					Start: 1,
					End:   10000,
				},
			},
		}, nil)

		require.Eventually(t, func() bool {
			if len(workersManager.CurrentWorkers) > 0 {
				require.Contains(t, workersManager.CurrentWorkers, "zooba-us")
				return true
			}

			return false
		}, 5*time.Second, 100*time.Millisecond)

		assertLogMessages(t, recorded, map[zapcore.Level][]string{
			zap.InfoLevel: {"new operation worker running"},
		})

		// guarantees we finish the process.
		cancelFn()
		require.Eventually(t, func() bool {
			select {
			case <-done:
				return true
			default:
			}

			return false
		}, time.Second, 100*time.Millisecond)
	})

	t.Run("with success when scheduler removed after bootstrap", func(t *testing.T) {
		BeforeTest(t)

		configs := configMock.NewMockConfig(mockCtrl)
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)

		workerStopCh := make(chan struct{})
		workerBuilder := func(_ *entities.Scheduler, _ *workers.WorkerOptions) workers.Worker {
			return &workerMock.MockWorker{Run: false, StopCh: workerStopCh}
		}

		ctx, cancelFn := context.WithCancel(context.Background())
		configs.EXPECT().GetDuration(syncWorkersIntervalPath).Return(time.Second)
		configs.EXPECT().GetDuration(workersStopTimeoutDurationPath).Return(10 * time.Second)
		schedulerStorage.EXPECT().GetAllSchedulers(ctx).Times(3).Return([]*entities.Scheduler{
			{
				Name:            "zooba-us",
				Game:            "zooba",
				State:           entities.StateCreating,
				RollbackVersion: "1.0.0",
				PortRange: &entities.PortRange{
					Start: 1,
					End:   10000,
				},
			},
		}, nil)

		workersManager := NewWorkersManager(workerBuilder, configs, schedulerStorage, nil)

		done := make(chan struct{})
		go func() {
			err := workersManager.Start(ctx)
			require.NoError(t, err)
			done <- struct{}{}
		}()

		// wait until the workers are started.
		require.Eventually(t, func() bool {
			return len(workersManager.CurrentWorkers) > 0
		}, time.Second, 100*time.Millisecond)

		require.Contains(t, workersManager.CurrentWorkers, "zooba-us")

		schedulerStorage.EXPECT().GetAllSchedulers(ctx).Return([]*entities.Scheduler{}, nil)

		// wait until the workers are stopped.
		require.Eventually(t, func() bool {
			return len(workersManager.CurrentWorkers) == 0
		}, 5*time.Second, 100*time.Millisecond)

		require.Empty(t, workersManager.CurrentWorkers)

		// guarantees we finish the process.
		cancelFn()
		require.Eventually(t, func() bool {
			select {
			case <-done:
				return true
			default:
			}

			return false
		}, time.Second, 100*time.Millisecond)
	})
}

func assertLogMessages(t *testing.T, recorded *observer.ObservedLogs, messages map[zapcore.Level][]string) {
	for level, values := range messages {

		levelRecords := recorded.FilterLevelExact(level)
		for _, message := range values {
			require.NotEmpty(t, levelRecords.Filter(func(le observer.LoggedEntry) bool {
				return le.Message == message
			}))
		}
	}
}
