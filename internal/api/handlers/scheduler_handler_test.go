//+build integration

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	opflow "github.com/topfreegames/maestro/internal/adapters/operation_flow/mock"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	schedulerStorageMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func TestGetAllSchedulers(t *testing.T) {

	t.Run("with valid request and persisted scheduler", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetAllSchedulers(gomock.Any()).Return([]*entities.Scheduler{
			{
				Name:            "zooba-us",
				Game:            "zooba",
				State:           entities.StateInSync,
				RollbackVersion: "1.0.0",
				PortRange: &entities.PortRange{
					Start: 1,
					End:   2,
				},
			},
		}, nil)

		mux := runtime.NewServeMux()
		api.RegisterSchedulersHandlerServer(context.Background(), mux, ProvideSchedulerHandler(schedulerManager, nil))

		req, err := http.NewRequest("GET", "/schedulers", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)

		bodyString := rr.Body.String()
		var reply api.ListSchedulersReply
		json.Unmarshal([]byte(bodyString), &reply)

		require.NotEmpty(t, reply.Schedulers)
	})

	t.Run("with valid request and no scheduler found", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetAllSchedulers(gomock.Any()).Return([]*entities.Scheduler{}, nil)

		mux := runtime.NewServeMux()
		api.RegisterSchedulersHandlerServer(context.Background(), mux, ProvideSchedulerHandler(schedulerManager, nil))

		req, err := http.NewRequest("GET", "/schedulers", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)

		bodyString := rr.Body.String()
		var reply api.ListSchedulersReply
		json.Unmarshal([]byte(bodyString), &reply)

		require.Empty(t, reply.Schedulers)
	})

	t.Run("with invalid request method", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mux := runtime.NewServeMux()
		api.RegisterSchedulersHandlerServer(context.Background(), mux, ProvideSchedulerHandler(nil, nil))

		req, err := http.NewRequest("PUT", "/schedulers", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 501, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)

		require.NoError(t, err)
		require.Equal(t, "Method Not Allowed", body["message"])
	})
}

func TestCreateScheduler(t *testing.T) {

	t.Run("with success", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors())
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, operationManager)

		scheduler := &entities.Scheduler{
			Name:  "scheduler",
			Game:  "game",
			State: entities.StateCreating,
			Spec: game_room.Spec{
				Version: "v1",
			},
		}

		schedulerStorage.EXPECT().CreateScheduler(gomock.Any(), gomock.Eq(scheduler)).Return(nil)
		operationStorage.EXPECT().CreateOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		operationFlow.EXPECT().InsertOperationID(gomock.Any(), "scheduler", gomock.Any()).Return(nil)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler").Return(scheduler, nil)

		mux := runtime.NewServeMux()
		api.RegisterSchedulersHandlerServer(context.Background(), mux, ProvideSchedulerHandler(schedulerManager, operationManager))

		reqBody := &api.CreateSchedulerRequest{
			Name:    "scheduler",
			Game:    "game",
			Version: "v1",
		}
		reqBodyString, err := json.Marshal(reqBody)
		req, err := http.NewRequest(http.MethodPost, "/schedulers", bytes.NewReader(reqBodyString))
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
		require.Equal(t, map[string]interface{}{
			"game":      "game",
			"name":      "scheduler",
			"portRange": interface{}(nil),
			"state":     "creating",
			"version":   "v1",
		}, body)
	})

	t.Run("fails when scheduler already exists", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().CreateScheduler(gomock.Any(), gomock.Any()).Return(errors.NewErrAlreadyExists("error creating scheduler %s: name already exists", "scheduler"))

		mux := runtime.NewServeMux()
		api.RegisterSchedulersHandlerServer(context.Background(), mux, ProvideSchedulerHandler(schedulerManager, nil))

		reqBody := &api.CreateSchedulerRequest{
			Name:    "scheduler",
			Game:    "game",
			Version: "v1",
		}

		reqBodyString, err := json.Marshal(reqBody)
		req, err := http.NewRequest(http.MethodPost, "/schedulers", bytes.NewReader(reqBodyString))
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 409, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)

		require.NoError(t, err)
		require.Equal(t, "error creating scheduler scheduler: name already exists", body["message"])
	})
}

func TestListOperations(t *testing.T) {

	t.Run("with success", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors())

		config.EXPECT().GetBool("management_api.enable").Return(true).AnyTimes()
		config.EXPECT().GetString("management_api.port").Return("8081").AnyTimes()
		config.EXPECT().GetString("management_api.gracefulShutdownTimeout").Return("10000").AnyTimes()

		operationFlow.EXPECT().ListSchedulerPendingOperationIDs(gomock.Any(), "zooba").Return([]string{}, nil)
		operationStorage.EXPECT().ListSchedulerFinishedOperations(gomock.Any(), "zooba").Return([]*operation.Operation{}, nil)
		operationStorage.EXPECT().ListSchedulerActiveOperations(gomock.Any(), "zooba").Return([]*operation.Operation{
			{
				ID:             "operation-1",
				Status:         operation.StatusInProgress,
				DefinitionName: "create_scheduler",
				SchedulerName:  "zooba",
			},
		}, nil)

		mux := runtime.NewServeMux()
		api.RegisterSchedulersHandlerServer(context.Background(), mux, ProvideSchedulerHandler(nil, operationManager))

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
					"iD":             "operation-1",
					"schedulerName":  "zooba",
					"status":         "in_progress",
				},
			}), body["activeOperations"])
	})

	t.Run("fails when operation is listed but does not exists", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		config := configmock.NewMockConfig(mockCtrl)

		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors())

		config.EXPECT().GetBool("management_api.enable").Return(true).AnyTimes()
		config.EXPECT().GetString("management_api.port").Return("8081").AnyTimes()
		config.EXPECT().GetString("management_api.gracefulShutdownTimeout").Return("10000").AnyTimes()

		operationID := "operation-1"
		schedulerName := "zooba"

		operationFlow.EXPECT().ListSchedulerPendingOperationIDs(gomock.Any(), schedulerName).Return([]string{operationID}, nil)
		operationStorage.EXPECT().GetOperation(gomock.Any(), schedulerName, operationID).Return(nil, nil, errors.NewErrNotFound("operation %s not found in scheduler %s", operationID, schedulerName))

		mux := runtime.NewServeMux()
		api.RegisterSchedulersHandlerServer(context.Background(), mux, ProvideSchedulerHandler(nil, operationManager))

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
			"failed to list all pending operations: operation operation-1 not found in scheduler zooba",
			body["message"])
	})
}
