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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/forwarder"

	"github.com/golang/mock/gomock"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	opflow "github.com/topfreegames/maestro/internal/adapters/operation_flow/mock"
	oplstorage "github.com/topfreegames/maestro/internal/adapters/operation_lease/mock"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	schedulerStorageMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/operations"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/services/operation_manager"
	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"
	"github.com/topfreegames/maestro/internal/validations"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func TestGetAllSchedulers(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}
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
				MaxSurge:        "10%",
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

func TestGetScheduler(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}
	t.Run("with valid request and persisted scheduler", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetSchedulerWithFilter(gomock.Any(), gomock.Any()).Return(&entities.Scheduler{
			Name:            "zooba-us",
			Game:            "zooba",
			State:           entities.StateInSync,
			MaxSurge:        "10%",
			RollbackVersion: "1.0.0",
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
			CreatedAt: time.Now(),
			PortRange: &entities.PortRange{
				Start: 1,
				End:   2,
			},
		}, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/schedulers/zooba-us", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)

		bodyString := rr.Body.String()
		var response api.GetSchedulerResponse
		err = json.Unmarshal([]byte(bodyString), &response)
		require.NoError(t, err)

		require.NotEmpty(t, response.Scheduler)
	})

	t.Run("with valid request and no scheduler found", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetSchedulerWithFilter(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrNotFound("scheduler NonExistentSchedule not found"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/schedulers/NonExistentSchedule", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 404, rr.Code)

		bodyString := rr.Body.String()
		var response api.GetSchedulerResponse
		err = json.Unmarshal([]byte(bodyString), &response)
		require.NoError(t, err)

		require.Empty(t, response.Scheduler)
	})

	t.Run("with invalid request", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetSchedulerWithFilter(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrInvalidArgument("Error"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/schedulers/NonExistentSchedule", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 500, rr.Code)

		bodyString := rr.Body.String()
		var response api.GetSchedulerResponse
		err = json.Unmarshal([]byte(bodyString), &response)
		require.NoError(t, err)

		require.Empty(t, response.Scheduler)
	})

}

func TestGetSchedulerVersions(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}
	t.Run("with valid request and persisted scheduler", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		versions := make([]*entities.SchedulerVersion, 1)
		versions[0] = &entities.SchedulerVersion{
			Version:   "v1.1",
			CreatedAt: time.Now(),
		}

		schedulerStorage.EXPECT().GetSchedulerVersions(gomock.Any(), gomock.Any()).Return(versions, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/schedulers/scheduler/versions", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)

		bodyString := rr.Body.String()
		var response api.GetSchedulerVersionsResponse
		err = json.Unmarshal([]byte(bodyString), &response)
		require.NoError(t, err)

		require.NotEmpty(t, response.Versions)
	})

	t.Run("with valid request and no scheduler found", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetSchedulerVersions(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrNotFound("scheduler NonExistentScheduler not found"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/schedulers/NonExistentScheduler/versions", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 404, rr.Code)

		bodyString := rr.Body.String()
		var response api.GetSchedulerVersionsResponse
		err = json.Unmarshal([]byte(bodyString), &response)
		require.NoError(t, err)

		require.Empty(t, response.Versions)
	})

	t.Run("with invalid request", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetSchedulerVersions(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrInvalidArgument("Error"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/schedulers/NonExistentScheduler/versions", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, 500, rr.Code)

		bodyString := rr.Body.String()
		var response api.GetSchedulerVersionsResponse
		err = json.Unmarshal([]byte(bodyString), &response)
		require.NoError(t, err)

		require.Empty(t, response.Versions)
	})

}

func TestCreateScheduler(t *testing.T) {
	dirPath, _ := os.Getwd()

	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	t.Run("with success", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationLeaseStorage := oplstorage.NewMockOperationLeaseStorage(mockCtrl)
		config := operation_manager.OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors(), operationLeaseStorage, config)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, operationManager)

		scheduler := &entities.Scheduler{
			Name:     "scheduler-name-1",
			Game:     "game-name",
			State:    entities.StateCreating,
			MaxSurge: "10%",
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
							Protocol: "tcp",
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

		request, err := ioutil.ReadFile(dirPath + "/fixtures/request/scheduler-config.json")
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
		require.Equal(t, "10%", schedulerContent["maxSurge"])
		require.NotNil(t, schedulerContent["createdAt"])
	})

	t.Run("with failure", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerManager := scheduler_manager.NewSchedulerManager(nil, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		request, err := ioutil.ReadFile(dirPath + "/fixtures/request/bad-scheduler-config.json")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 400, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)
		require.NoError(t, err)

		schedulerMessage, ok := body["message"]
		require.True(t, ok)
		require.NotNil(t, schedulerMessage)
		require.Contains(t, schedulerMessage, "Key: 'Scheduler.Name' Error:Field validation for 'Name' failed on the 'required' tag")
		require.Contains(t, schedulerMessage, "Key: 'Scheduler.Spec.Containers[0].Environment[0].Name' Error:Field validation for 'Name' failed on the 'required' tag")
		require.Contains(t, schedulerMessage, "Key: 'Scheduler.Spec.Containers[0].ImagePullPolicy' Error:Field validation for 'ImagePullPolicy' failed on the 'image_pull_policy' tag")
		require.Contains(t, schedulerMessage, "Key: 'Scheduler.Spec.Containers[0].Requests.Memory' Error:Field validation for 'Memory' failed on the 'required' tag")
		require.Contains(t, schedulerMessage, "Key: 'Scheduler.Spec.TerminationGracePeriod' Error:Field validation for 'TerminationGracePeriod' failed on the 'gt' tag")
		require.Contains(t, schedulerMessage, "Key: 'Scheduler.Spec.Version' Error:Field validation for 'Version' failed on the 'required' tag")
		require.Contains(t, schedulerMessage, "Key: 'Scheduler.Spec.Containers[0].Command' Error:Field validation for 'Command' failed on the 'required' tag")
		require.Contains(t, schedulerMessage, "Key: 'Scheduler.Game' Error:Field validation for 'Game' failed on the 'required' tag")
		require.Contains(t, schedulerMessage, "Key: 'Scheduler.Spec.Containers[0].Requests.CPU' Error:Field validation for 'CPU' failed on the 'required' tag")
		require.Contains(t, schedulerMessage, "Key: 'Scheduler.Spec.Containers[0].Name' Error:Field validation for 'Name' failed on the 'required' tag")
		require.Contains(t, schedulerMessage, "Key: 'Scheduler.MaxSurge' Error:Field validation for 'MaxSurge' failed on the 'required' tag")
		require.Contains(t, schedulerMessage, "Key: 'Scheduler.Spec.Containers[0].Image' Error:Field validation for 'Image' failed on the 'required' tag")
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

		request, err := ioutil.ReadFile(dirPath + "/fixtures/request/scheduler-config.json")
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

func TestAddRooms(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}
	t.Run("with success", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationLeaseStorage := oplstorage.NewMockOperationLeaseStorage(mockCtrl)
		config := operation_manager.OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors(), operationLeaseStorage, config)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, operationManager)

		operationStorage.EXPECT().CreateOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		operationFlow.EXPECT().InsertOperationID(gomock.Any(), "scheduler-name-1", gomock.Any()).Return(nil)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(nil, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers/scheduler-name-1/add-rooms", bytes.NewReader([]byte("{\"amount\": 10}")))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)
		require.NoError(t, err)

		require.NotEmpty(t, body["operationId"])
	})

	t.Run("fails when scheduler does not exists", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(nil, errors.NewErrNotFound("err"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers/scheduler-name-1/add-rooms", bytes.NewReader([]byte("{\"amount\": 10}")))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 404, rr.Code)
		require.Contains(t, rr.Body.String(), "no scheduler found to add rooms on it: err")
	})

	t.Run("fails when operation enqueue fails", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationLeaseStorage := oplstorage.NewMockOperationLeaseStorage(mockCtrl)
		config := operation_manager.OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		operationManager := operation_manager.New(nil, operationStorage, operations.NewDefinitionConstructors(), operationLeaseStorage, config)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, operationManager)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(nil, nil)
		operationStorage.EXPECT().CreateOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.NewErrUnexpected("storage offline"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers/scheduler-name-1/add-rooms", bytes.NewReader([]byte("{\"amount\": 10}")))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 500, rr.Code)
		require.Contains(t, rr.Body.String(), "not able to schedule the 'add rooms' operation: failed to create operation: storage offline")
	})
}

func TestRemoveRooms(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}
	t.Run("with success", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationLeaseStorage := oplstorage.NewMockOperationLeaseStorage(mockCtrl)
		config := operation_manager.OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors(), operationLeaseStorage, config)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, operationManager)

		operationStorage.EXPECT().CreateOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		operationFlow.EXPECT().InsertOperationID(gomock.Any(), "scheduler-name-1", gomock.Any()).Return(nil)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(nil, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers/scheduler-name-1/remove-rooms", bytes.NewReader([]byte("{\"amount\": 10}")))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)
		require.NoError(t, err)

		require.NotEmpty(t, body["operationId"])
	})

	t.Run("fails when scheduler does not exists", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(nil, errors.NewErrNotFound("err"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers/scheduler-name-1/remove-rooms", bytes.NewReader([]byte("{\"amount\": 10}")))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 404, rr.Code)
		require.Contains(t, rr.Body.String(), "no scheduler found for removing rooms: err")
	})

	t.Run("fails when operation enqueue fails", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationLeaseStorage := oplstorage.NewMockOperationLeaseStorage(mockCtrl)
		config := operation_manager.OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		operationManager := operation_manager.New(nil, operationStorage, operations.NewDefinitionConstructors(), operationLeaseStorage, config)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, operationManager)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(nil, nil)
		operationStorage.EXPECT().CreateOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.NewErrUnexpected("storage offline"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers/scheduler-name-1/remove-rooms", bytes.NewReader([]byte("{\"amount\": 10}")))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 500, rr.Code)
		require.Contains(t, rr.Body.String(), "not able to schedule the 'remove rooms' operation: failed to create operation: storage offline")
	})
}

func TestUpdateScheduler(t *testing.T) {
	dirPath, _ := os.Getwd()

	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	t.Run("with success", func(t *testing.T) {

		currentScheduler := newValidScheduler()
		currentScheduler.PortRange = &entities.PortRange{Start: 1, End: 2}

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationFlow := opflow.NewMockOperationFlow(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationLeaseStorage := oplstorage.NewMockOperationLeaseStorage(mockCtrl)
		config := operation_manager.OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		operationManager := operation_manager.New(operationFlow, operationStorage, operations.NewDefinitionConstructors(), operationLeaseStorage, config)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, operationManager)

		operationStorage.EXPECT().CreateOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		operationFlow.EXPECT().InsertOperationID(gomock.Any(), "scheduler-name-1", gomock.Any()).Return(nil)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(currentScheduler, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		request, err := ioutil.ReadFile(dirPath + "/fixtures/request/scheduler-config.json")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPut, "/schedulers/scheduler-name-1", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 200, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)
		require.NoError(t, err)

		require.NotEmpty(t, body["operationId"])
	})

	t.Run("fails when scheduler does not exists", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(nil, errors.NewErrNotFound("err"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		request, err := ioutil.ReadFile(dirPath + "/fixtures/request/scheduler-config.json")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPut, "/schedulers/scheduler-name-1", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 404, rr.Code)
		require.Contains(t, rr.Body.String(), "no scheduler found to be updated: err")
	})

	t.Run("with failure", func(t *testing.T) {
		currentScheduler := newValidScheduler()
		currentScheduler.PortRange = &entities.PortRange{Start: 1, End: 2}

		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := schedulerStorageMock.NewMockSchedulerStorage(mockCtrl)
		operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
		operationLeaseStorage := oplstorage.NewMockOperationLeaseStorage(mockCtrl)
		config := operation_manager.OperationManagerConfig{OperationLeaseTtl: time.Millisecond * 1000}
		operationManager := operation_manager.New(nil, operationStorage, operations.NewDefinitionConstructors(), operationLeaseStorage, config)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, operationManager)

		operationStorage.EXPECT().CreateOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.NewErrUnexpected("storage offline"))
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(currentScheduler, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		request, err := ioutil.ReadFile(dirPath + "/fixtures/request/scheduler-config.json")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPut, "/schedulers/scheduler-name-1", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 500, rr.Code)
		require.Contains(t, rr.Body.String(), "failed to schedule 'update scheduler' operation")
	})
}

func newValidScheduler() *entities.Scheduler {
	fwd := &forwarder.Forwarder{
		Name:        "fwd",
		Enabled:     true,
		ForwardType: forwarder.TypeGrpc,
		Address:     "address",
		Options: &forwarder.ForwardOptions{
			Timeout:  time.Second * 5,
			Metadata: nil,
		},
	}
	forwarders := []*forwarder.Forwarder{fwd}

	return &entities.Scheduler{
		Name:            "scheduler-name-1",
		Game:            "game",
		State:           entities.StateCreating,
		MaxSurge:        "10%",
		RollbackVersion: "",
		Spec: game_room.Spec{
			Version:                "v1",
			TerminationGracePeriod: 60,
			Toleration:             "toleration",
			Affinity:               "affinity",
			Containers: []game_room.Container{
				{
					Name:            "default",
					Image:           "some-image",
					ImagePullPolicy: "Always",
					Command:         []string{"hello"},
					Ports: []game_room.ContainerPort{
						{Name: "tcp", Protocol: "tcp", Port: 80},
					},
					Requests: game_room.ContainerResources{
						CPU:    "10m",
						Memory: "100Mi",
					},
					Limits: game_room.ContainerResources{
						CPU:    "10m",
						Memory: "100Mi",
					},
				},
			},
		},
		PortRange: &entities.PortRange{
			Start: 40000,
			End:   60000,
		},
		Forwarders: forwarders,
	}
}
