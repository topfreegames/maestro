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

package add

import (
	"context"
	"fmt"
	"testing"
	"time"

	clock_mock "github.com/topfreegames/maestro/internal/core/ports/clock_mock.go"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
)

func TestExecutor_Execute(t *testing.T) {
	clockMock := clock_mock.NewFakeClock(time.Now())

	definition := Definition{Amount: 10}

	container1 := game_room.Container{
		Name: "container1",
		Ports: []game_room.ContainerPort{
			{Protocol: "tcp"},
		},
	}

	container2 := game_room.Container{
		Name: "container2",
		Ports: []game_room.ContainerPort{
			{Protocol: "udp"},
		},
	}

	scheduler := entities.Scheduler{
		Name: "zooba_blue:1.0.0",
		Spec: game_room.Spec{
			Version:    "1.0.0",
			Containers: []game_room.Container{container1, container2},
		},
		PortRange: nil,
	}

	gameRoom := game_room.GameRoom{
		ID:          "game-1",
		SchedulerID: "zooba_blue:1.0.0",
		Version:     "1.0.0",
		Status:      game_room.GameStatusPending,
		LastPingAt:  clockMock.Now(),
	}

	gameRoomInstance := game_room.Instance{
		ID:          "game-1",
		SchedulerID: "game",
	}

	config := Config{
		AmountLimit: 10,
	}

	op := operation.Operation{
		ID:             "some-op-id",
		SchedulerName:  "zooba_blue:1.0.0",
		Status:         operation.StatusPending,
		DefinitionName: "zooba_blue:1.0.0",
	}

	t.Run("should succeed - all room creations succeed => return nil, no error", func(t *testing.T) {
		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, config)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(&scheduler, nil)
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(10)
		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, fmt.Sprintf("added %d rooms", definition.Amount))

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, config)
		err := executor.Execute(context.Background(), &op, &definition)

		require.Nil(t, err)
	})

	t.Run("should succeed - some room creation fail, others succeed => returns success", func(t *testing.T) {
		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, config)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), op.SchedulerName).Return(&scheduler, nil)

		gameRoomReady := gameRoom
		gameRoomReady.Status = game_room.GameStatusReady

		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(9)

		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(nil, nil, porterrors.NewErrUnexpected("error"))

		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, fmt.Sprintf("added %d rooms", definition.Amount))

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, config)
		err := executor.Execute(context.Background(), &op, &definition)

		require.Nil(t, err)
	})

	t.Run("should fail - no scheduler found => returns error", func(t *testing.T) {
		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, config)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(nil, porterrors.NewErrNotFound("scheduler not found"))
		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, "error fetching scheduler from storage: scheduler not found")

		err := NewExecutor(roomsManager, schedulerStorage, operationsManager, config).Execute(context.Background(), &op, &definition)
		require.NotNil(t, err)
		require.ErrorContains(t, err, "error fetching scheduler from storage: scheduler not found")
	})

	t.Run("use default amount limit if AmountLimit not set", func(t *testing.T) {
		executor, _, _, _ := testSetup(t, Config{})
		require.NotNil(t, executor)
		require.Equal(t, executor.config.AmountLimit, int32(DefaultAmountLimit))
	})

	t.Run("use default amount limit if invalid AmountLimit value", func(t *testing.T) {
		executor, _, _, _ := testSetup(t, Config{AmountLimit: -1})
		require.NotNil(t, executor)
		require.Equal(t, executor.config.AmountLimit, int32(DefaultAmountLimit))
	})

	t.Run("should succeed - cap to the default amount if asked to create more than whats configured", func(t *testing.T) {
		bigAmountDefinition := Definition{Amount: 10000}
		smallDefaultConfig := Config{AmountLimit: 10}

		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, smallDefaultConfig)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(&scheduler, nil)
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(int(smallDefaultConfig.AmountLimit))
		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, fmt.Sprintf("added %d rooms", smallDefaultConfig.AmountLimit))

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, smallDefaultConfig)
		err := executor.Execute(context.Background(), &op, &bigAmountDefinition)

		require.Nil(t, err)
	})

	t.Run("should fail - majority of room creation fail, others succeed => returns error", func(t *testing.T) {
		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, config)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), op.SchedulerName).Return(&scheduler, nil)

		gameRoomReady := gameRoom
		gameRoomReady.Status = game_room.GameStatusReady

		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(4)
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(nil, nil, porterrors.NewErrUnexpected("error")).Times(6)

		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, "more rooms failed than succeeded, errors: 6 and successes: 4 of amount: 10")

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, config)
		err := executor.Execute(context.Background(), &op, &definition)

		require.NotNil(t, err)
		require.ErrorContains(t, err, "more rooms failed than succeeded, errors: 6 and successes: 4 of amount: 10")
	})

	t.Run("use default batch size if BatchSize not set", func(t *testing.T) {
		executor, _, _, _ := testSetup(t, Config{})
		require.NotNil(t, executor)
		require.Equal(t, executor.config.BatchSize, int32(DefaultBatchSize))
	})

	t.Run("use default batch size if invalid BatchSize value", func(t *testing.T) {
		executor, _, _, _ := testSetup(t, Config{BatchSize: -1})
		require.NotNil(t, executor)
		require.Equal(t, executor.config.BatchSize, int32(DefaultBatchSize))
	})

	t.Run("should succeed - process rooms in batches when amount exceeds batch size", func(t *testing.T) {
		batchConfig := Config{
			AmountLimit: 100,
			BatchSize:   3, // Small batch size for testing
		}
		largeDefinition := Definition{Amount: 7} // Will create 3 batches: 3, 3, 1

		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, batchConfig)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(&scheduler, nil)
		// Expect 7 room creations total (3 + 3 + 1)
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(7)
		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, fmt.Sprintf("added %d rooms", largeDefinition.Amount))

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, batchConfig)
		err := executor.Execute(context.Background(), &op, &largeDefinition)

		require.Nil(t, err)
	})

	t.Run("should succeed - process rooms in batches with errors in some batches", func(t *testing.T) {
		batchConfig := Config{
			AmountLimit: 100,
			BatchSize:   2, // Small batch size for testing
		}
		definition := Definition{Amount: 6} // Will create 3 batches: 2, 2, 2

		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, batchConfig)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(&scheduler, nil)
		// First batch: 2 successes
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(2)
		// Second batch: 1 success, 1 failure
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(1)
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(nil, nil, porterrors.NewErrUnexpected("error")).Times(1)
		// Third batch: 2 successes
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(2)

		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, fmt.Sprintf("added %d rooms", definition.Amount))

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, batchConfig)
		err := executor.Execute(context.Background(), &op, &definition)

		require.Nil(t, err) // Should succeed because only 1 out of 6 failed
	})

	t.Run("should fail - majority failures across batches", func(t *testing.T) {
		batchConfig := Config{
			AmountLimit: 100,
			BatchSize:   2, // Small batch size for testing
		}
		definition := Definition{Amount: 6} // Will create 3 batches: 2, 2, 2

		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, batchConfig)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(&scheduler, nil)
		// First batch: 2 failures
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(nil, nil, porterrors.NewErrUnexpected("error")).Times(2)
		// Second batch: 2 failures
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(nil, nil, porterrors.NewErrUnexpected("error")).Times(2)
		// Third batch: 1 success, 1 failure
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(1)
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(nil, nil, porterrors.NewErrUnexpected("error")).Times(1)

		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, "more rooms failed than succeeded, errors: 5 and successes: 1 of amount: 6")

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, batchConfig)
		err := executor.Execute(context.Background(), &op, &definition)

		require.NotNil(t, err)
		require.ErrorContains(t, err, "more rooms failed than succeeded, errors: 5 and successes: 1 of amount: 6")
	})

	t.Run("should succeed - amount exactly equals batch size", func(t *testing.T) {
		batchConfig := Config{
			AmountLimit: 100,
			BatchSize:   5,
		}
		exactDefinition := Definition{Amount: 5} // Exactly equals batch size

		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, batchConfig)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(&scheduler, nil)
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(5)
		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, fmt.Sprintf("added %d rooms", exactDefinition.Amount))

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, batchConfig)
		err := executor.Execute(context.Background(), &op, &exactDefinition)

		require.Nil(t, err)
	})

	t.Run("should succeed - large amount with default batch size", func(t *testing.T) {
		largeDefinition := Definition{Amount: 100} // Large amount to test default batch processing
		largeConfig := Config{
			AmountLimit: 200, // High enough to allow 100 rooms
			BatchSize:   25,  // Default batch size
		}

		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, largeConfig)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(&scheduler, nil)
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).Return(&gameRoom, &gameRoomInstance, nil).Times(100)
		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, fmt.Sprintf("added %d rooms", largeDefinition.Amount))

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, largeConfig)
		err := executor.Execute(context.Background(), &op, &largeDefinition)

		require.Nil(t, err)
	})

	t.Run("use default operation timeout if OperationTimeout not set", func(t *testing.T) {
		executor, _, _, _ := testSetup(t, Config{})
		require.NotNil(t, executor)
		require.Equal(t, executor.config.OperationTimeout, DefaultOperationTimeout)
	})

	t.Run("use default operation timeout if invalid OperationTimeout value", func(t *testing.T) {
		executor, _, _, _ := testSetup(t, Config{OperationTimeout: -1 * time.Second})
		require.NotNil(t, executor)
		require.Equal(t, executor.config.OperationTimeout, DefaultOperationTimeout)
	})

	t.Run("use custom operation timeout when valid value provided", func(t *testing.T) {
		customTimeout := 10 * time.Second
		executor, _, _, _ := testSetup(t, Config{OperationTimeout: customTimeout})
		require.NotNil(t, executor)
		require.Equal(t, executor.config.OperationTimeout, customTimeout)
	})

	t.Run("should timeout when room creation takes longer than configured timeout", func(t *testing.T) {
		shortTimeoutDefinition := Definition{Amount: 1}
		shortTimeoutConfig := Config{
			AmountLimit:      10,
			BatchSize:        10,
			OperationTimeout: 10 * time.Millisecond, // Very short timeout
		}

		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, shortTimeoutConfig)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(&scheduler, nil)

		// Mock room creation to take longer than the timeout
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).DoAndReturn(
			func(ctx context.Context, scheduler entities.Scheduler, checkRoomDeletion bool) (*game_room.GameRoom, *game_room.Instance, error) {
				select {
				// Sleep longer than the timeout to trigger context deadline
				case <-time.After(100 * time.Millisecond):
					return &gameRoom, &gameRoomInstance, nil
				// Should return context deadline exceeded
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				}
			},
		).Times(1)

		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, "more rooms failed than succeeded, errors: 1 and successes: 0 of amount: 1")

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, shortTimeoutConfig)
		err := executor.Execute(context.Background(), &op, &shortTimeoutDefinition)

		require.NotNil(t, err)
		require.ErrorContains(t, err, "more rooms failed than succeeded")
	})

	t.Run("should succeed when room creation completes within configured timeout", func(t *testing.T) {
		fastDefinition := Definition{Amount: 2}
		reasonableTimeoutConfig := Config{
			AmountLimit:      10,
			BatchSize:        10,
			OperationTimeout: 1 * time.Second, // Reasonable timeout
		}

		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, reasonableTimeoutConfig)

		schedulerStorage.EXPECT().GetScheduler(context.Background(), op.SchedulerName).Return(&scheduler, nil)

		// Mock room creation to complete quickly within the timeout
		roomsManager.EXPECT().CreateRoom(gomock.Any(), gomock.Any(), false).DoAndReturn(
			func(ctx context.Context, scheduler entities.Scheduler, checkRoomDeletion bool) (*game_room.GameRoom, *game_room.Instance, error) {
				// Complete quickly within timeout
				time.Sleep(50 * time.Millisecond) // Much less than 1 second
				return &gameRoom, &gameRoomInstance, nil
			},
		).Times(2)

		operationsManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), &op, fmt.Sprintf("added %d rooms", fastDefinition.Amount))

		executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, reasonableTimeoutConfig)
		err := executor.Execute(context.Background(), &op, &fastDefinition)

		require.Nil(t, err)
	})
}

func TestExecutor_Rollback(t *testing.T) {
	definition := Definition{Amount: 10}

	op := operation.Operation{
		ID:             "some-op-id",
		SchedulerName:  "zooba_blue:1.0.0",
		Status:         operation.StatusPending,
		DefinitionName: "zooba_blue:1.0.0",
	}

	config := Config{
		AmountLimit: 10,
	}

	t.Run("does nothing and return nil", func(t *testing.T) {
		_, roomsManager, schedulerStorage, operationsManager := testSetup(t, config)

		rollbackErr := NewExecutor(roomsManager, schedulerStorage, operationsManager, config).Rollback(context.Background(), &op, &definition, nil)
		require.NoError(t, rollbackErr)
	})
}

func testSetup(t *testing.T, cfg Config) (*Executor, *mockports.MockRoomManager, *mockports.MockSchedulerStorage, *mockports.MockOperationManager) {
	mockCtrl := gomock.NewController(t)

	roomsManager := mockports.NewMockRoomManager(mockCtrl)
	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
	operationsManager := mockports.NewMockOperationManager(mockCtrl)
	executor := NewExecutor(roomsManager, schedulerStorage, operationsManager, cfg)
	return executor, roomsManager, schedulerStorage, operationsManager
}
