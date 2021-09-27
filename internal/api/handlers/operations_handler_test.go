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

//+build integration

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	opflow "github.com/topfreegames/maestro/internal/adapters/operation_flow/mock"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func TestListOperations(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors())
		createdAtString := "2020-01-01T00:00:00.001Z"
		createdAt, _ := time.Parse(time.RFC3339Nano, createdAtString)

		operationFlow.EXPECT().ListSchedulerPendingOperationIDs(gomock.Any(), "zooba").Return([]string{}, nil)
		operationStorage.EXPECT().ListSchedulerFinishedOperations(gomock.Any(), "zooba").Return([]*operation.Operation{}, nil)
		operationStorage.EXPECT().ListSchedulerActiveOperations(gomock.Any(), "zooba").Return([]*operation.Operation{
			{
				ID:             "operation-1",
				Status:         operation.StatusInProgress,
				DefinitionName: "create_scheduler",
				SchedulerName:  "zooba",
				CreatedAt:      createdAt,
			},
		}, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)

		require.NoError(t, err)
		require.Equal(t,
			[]interface{}([]interface{}{
				map[string]interface{}{
					"definitionName": "create_scheduler",
					"id":             "operation-1",
					"schedulerName":  "zooba",
					"status":         "in_progress",
					"createdAt":      createdAtString,
				},
			}), body["activeOperations"])
	})

	t.Run("fails when operation is listed but does not exists", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors())

		operationID := "operation-1"
		schedulerName := "zooba"

		operationFlow.EXPECT().ListSchedulerPendingOperationIDs(gomock.Any(), schedulerName).Return([]string{operationID}, nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), schedulerName, operationID).Return(nil, nil, errors.NewErrNotFound("operation %s not found in scheduler %s", operationID, schedulerName))

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 500, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)

		require.NoError(t, err)
		require.Equal(t,
			"operation operation-1 not found in scheduler zooba",
			body["message"])
	})
}

func TestCancelOperation(t *testing.T) {
	schedulerName := uuid.New().String()
	operationID := uuid.New().String()

	t.Run("enqueues operation cancelation request with success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationManager := operation_manager.New(operationFlow, nil, nil)

		operationFlow.EXPECT().EnqueueOperationCancelationRequest(gomock.Any(), gomock.Eq(ports.OperationCancelationRequest{
			SchedulerName: schedulerName,
			OperationID:   operationID,
		})).Return(nil)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("/schedulers/%s/operations/%s/cancel", schedulerName, operationID), nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)
	})

	t.Run("fails to enqueues operation cancelation request when operation_flow fails", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationManager := operation_manager.New(operationFlow, nil, nil)

		operationFlow.EXPECT().EnqueueOperationCancelationRequest(gomock.Any(), gomock.Eq(ports.OperationCancelationRequest{
			SchedulerName: schedulerName,
			OperationID:   operationID,
		})).Return(errors.NewErrUnexpected("failed to persist request"))

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("/schedulers/%s/operations/%s/cancel", schedulerName, operationID), nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 500, rr.Code)
	})
}
