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

package create_scheduler

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/ports/errors"

	runtimeMock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
)

func TestExecute(t *testing.T) {

	t.Run("with success", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)

		runtime := runtimeMock.NewMockRuntime(mockCtrl)
		schedulerManager := mockports.NewMockSchedulerManager(mockCtrl)

		definition := CreateSchedulerDefinition{}
		op := operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "zooba_blue:1.0.0",
			Status:         operation.StatusPending,
			DefinitionName: "zooba_blue:1.0.0",
		}

		runtime.EXPECT().CreateScheduler(context.Background(), &entities.Scheduler{Name: op.SchedulerName}).Return(nil)

		err := NewExecutor(runtime, schedulerManager).Execute(context.Background(), &op, &definition)
		require.Nil(t, err)
	})

	t.Run("fails with runtime request fails", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)

		runtime := runtimeMock.NewMockRuntime(mockCtrl)
		schedulerManager := mockports.NewMockSchedulerManager(mockCtrl)

		definition := CreateSchedulerDefinition{}
		op := operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "zooba_blue:1.0.0",
			Status:         operation.StatusPending,
			DefinitionName: "zooba_blue:1.0.0",
		}

		runtime.EXPECT().CreateScheduler(context.Background(), &entities.Scheduler{Name: op.SchedulerName}).Return(errors.ErrUnexpected)

		err := NewExecutor(runtime, schedulerManager).Execute(context.Background(), &op, &definition)
		require.NotNil(t, err)
	})
}

func TestRollback(t *testing.T) {
	t.Run("it returns nil when delete scheduler on execution error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		runtime := runtimeMock.NewMockRuntime(mockCtrl)
		schedulerManager := mockports.NewMockSchedulerManager(mockCtrl)
		definition := &CreateSchedulerDefinition{}
		op := operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "zooba_blue:1.0.0",
			Status:         operation.StatusPending,
			DefinitionName: "zooba_blue:1.0.0",
		}
		schedulerManager.EXPECT().DeleteScheduler(gomock.Any(), op.SchedulerName).Return(nil)

		err := NewExecutor(runtime, schedulerManager).Rollback(context.Background(), &op, definition, errors.ErrUnexpected)

		assert.NoError(t, err)
	})

	t.Run("it returns error when couldn't delete scheduler on execution error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		runtime := runtimeMock.NewMockRuntime(mockCtrl)
		schedulerManager := mockports.NewMockSchedulerManager(mockCtrl)
		definition := &CreateSchedulerDefinition{}
		op := operation.Operation{
			ID:             "some-op-id",
			SchedulerName:  "zooba_blue:1.0.0",
			Status:         operation.StatusPending,
			DefinitionName: "zooba_blue:1.0.0",
		}
		schedulerManager.EXPECT().DeleteScheduler(gomock.Any(), op.SchedulerName).Return(errors.NewErrUnexpected("err"))

		err := NewExecutor(runtime, schedulerManager).Rollback(context.Background(), &op, definition, errors.ErrUnexpected)

		assert.Error(t, err)
		assert.ErrorIs(t, err, errors.ErrUnexpected)
		assert.Contains(t, err.Error(), "error in Rollback function execution")
	})
}
