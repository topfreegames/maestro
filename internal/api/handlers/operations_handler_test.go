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
		schedulerName := "zooba"
		dates := []time.Time{
			time.Time{}.AddDate(2020, 0, 0),
			time.Time{}.AddDate(2020, 1, 0),
			time.Time{}.AddDate(2020, 2, 0),
		}
		pendingOperations := []*operation.Operation{
			&operation.Operation{ID: "d28f3fc7-ca32-4ca8-8b6a-8fbb19003389", Status: operation.StatusPending, CreatedAt: dates[0], SchedulerName: schedulerName, DefinitionName: "create_scheduler"},
			&operation.Operation{ID: "7af3250c-af5b-428a-955f-a8fa22fb7cf7", Status: operation.StatusPending, CreatedAt: dates[1], SchedulerName: schedulerName, DefinitionName: "create_scheduler"},
			&operation.Operation{ID: "83cc7850-9c90-4033-948f-368eea4b976e", Status: operation.StatusPending, CreatedAt: dates[2], SchedulerName: schedulerName, DefinitionName: "create_scheduler"},
		}
		finishedOperations := []*operation.Operation{
			&operation.Operation{ID: "c241b467-db15-42ba-b2a8-017c37234237", Status: operation.StatusFinished, CreatedAt: dates[0], SchedulerName: schedulerName, DefinitionName: "create_scheduler"},
			&operation.Operation{ID: "f1fce7b2-3374-464e-9eb4-08b25fa0da54", Status: operation.StatusFinished, CreatedAt: dates[1], SchedulerName: schedulerName, DefinitionName: "create_scheduler"},
			&operation.Operation{ID: "ae218cc1-2dd8-448b-a78f-0cc979f89f37", Status: operation.StatusFinished, CreatedAt: dates[2], SchedulerName: schedulerName, DefinitionName: "create_scheduler"},
		}
		activeOperations := []*operation.Operation{
			&operation.Operation{ID: "72e108f8-8025-4e96-9f3f-b81ac5b40d50", Status: operation.StatusInProgress, CreatedAt: dates[0], SchedulerName: schedulerName, DefinitionName: "create_scheduler"},
			&operation.Operation{ID: "59e58c61-1758-4f02-b6ea-a87a64172902", Status: operation.StatusInProgress, CreatedAt: dates[1], SchedulerName: schedulerName, DefinitionName: "create_scheduler"},
			&operation.Operation{ID: "2d88b86b-0e70-451c-93cf-2334ec0d472e", Status: operation.StatusInProgress, CreatedAt: dates[2], SchedulerName: schedulerName, DefinitionName: "create_scheduler"},
		}

		operationFlow.EXPECT().ListSchedulerPendingOperationIDs(gomock.Any(), schedulerName).Return([]string{"1", "2", "3"}, nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), schedulerName, "1").Return(pendingOperations[0], []byte{}, nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), schedulerName, "2").Return(pendingOperations[1], []byte{}, nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), schedulerName, "3").Return(pendingOperations[2], []byte{}, nil)
		operationStorage.EXPECT().ListSchedulerFinishedOperations(gomock.Any(), schedulerName).Return(finishedOperations, nil)
		operationStorage.EXPECT().ListSchedulerActiveOperations(gomock.Any(), schedulerName).Return(activeOperations, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?", nil)
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
			[]interface{}(
				[]interface{}{
					map[string]interface{}{"createdAt": "2021-03-01T00:00:00Z", "definitionName": "create_scheduler", "id": "2d88b86b-0e70-451c-93cf-2334ec0d472e", "schedulerName": schedulerName, "status": "in_progress"},
					map[string]interface{}{"createdAt": "2021-02-01T00:00:00Z", "definitionName": "create_scheduler", "id": "59e58c61-1758-4f02-b6ea-a87a64172902", "schedulerName": schedulerName, "status": "in_progress"},
					map[string]interface{}{"createdAt": "2021-01-01T00:00:00Z", "definitionName": "create_scheduler", "id": "72e108f8-8025-4e96-9f3f-b81ac5b40d50", "schedulerName": schedulerName, "status": "in_progress"},
				}),
			body["activeOperations"])
		require.Equal(t,
			[]interface{}(
				[]interface{}{
					map[string]interface{}{"createdAt": "2021-03-01T00:00:00Z", "definitionName": "create_scheduler", "id": "ae218cc1-2dd8-448b-a78f-0cc979f89f37", "schedulerName": schedulerName, "status": "finished"},
					map[string]interface{}{"createdAt": "2021-02-01T00:00:00Z", "definitionName": "create_scheduler", "id": "f1fce7b2-3374-464e-9eb4-08b25fa0da54", "schedulerName": schedulerName, "status": "finished"},
					map[string]interface{}{"createdAt": "2021-01-01T00:00:00Z", "definitionName": "create_scheduler", "id": "c241b467-db15-42ba-b2a8-017c37234237", "schedulerName": schedulerName, "status": "finished"},
				}),
			body["finishedOperations"])
		require.Equal(t,
			[]interface{}(
				[]interface{}{
					map[string]interface{}{"createdAt": "2021-03-01T00:00:00Z", "definitionName": "create_scheduler", "id": "83cc7850-9c90-4033-948f-368eea4b976e", "schedulerName": schedulerName, "status": "pending"},
					map[string]interface{}{"createdAt": "2021-02-01T00:00:00Z", "definitionName": "create_scheduler", "id": "7af3250c-af5b-428a-955f-a8fa22fb7cf7", "schedulerName": schedulerName, "status": "pending"},
					map[string]interface{}{"createdAt": "2021-01-01T00:00:00Z", "definitionName": "create_scheduler", "id": "d28f3fc7-ca32-4ca8-8b6a-8fbb19003389", "schedulerName": schedulerName, "status": "pending"},
				}),
			body["pendingOperations"])
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
