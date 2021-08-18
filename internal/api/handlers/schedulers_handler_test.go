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
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	opflow "github.com/topfreegames/maestro/internal/adapters/operation_flow/mock"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	schedulerStorageMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
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
				CreatedAt:       time.Now(),
				PortRange: &entities.PortRange{
					Start: 1,
					End:   2,
				},
			},
		}, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/schedulers", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)

		bodyString := rr.Body.String()
		var response api.ListSchedulersResponse
		err = json.Unmarshal([]byte(bodyString), &response)
		require.NoError(t, err)

		require.NotEmpty(t, response.Schedulers)
	})

	t.Run("with valid request and no scheduler found", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetAllSchedulers(gomock.Any()).Return([]*entities.Scheduler{}, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/schedulers", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)

		bodyString := rr.Body.String()
		var response api.ListSchedulersResponse
		err = json.Unmarshal([]byte(bodyString), &response)
		require.NoError(t, err)

		require.Empty(t, response.Schedulers)
	})

	t.Run("with invalid request method", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(nil))
		require.NoError(t, err)

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
	dirPath, _ := os.Getwd()

	t.Run("with success", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors())
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, operationManager)

		scheduler := &entities.Scheduler{
			Name:  "scheduler-name-1",
			Game:  "game-name",
			State: entities.StateCreating,
			Spec: game_room.Spec{
				Version:                "v1.0.0",
				TerminationGracePeriod: 100 * time.Nanosecond,
				Containers: []game_room.Container{
					{
						Name:            "game-room-container-name",
						Image:           "game-room-container-image",
						ImagePullPolicy: "IfNotPresent",
						Command:         []string{"./run"},
						Environment: []game_room.ContainerEnvironment{{
							Name:  "env-var-name",
							Value: "env-var-value",
						}},
						Requests: game_room.ContainerResources{
							Memory: "100mi",
							CPU:    "100m",
						},
						Limits: game_room.ContainerResources{
							Memory: "200mi",
							CPU:    "200m",
						},
						Ports: []game_room.ContainerPort{{
							Name:     "container-port-name",
							Protocol: "https",
							Port:     12345,
							HostPort: 54321,
						}},
					},
				},
			},
			PortRange: &entities.PortRange{
				Start: 1,
				End:   1000,
			},
		}

		schedulerStorage.EXPECT().CreateScheduler(gomock.Any(), gomock.Eq(scheduler)).Return(nil)
		operationStorage.EXPECT().CreateOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		operationFlow.EXPECT().InsertOperationID(gomock.Any(), "scheduler-name-1", gomock.Any()).Return(nil)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(scheduler, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		request, err := ioutil.ReadFile(dirPath + "/fixtures/scheduler-config.json")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)
		require.NoError(t, err)

		schedulerContent, ok := body["scheduler"].(map[string]interface{})
		require.NotNil(t, schedulerContent)
		require.True(t, ok)

		require.Equal(t, "game-name", schedulerContent["game"])
		require.Equal(t, "scheduler-name-1", schedulerContent["name"])
		require.NotNil(t, schedulerContent["portRange"])
		require.Equal(t, "creating", schedulerContent["state"])
		require.Equal(t, "v1.0.0", schedulerContent["version"])
		require.NotNil(t, schedulerContent["createdAt"])
	})

	t.Run("fails when scheduler already exists", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().CreateScheduler(gomock.Any(), gomock.Any()).Return(errors.NewErrAlreadyExists("error creating scheduler %s: name already exists", "scheduler"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		request, err := ioutil.ReadFile(dirPath + "/fixtures/scheduler-config.json")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers", bytes.NewReader(request))
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
