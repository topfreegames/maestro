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

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/ports/mock"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func TestListOperations(t *testing.T) {
	schedulerName := "zooba"

	dates := []time.Time{
		time.Time{}.AddDate(2020, 0, 0),
		time.Time{}.AddDate(2020, 1, 0),
		time.Time{}.AddDate(2020, 2, 0),
	}
	pendingOperations := []*operation.Operation{
		&operation.Operation{
			ID:             "d28f3fc7-ca32-4ca8-8b6a-8fbb19003389",
			Status:         operation.StatusPending,
			CreatedAt:      dates[0],
			SchedulerName:  schedulerName,
			DefinitionName: "create_scheduler",
			ExecutionHistory: []operation.OperationEvent{
				{
					CreatedAt: time.Date(1999, time.November, 29, 8, 0, 0, 0, time.UTC),
					Event:     "some-event",
				},
			},
			Input: []byte("{\"scheduler\": {\"name\": \"some-scheduler\"}}"),
		},
		&operation.Operation{
			ID:             "7af3250c-af5b-428a-955f-a8fa22fb7cf7",
			Status:         operation.StatusPending,
			CreatedAt:      dates[1],
			SchedulerName:  schedulerName,
			DefinitionName: "create_scheduler",
			ExecutionHistory: []operation.OperationEvent{
				{
					CreatedAt: time.Date(1999, time.November, 29, 8, 0, 0, 0, time.UTC),
					Event:     "some-event",
				},
			},
			Input: []byte("{\"scheduler\": {\"name\": \"some-scheduler\"}}"),
		},
		&operation.Operation{
			ID:             "83cc7850-9c90-4033-948f-368eea4b976e",
			Status:         operation.StatusPending,
			CreatedAt:      dates[2],
			SchedulerName:  schedulerName,
			DefinitionName: "create_scheduler",
			ExecutionHistory: []operation.OperationEvent{
				{
					CreatedAt: time.Date(1999, time.November, 29, 8, 0, 0, 0, time.UTC),
					Event:     "some-event",
				},
			},
			Input: []byte("{\"scheduler\": {\"name\": \"some-scheduler\"}}"),
		},
	}
	finishedOperations := []*operation.Operation{
		&operation.Operation{
			ID:             "c241b467-db15-42ba-b2a8-017c37234237",
			Status:         operation.StatusFinished,
			CreatedAt:      dates[0],
			SchedulerName:  schedulerName,
			DefinitionName: "create_scheduler",
			ExecutionHistory: []operation.OperationEvent{
				{
					CreatedAt: time.Date(1999, time.November, 29, 8, 0, 0, 0, time.UTC),
					Event:     "some-event",
				},
			},
			Input: []byte("{\"scheduler\": {\"name\": \"some-scheduler\"}}"),
		},
		&operation.Operation{
			ID:             "f1fce7b2-3374-464e-9eb4-08b25fa0da54",
			Status:         operation.StatusFinished,
			CreatedAt:      dates[1],
			SchedulerName:  schedulerName,
			DefinitionName: "create_scheduler",
			ExecutionHistory: []operation.OperationEvent{
				{
					CreatedAt: time.Date(1999, time.November, 29, 8, 0, 0, 0, time.UTC),
					Event:     "some-event",
				},
			},
			Input: []byte("{\"scheduler\": {\"name\": \"some-scheduler\"}}"),
		},
		&operation.Operation{
			ID:             "ae218cc1-2dd8-448b-a78f-0cc979f89f37",
			Status:         operation.StatusFinished,
			CreatedAt:      dates[2],
			SchedulerName:  schedulerName,
			DefinitionName: "create_scheduler",
			ExecutionHistory: []operation.OperationEvent{
				{
					CreatedAt: time.Date(1999, time.November, 29, 8, 0, 0, 0, time.UTC),
					Event:     "some-event",
				},
			},
			Input: []byte("{\"scheduler\": {\"name\": \"some-scheduler\"}}"),
		},
	}

	activeOperations := []*operation.Operation{
		&operation.Operation{
			ID:             "72e108f8-8025-4e96-9f3f-b81ac5b40d50",
			Status:         operation.StatusInProgress,
			CreatedAt:      dates[0],
			SchedulerName:  schedulerName,
			DefinitionName: "create_scheduler",
			Lease:          &operation.OperationLease{Ttl: time.Unix(1641306511, 0)},
			ExecutionHistory: []operation.OperationEvent{
				{
					CreatedAt: time.Date(1999, time.November, 29, 8, 0, 0, 0, time.UTC),
					Event:     "some-event",
				},
			},
			Input: []byte("{\"scheduler\": {\"name\": \"some-scheduler\"}}"),
		},
		&operation.Operation{
			ID:             "59e58c61-1758-4f02-b6ea-a87a64172902",
			Status:         operation.StatusInProgress,
			CreatedAt:      dates[1],
			SchedulerName:  schedulerName,
			DefinitionName: "create_scheduler",
			Lease:          &operation.OperationLease{Ttl: time.Unix(1641306521, 0)},
			ExecutionHistory: []operation.OperationEvent{
				{
					CreatedAt: time.Date(1999, time.November, 29, 8, 0, 0, 0, time.UTC),
					Event:     "some-event",
				},
			},
			Input: []byte("{\"scheduler\": {\"name\": \"some-scheduler\"}}"),
		},
		&operation.Operation{
			ID:             "2d88b86b-0e70-451c-93cf-2334ec0d472e",
			Status:         operation.StatusInProgress,
			CreatedAt:      dates[2],
			SchedulerName:  schedulerName,
			DefinitionName: "create_scheduler",
			Lease:          &operation.OperationLease{Ttl: time.Unix(1641306531, 0)},
			ExecutionHistory: []operation.OperationEvent{
				{
					CreatedAt: time.Date(1999, time.November, 29, 8, 0, 0, 0, time.UTC),
					Event:     "some-event",
				},
			},
			Input: []byte("{\"scheduler\": {\"name\": \"some-scheduler\"}}"),
		},
	}

	t.Run("with success and default sorting", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationManager.EXPECT().ListSchedulerFinishedOperations(gomock.Any(), schedulerName, int64(0), int64(15), "desc").Return(finishedOperations, int64(3), nil)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=final", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, 200, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/list_operations_default_sorting.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with success and ascending sorting", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationManager.EXPECT().ListSchedulerFinishedOperations(gomock.Any(), schedulerName, int64(0), int64(15), "asc").Return(finishedOperations, int64(3), nil)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=final&order_by=createdAt asc", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/list_operations_ascending_sorting.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with success and descending sorting", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationManager.EXPECT().ListSchedulerFinishedOperations(gomock.Any(), schedulerName, int64(0), int64(15), "desc").Return(finishedOperations, int64(3), nil)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=final&order_by=createdAt desc", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)

		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/list_operations_descending_sorting.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with invalid sorting field", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=final&order_by=invalidField", nil)
		if err != nil {
			t.Fatal(err)
		}

		mux.ServeHTTP(rr, req)

		require.Equal(t, 400, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)
		require.NoError(t, err)
		require.Equal(t, "invalid sorting field: invalidField", body["message"])
	})

	t.Run("with invalid sorting order", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=final&order_by=createdAt invalidOrder", nil)
		if err != nil {
			t.Fatal(err)
		}

		mux.ServeHTTP(rr, req)

		require.Equal(t, 400, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)
		require.NoError(t, err)
		require.Equal(t, "invalid sorting order: invalidOrder", body["message"])
	})

	t.Run("with success and operations pending stage", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationManager.EXPECT().ListSchedulerPendingOperations(gomock.Any(), schedulerName).Return(pendingOperations, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=pending", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, 200, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/list_operations_pending_stage_success.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with success and operations active stage", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationManager.EXPECT().ListSchedulerActiveOperations(gomock.Any(), schedulerName).Return(activeOperations, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=active", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, 200, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/list_operations_active_stage_success.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with success and operations final stage with default pagination", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationManager.EXPECT().ListSchedulerFinishedOperations(gomock.Any(), schedulerName, int64(0), int64(15), "desc").Return(finishedOperations, int64(3), nil)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=final", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, 200, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/list_operations_final_stage_success.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with success and operations final stage with custom pagination", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationManager.EXPECT().ListSchedulerFinishedOperations(gomock.Any(), schedulerName, int64(0), int64(10), "desc").Return(finishedOperations, int64(13), nil)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=final&page=1&perPage=10", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, 200, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/list_operations_final_stage_pagination_success.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with error when listing operations in pending stage", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		operationManager.EXPECT().ListSchedulerPendingOperations(gomock.Any(), schedulerName).Return(nil, errors.NewErrUnexpected("some error"))

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=pending", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, 500, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/error_listing_pending_operations.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with error when listing operations in pending stage with invalid pagination parameters", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=pending&page=1&perPage=15", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, 400, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/error_listing_pending_operations_with_pag_parameters.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with error when listing operations in active stage", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationManager.EXPECT().ListSchedulerActiveOperations(gomock.Any(), schedulerName).Return(nil, errors.NewErrUnexpected("some error"))

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=active", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, 500, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/error_listing_active_operations.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with error when listing operations in active stage with invalid pagination parameters", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=active&page=1&perPage=15", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, 400, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/error_listing_active_operations_with_pag_parameters.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with error when listing operations in final stage", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationManager.EXPECT().ListSchedulerFinishedOperations(gomock.Any(), schedulerName, int64(0), int64(15), "desc").Return(nil, int64(0), errors.NewErrUnexpected("error listing finished operations"))

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=final", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, 500, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/error_listing_final_operations.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with error when listing operations in final with invalid pagination parameters", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=final&page=0", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, 400, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/error_listing_final_operations_invalid_page.json")
		require.Equal(t, expectedResponseBody, responseBody)

		req, err = http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=final&page=10&perPage=101", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr = httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, 400, rr.Code)
		responseBody, expectedResponseBody = extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/error_listing_final_operations_invalid_per_page.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with error when listing operations in invalid stage", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, "/schedulers/zooba/operations?stage=invalid", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, 400, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/error_listing_invalid_operations.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

}

func TestCancelOperation(t *testing.T) {
	schedulerName := uuid.New().String()
	operationID := uuid.New().String()

	t.Run("enqueues operation cancellation request with success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationManager.EXPECT().EnqueueOperationCancellationRequest(gomock.Any(), schedulerName, operationID)

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

	t.Run("fails to enqueues operation cancellation request when operation is already on final status", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationManager.EXPECT().EnqueueOperationCancellationRequest(gomock.Any(), schedulerName, operationID).Return(errors.NewErrConflict("operation on final status"))

		mux := runtime.NewServeMux()
		err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("/schedulers/%s/operations/%s/cancel", schedulerName, operationID), nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 409, rr.Code)
	})

	t.Run("fails to enqueues operation cancellation request when operation_flow fails", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		operationManager := mock.NewMockOperationManager(mockCtrl)

		operationManager.EXPECT().EnqueueOperationCancellationRequest(gomock.Any(), schedulerName, operationID).Return(errors.NewErrUnexpected("failed to persist request"))

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

func TestGetOperation(t *testing.T) {
	dates := []time.Time{
		time.Time{}.AddDate(2020, 0, 0),
		time.Time{}.AddDate(2020, 1, 0),
		time.Time{}.AddDate(2020, 2, 0),
	}

	type TestMock struct {
		Operation *operation.Operation
		Err       error
	}
	type TestInput struct {
		OperationId   string
		SchedulerName string
	}
	type TestOutput struct {
		Operation *operation.Operation
		Status    int
	}
	type Test struct {
		Description string
		Input       TestInput
		Output      TestOutput
		Mock        TestMock
	}

	for _, test := range []Test{
		Test{
			Description: "returns 200 with operation when operation found",
			Mock: TestMock{
				Operation: &operation.Operation{
					ID:             "d28f3fc7-ca32-4ca8-8b6a-8fbb19003389",
					Status:         operation.StatusPending,
					CreatedAt:      dates[0],
					SchedulerName:  "scheduler",
					DefinitionName: "create_scheduler",
					ExecutionHistory: []operation.OperationEvent{
						{
							CreatedAt: time.Date(1999, time.November, 29, 8, 0, 0, 0, time.UTC),
							Event:     "some-event",
						},
					},
					Input: []byte("{\"scheduler\": {\"name\": \"some-scheduler\"}}"),
				},
			},
			Input: TestInput{
				OperationId:   "d28f3fc7-ca32-4ca8-8b6a-8fbb19003389",
				SchedulerName: "scheduler",
			},
			Output: TestOutput{
				Operation: &operation.Operation{
					ID:             "d28f3fc7-ca32-4ca8-8b6a-8fbb19003389",
					Status:         operation.StatusPending,
					CreatedAt:      dates[0],
					SchedulerName:  "scheduler",
					DefinitionName: "create_scheduler",
					ExecutionHistory: []operation.OperationEvent{
						{
							CreatedAt: time.Date(1999, time.November, 29, 8, 0, 0, 0, time.UTC),
							Event:     "some-event",
						},
					},
					Input: []byte("{\"scheduler\": {\"name\": \"some-scheduler\"}}"),
				},
				Status: 200,
			},
		},
		Test{
			Description: "returns 404 when op not found",
			Mock: TestMock{
				Operation: &operation.Operation{
					ID:             "d28f3fc7-ca32-4ca8-8b6a-8fbb19003389",
					Status:         operation.StatusPending,
					CreatedAt:      dates[0],
					SchedulerName:  "scheduler",
					DefinitionName: "create_scheduler",
					ExecutionHistory: []operation.OperationEvent{
						{
							CreatedAt: time.Date(1999, time.November, 29, 8, 0, 0, 0, time.UTC),
							Event:     "some-event",
						},
					},
					Input: []byte("{\"scheduler\": {\"name\": \"some-scheduler\"}}"),
				},
				Err: errors.NewErrNotFound("error"),
			},
			Input: TestInput{
				OperationId:   "NON-EXISTENT-OPERATIONS",
				SchedulerName: "scheduler",
			},
			Output: TestOutput{
				Operation: &operation.Operation{
					ID: "NON-EXISTENT-OPERATIONS",
				},
				Status: 404,
			},
		},
		Test{
			Description: "returns 500 when error unexpected",
			Mock: TestMock{
				Operation: &operation.Operation{
					ID:             "d28f3fc7-ca32-4ca8-8b6a-8fbb19003389",
					Status:         operation.StatusPending,
					CreatedAt:      dates[0],
					SchedulerName:  "scheduler",
					DefinitionName: "create_scheduler",
					ExecutionHistory: []operation.OperationEvent{
						{
							CreatedAt: time.Date(1999, time.November, 29, 8, 0, 0, 0, time.UTC),
							Event:     "some-event",
						},
					},
					Input: []byte("{\"scheduler\": {\"name\": \"some-scheduler\"}}"),
				},
				Err: errors.NewErrUnexpected("error"),
			},
			Input: TestInput{
				OperationId:   "d28f3fc7-ca32-4ca8-8b6a-8fbb19003389",
				SchedulerName: "scheduler",
			},
			Output: TestOutput{
				Operation: &operation.Operation{
					ID: "d28f3fc7-ca32-4ca8-8b6a-8fbb19003389",
				},
				Status: 500,
			},
		},
	} {
		t.Run(test.Description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			operationManager := mock.NewMockOperationManager(mockCtrl)

			operationManager.EXPECT().GetOperation(gomock.Any(), test.Input.SchedulerName, test.Input.OperationId).Return(test.Mock.Operation, nil, test.Mock.Err)

			mux := runtime.NewServeMux()
			err := api.RegisterOperationsServiceHandlerServer(context.Background(), mux, ProvideOperationsHandler(operationManager))
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/schedulers/%s/operations/%s", test.Input.SchedulerName, test.Input.OperationId), nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)
			require.Equal(t, test.Output.Status, rr.Code)

			if rr.Code == 200 {
				responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "operations_handler/get_operation.json")
				require.Equal(t, responseBody, expectedResponseBody)
			}
		})
	}
}

func extractBodyForComparison(t *testing.T, body []byte, expectedBodyFixturePath string) (string, string) {
	fixture, err := os.ReadFile(fmt.Sprintf("%s/response/%s", fixturesRelativePath, expectedBodyFixturePath))
	require.NoError(t, err)
	bodyBuffer := new(bytes.Buffer)
	expectedBodyBuffer := new(bytes.Buffer)
	err = json.Compact(bodyBuffer, body)
	require.NoError(t, err)
	err = json.Compact(expectedBodyBuffer, fixture)
	require.NoError(t, err)
	return bodyBuffer.String(), expectedBodyBuffer.String()
}
