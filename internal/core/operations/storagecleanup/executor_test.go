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

package storagecleanup_test

import (
	"errors"

	"github.com/topfreegames/maestro/internal/core/operations/storagecleanup"
	"github.com/topfreegames/maestro/internal/core/ports"

	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
)

func TestExecutor_Execute(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	operation := &operation.Operation{
		SchedulerName: "scheduler-name",
	}
	definition := &storagecleanup.Definition{}

	t.Run("should succeed", func(t *testing.T) {
		t.Run("operation storage CleanOperationHistory return no error => return no error", func(t *testing.T) {
			operationStorage := mockports.NewMockOperationStorage(mockCtrl)

			operationStorage.EXPECT().CleanOperationsHistory(context.Background(), operation.SchedulerName).Return(nil)

			executor := storagecleanup.NewExecutor(operationStorage)
			err := executor.Execute(context.Background(), operation, definition)

			require.NoError(t, err)
		})
	})

	t.Run("should fail", func(t *testing.T) {
		t.Run("operation storage CleaOperationHistory fails return unexpected error", func(t *testing.T) {
			operationStorage := mockports.NewMockOperationStorage(mockCtrl)

			operationStorage.EXPECT().CleanOperationsHistory(context.Background(), operation.SchedulerName).Return(errors.New("error"))

			executor := storagecleanup.NewExecutor(operationStorage)
			err := executor.Execute(context.Background(), operation, definition)
			require.ErrorContains(t, err, "failed to clean operation history on storage clean up operation: ")
		})
	})
}

func TestExecutor_Rollback(t *testing.T) {
	operation := &operation.Operation{}
	definition := &storagecleanup.Definition{}

	t.Run("does nothing and return nil", func(t *testing.T) {
		executor := storagecleanup.NewExecutor((ports.OperationStorage)(nil))

		err := executor.Rollback(context.Background(), operation, definition, nil)
		require.Nil(t, err)
	})

}
