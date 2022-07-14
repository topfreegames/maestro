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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"

	"github.com/topfreegames/maestro/internal/core/services/scheduler_manager"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/filters"
	portsErrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/internal/core/ports/mock"

	"github.com/topfreegames/maestro/internal/core/entities/forwarder"

	"github.com/golang/mock/gomock"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
	"github.com/topfreegames/maestro/internal/validations"
	api "github.com/topfreegames/maestro/pkg/api/v1"
)

func TestListSchedulers(t *testing.T) {
	t.Run("with valid request with parameters and persisted scheduler", func(t *testing.T) {
		schedulerName := "zooba-us"
		game := "zooba"
		version := "1.0.0-any.version"

		mockCtrl := gomock.NewController(t)

		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil, nil, nil)

		schedulerStorage.EXPECT().GetSchedulersWithFilter(gomock.Any(), &filters.SchedulerFilter{Name: schedulerName, Game: game, Version: version}).Return([]*entities.Scheduler{
			{
				Name:            schedulerName,
				Game:            game,
				State:           entities.StateInSync,
				MaxSurge:        "10%",
				RoomsReplicas:   6,
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

		url := fmt.Sprintf("/schedulers?name=%s&game=%s&version=%s", schedulerName, game, version)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		bodyString := rr.Body.String()
		var response api.ListSchedulersResponse
		err = json.Unmarshal([]byte(bodyString), &response)
		require.NoError(t, err)

		require.NotEmpty(t, response.Schedulers)
	})

	t.Run("with valid request and no scheduler found", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil, nil, nil)

		schedulerStorage.EXPECT().GetSchedulersWithFilter(gomock.Any(), gomock.Any()).Return([]*entities.Scheduler{}, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/schedulers", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		bodyString := rr.Body.String()
		var response api.ListSchedulersResponse
		err = json.Unmarshal([]byte(bodyString), &response)
		require.NoError(t, err)

		require.Empty(t, response.Schedulers)
	})

	t.Run("when GetSchedulersWithFilter return in error should respond with internal server error status code", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil, nil, nil)

		schedulerStorage.EXPECT().GetSchedulersWithFilter(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("GetSchedulersWithFilter error"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/schedulers", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code)

		bodyString := rr.Body.String()
		var response api.ListSchedulersResponse
		err = json.Unmarshal([]byte(bodyString), &response)
		require.NoError(t, err)

		require.Empty(t, response.Schedulers)
	})

	t.Run("with invalid request method", func(t *testing.T) {

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(nil))
		require.NoError(t, err)

		req, err := http.NewRequest("PUT", "/schedulers", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, http.StatusNotImplemented, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)

		require.NoError(t, err)
		require.Equal(t, "Method Not Allowed", body["message"])
	})
}

func TestGetScheduler(t *testing.T) {
	t.Run("with valid request and persisted scheduler", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(nil, schedulerCache, nil, nil)

		scheduler := &entities.Scheduler{
			Name:            "zooba-us",
			Game:            "zooba",
			State:           entities.StateInSync,
			MaxSurge:        "10%",
			RoomsReplicas:   6,
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
						Environment: []game_room.ContainerEnvironment{
							{
								Name:  "env-var-name",
								Value: "env-var-value",
							},
							{
								Name: "env-var-field-ref",
								ValueFrom: &game_room.ValueFrom{
									FieldRef: &game_room.FieldRef{FieldPath: "metadata.name"},
								},
							},
							{
								Name: "env-var-secret-ref",
								ValueFrom: &game_room.ValueFrom{
									SecretKeyRef: &game_room.SecretKeyRef{Name: "secret_name", Key: "secret_key"},
								},
							},
						},
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
			CreatedAt: time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC),
			PortRange: &entities.PortRange{
				Start: 1,
				End:   2,
			},
			Forwarders: []*forwarder.Forwarder{
				{
					Name:        "forwarder-1",
					Enabled:     true,
					ForwardType: "gRPC",
					Address:     "127.0.0.1:9090",
					Options: &forwarder.ForwardOptions{
						Timeout: time.Duration(1000),
					},
				},
			},
		}

		schedulerCache.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(scheduler, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/schedulers/zooba-us", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "schedulers_handler/get_scheduler.json")
		require.Equal(t, expectedResponseBody, responseBody)

	})

	t.Run("with valid request and no scheduler found", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, schedulerCache, nil, nil)

		schedulerCache.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrNotFound("scheduler NonExistentSchedule not found"))
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrNotFound("scheduler NonExistentSchedule not found"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/schedulers/NonExistentSchedule", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, http.StatusNotFound, rr.Code)

		bodyString := rr.Body.String()
		var response api.GetSchedulerResponse
		err = json.Unmarshal([]byte(bodyString), &response)
		require.NoError(t, err)

		require.Empty(t, response.Scheduler)
	})

	t.Run("with invalid request", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, schedulerCache, nil, nil)

		schedulerCache.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrInvalidArgument("Error"))
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrInvalidArgument("Error"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest("GET", "/schedulers/NonExistentSchedule", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code)

		bodyString := rr.Body.String()
		var response api.GetSchedulerResponse
		err = json.Unmarshal([]byte(bodyString), &response)
		require.NoError(t, err)

		require.Empty(t, response.Scheduler)
	})

}

func TestGetSchedulerVersions(t *testing.T) {
	t.Run("with valid request and persisted scheduler", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil, nil, nil)

		createdAtV1, _ := time.Parse(time.RFC3339Nano, "2020-01-01T00:00:00.001Z")
		createdAtV2, _ := time.Parse(time.RFC3339Nano, "2020-01-01T00:00:00.001Z")
		versions := []*entities.SchedulerVersion{
			{
				Version:   "v1.1",
				IsActive:  true,
				CreatedAt: createdAtV1,
			},
			{
				Version:   "v2.0",
				IsActive:  false,
				CreatedAt: createdAtV2,
			},
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

		require.Equal(t, http.StatusOK, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "schedulers_handler/list_versions_success.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with valid request and no scheduler found", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil, nil, nil)

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

		require.Equal(t, http.StatusNotFound, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "schedulers_handler/list_versions_not_found.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with invalid request", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil, nil, nil)

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

		require.Equal(t, http.StatusInternalServerError, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "schedulers_handler/list_versions_invalid_request.json")
		require.Equal(t, expectedResponseBody, responseBody)
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
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		scheduler := &entities.Scheduler{
			Name:          "scheduler-name-1",
			Game:          "game-name",
			State:         entities.StateCreating,
			MaxSurge:      "10%",
			RoomsReplicas: 6,
			Spec: game_room.Spec{
				Version:                "v1.0.0",
				TerminationGracePeriod: 100 * time.Nanosecond,
				Containers: []game_room.Container{
					{
						Name:            "game-room-container-name",
						Image:           "game-room-container-image",
						ImagePullPolicy: "IfNotPresent",
						Command:         []string{"./run"},
						Environment: []game_room.ContainerEnvironment{
							{
								Name:  "env-var-name",
								Value: "env-var-value",
							},
							{
								Name: "env-var-field-ref",
								ValueFrom: &game_room.ValueFrom{
									FieldRef: &game_room.FieldRef{FieldPath: "metadata.name"},
								},
							},
							{
								Name: "env-var-secret-ref",
								ValueFrom: &game_room.ValueFrom{
									SecretKeyRef: &game_room.SecretKeyRef{Name: "secret_name", Key: "secret_key"},
								},
							},
						},
						Requests: game_room.ContainerResources{
							Memory: "100mi",
							CPU:    "100m",
						},
						Limits: game_room.ContainerResources{
							Memory: "200mi",
							CPU:    "200m",
						},
						Ports: []game_room.ContainerPort{{
							Name:     "port-name",
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
			Forwarders: []*forwarder.Forwarder{
				{
					Name:        "forwarder-1",
					Enabled:     true,
					ForwardType: "gRPC",
					Address:     "127.0.0.1:9090",
					Options: &forwarder.ForwardOptions{
						Timeout: time.Duration(1000),
					},
				},
			},
		}

		schedulerStorage.EXPECT().CreateScheduler(gomock.Any(), gomock.Any()).Do(
			func(_ interface{}, arg *entities.Scheduler) {
				assert.Equal(t, scheduler.Name, arg.Name)
				assert.Equal(t, scheduler.Game, arg.Game)
				assert.Equal(t, scheduler.State, arg.State)
				assert.Equal(t, scheduler.MaxSurge, arg.MaxSurge)
				assert.Equal(t, scheduler.RoomsReplicas, arg.RoomsReplicas)
				assert.Equal(t, scheduler.Spec, arg.Spec)
				assert.Equal(t, scheduler.PortRange, arg.PortRange)
				for i, forwarder := range arg.Forwarders {
					assert.Equal(t, scheduler.Forwarders[i].Name, forwarder.Name)
					assert.Equal(t, scheduler.Forwarders[i].Enabled, forwarder.Enabled)
					assert.Equal(t, scheduler.Forwarders[i].Address, forwarder.Address)
					assert.Equal(t, scheduler.Forwarders[i].Options.Timeout, forwarder.Options.Timeout)
				}
			},
		).Return(nil)
		operationManager.EXPECT().CreateOperation(gomock.Any(), scheduler.Name, gomock.Any()).Return(&operation.Operation{ID: "id-1"}, nil)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(scheduler, nil)

		mux := runtime.NewServeMux()
		err = api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		request, err := ioutil.ReadFile(dirPath + "/fixtures/request/scheduler-config.json")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "schedulers_handler/create_scheduler.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with failure", func(t *testing.T) {
		schedulerManager := scheduler_manager.NewSchedulerManager(nil, nil, nil, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		request, err := ioutil.ReadFile(dirPath + "/fixtures/request/bad-scheduler-config.json")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		bodyString := rr.Body.String()
		var body map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &body)
		require.NoError(t, err)

		schedulerMessage, ok := body["message"]
		assert.True(t, ok)
		assert.NotNil(t, schedulerMessage)
		assert.Contains(t, schedulerMessage, "Scheduler.Name: Name is a required field")
		assert.Contains(t, schedulerMessage, "Scheduler.Game: Game is a required field")
		assert.Contains(t, schedulerMessage, "Scheduler.MaxSurge: MaxSurge is a required field")
		assert.Contains(t, schedulerMessage, "Scheduler.RoomsReplicas: RoomsReplicas must be 0 or greater")
		assert.Contains(t, schedulerMessage, "Scheduler.PortRange.Start: Start must be less than End")
		assert.Contains(t, schedulerMessage, "Scheduler.PortRange.Start: Start must be less than End")
		assert.Contains(t, schedulerMessage, "Scheduler.Spec.Containers[0].Environment[0].Name: Name is a required field")
		assert.Contains(t, schedulerMessage, "Scheduler.Spec.TerminationGracePeriod: TerminationGracePeriod must be greater than 0")
		assert.Contains(t, schedulerMessage, "Scheduler.Spec.Containers[0].Name: Name is a required field")
		assert.Contains(t, schedulerMessage, "Scheduler.Spec.Containers[0].Image: Image is a required field")
		assert.Contains(t, schedulerMessage, "Scheduler.Spec.Containers[0].Ports[0].Name: Name must be a maximum of 15 characters in length")
		assert.Contains(t, schedulerMessage, "Scheduler.Spec.Containers[0].Requests.CPU: CPU is a required field")
		assert.Contains(t, schedulerMessage, "Scheduler.Spec.Containers[0].Requests.Memory: Memory is a required field")
		assert.Contains(t, schedulerMessage, "Scheduler.Spec.Containers[0].Ports[0].Protocol: Protocol must be one of the following options: tcp, udp, sctp")
		assert.Contains(t, schedulerMessage, "Scheduler.Spec.Containers[0].ImagePullPolicy: ImagePullPolicy must be one of the following options: Always, Never, IfNotPresent")
		assert.Contains(t, schedulerMessage, "Scheduler.Forwarders[0].Name: Name is a required field")
		assert.Contains(t, schedulerMessage, "Scheduler.Forwarders[0].ForwardType: ForwardType must be one of the following options: gRPC")
		assert.Contains(t, schedulerMessage, "Scheduler.Forwarders[0].Address: Address is a required field")
	})

func TestNewSchedulerVersion(t *testing.T) {
	dirPath, _ := os.Getwd()

	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	t.Run("with success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		currentScheduler := newValidScheduler()
		currentScheduler.PortRange = &entities.PortRange{Start: 1, End: 2}

		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(gomock.Any(), "scheduler-name-1", gomock.Any()).Return(&operation.Operation{ID: "id-1"}, nil)
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(currentScheduler, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		request, err := ioutil.ReadFile(dirPath + "/fixtures/request/scheduler-config.json")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers/scheduler-name-1", bytes.NewReader(request))
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
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil, nil, roomStorage)

		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(nil, errors.NewErrNotFound("err"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		request, err := ioutil.ReadFile(dirPath + "/fixtures/request/scheduler-config.json")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers/scheduler-name-1", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 404, rr.Code)
		require.Contains(t, rr.Body.String(), "no scheduler found, can not create new version for inexistent scheduler: err")
	})

	t.Run("with failure", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		currentScheduler := newValidScheduler()
		currentScheduler.PortRange = &entities.PortRange{Start: 1, End: 2}

		scheduler := newValidScheduler()
		scheduler.PortRange = &entities.PortRange{Start: 0, End: 1}

		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(gomock.Any(), "scheduler-name-1", gomock.Any()).Return(nil, errors.NewErrUnexpected("storage offline"))
		schedulerStorage.EXPECT().GetScheduler(gomock.Any(), "scheduler-name-1").Return(currentScheduler, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		request, err := ioutil.ReadFile(dirPath + "/fixtures/request/scheduler-config.json")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/schedulers/scheduler-name-1", bytes.NewReader(request))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 500, rr.Code)
		require.Contains(t, rr.Body.String(), "failed to schedule create_new_scheduler_version operation")
	})
}

func TestSwitchActiveVersion(t *testing.T) {
	err := validations.RegisterValidations()
	if err != nil {
		t.Errorf("unexpected error %d'", err)
	}

	t.Run("with success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(gomock.Any(), "scheduler-name-1", gomock.Any()).Return(&operation.Operation{ID: "id-1"}, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPut, "/schedulers/scheduler-name-1", bytes.NewReader([]byte("{\"version\": \"v2.0.0\"}")))
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

	t.Run("fails when operation enqueue fails since version does not exist", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		operationManager := mock.NewMockOperationManager(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, schedulerCache, operationManager, roomStorage)

		operationManager.EXPECT().CreateOperation(gomock.Any(), "scheduler-name-1", gomock.Any()).Return(nil, errors.NewErrUnexpected("internal error"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPut, "/schedulers/scheduler-name-1", bytes.NewReader([]byte("{\"version\": \"v2.0.0\"}")))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, 500, rr.Code)
		require.Contains(t, rr.Body.String(), "internal error")
	})
}

func TestGetSchedulersInfo(t *testing.T) {
	t.Run("with valid request and persisted schedulers and game rooms", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		roomStorage := mockports.NewMockRoomStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil, nil, roomStorage)

		scheduler := newValidScheduler()
		scheduler.Autoscaling = &autoscaling.Autoscaling{
			Enabled: true,
			Min:     1,
			Max:     5,
		}
		schedulers := []*entities.Scheduler{scheduler}
		schedulerStorage.EXPECT().GetSchedulersWithFilter(gomock.Any(), gomock.Any()).Return(schedulers, nil)

		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(5, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(10, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(15, nil)
		roomStorage.EXPECT().GetRoomCountByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(20, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)
		game := "tennis-clash"

		url := fmt.Sprintf("/schedulers/info?game=%s", game)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "schedulers_handler/get_schedulers_info.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with valid request and no scheduler and game rooms found", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil, nil, nil)
		schedulerStorage.EXPECT().GetSchedulersWithFilter(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrNotFound("err"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		game := "tennis-clash"
		url := fmt.Sprintf("/schedulers/info?game=%s", game)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "schedulers_handler/get_schedulers_info_not_found.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with unknown error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
		schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil, nil, nil)
		schedulerStorage.EXPECT().GetSchedulersWithFilter(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrUnexpected("exception"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)
		game := "tennis-clash"
		url := fmt.Sprintf("/schedulers/info?game=%s", game)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "schedulers_handler/get_schedulers_info_internal_error.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})
}

func TestPatchScheduler(t *testing.T) {
	type Input struct {
		Request *api.PatchSchedulerRequest
	}

	type Mocks struct {
		RequestFile           string
		GetSchedulerReturn    *entities.Scheduler
		GetSchedulerError     error
		CreateOperationReturn *operation.Operation
		CreateOperationError  error
	}

	type Output struct {
		Response *api.PatchSchedulerResponse
		Status   int
	}

	roomsReplicas := int32(6)
	wrongMaxSurge := "wrong-max-surge"

	testCases := []struct {
		Title string
		Input
		Mocks
		Output
	}{
		{
			Title: "When all is ok",
			Input: Input{
				Request: &api.PatchSchedulerRequest{
					RoomsReplicas: &roomsReplicas,
				},
			},
			Mocks: Mocks{
				RequestFile:        "scheduler-patch.json",
				GetSchedulerReturn: newValidScheduler(),
				GetSchedulerError:  nil,
				CreateOperationReturn: &operation.Operation{
					ID: "some-id",
				},
				CreateOperationError: nil,
			},
			Output: Output{
				Response: &api.PatchSchedulerResponse{
					OperationId: "some-id",
				},
				Status: http.StatusOK,
			},
		},
		{
			Title: "When invalid request payload return 400",
			Input: Input{
				Request: &api.PatchSchedulerRequest{
					RoomsReplicas: &roomsReplicas,
				},
			},
			Mocks: Mocks{
				RequestFile:           "invalid-scheduler-patch.json",
				GetSchedulerReturn:    newValidScheduler(),
				GetSchedulerError:     portsErrors.NewErrNotFound("not found error"),
				CreateOperationReturn: nil,
				CreateOperationError:  nil,
			},
			Output: Output{
				Response: nil,
				Status:   http.StatusBadRequest,
			},
		},
		{
			Title: "When request payload does not change scheduler return 409",
			Input: Input{
				Request: &api.PatchSchedulerRequest{},
			},
			Mocks: Mocks{
				RequestFile:           "empty-scheduler-patch.json",
				GetSchedulerReturn:    newValidScheduler(),
				GetSchedulerError:     nil,
				CreateOperationReturn: nil,
				CreateOperationError:  nil,
			},
			Output: Output{
				Response: nil,
				Status:   http.StatusConflict,
			},
		},
		{
			Title: "When PatchSchedulerAndCreateNewSchedulerVersionOperation return portsErrors.ErrNotFound return 404",
			Input: Input{
				Request: &api.PatchSchedulerRequest{
					RoomsReplicas: &roomsReplicas,
				},
			},
			Mocks: Mocks{
				RequestFile:           "scheduler-patch.json",
				GetSchedulerReturn:    newValidScheduler(),
				GetSchedulerError:     portsErrors.NewErrNotFound("not found error"),
				CreateOperationReturn: nil,
				CreateOperationError:  nil,
			},
			Output: Output{
				Response: nil,
				Status:   http.StatusNotFound,
			},
		},
		{
			Title: "When patch scheduler results in an invalid scheduler return 400",
			Input: Input{
				Request: &api.PatchSchedulerRequest{
					MaxSurge: &wrongMaxSurge,
				},
			},
			Mocks: Mocks{
				RequestFile:           "invalid-scheduler-patch.json",
				GetSchedulerReturn:    newValidScheduler(),
				GetSchedulerError:     nil,
				CreateOperationReturn: nil,
				CreateOperationError:  nil,
			},
			Output: Output{
				Response: nil,
				Status:   http.StatusBadRequest,
			},
		},
		{
			Title: "When PatchSchedulerAndCreateNewSchedulerVersionOperation return portsErrors.ErrUnexpected return 500",
			Input: Input{
				Request: &api.PatchSchedulerRequest{
					RoomsReplicas: &roomsReplicas,
				},
			},
			Mocks: Mocks{
				RequestFile:           "scheduler-patch.json",
				GetSchedulerReturn:    newValidScheduler(),
				GetSchedulerError:     portsErrors.NewErrUnexpected("unexpected error"),
				CreateOperationReturn: nil,
				CreateOperationError:  nil,
			},
			Output: Output{
				Response: nil,
				Status:   http.StatusInternalServerError,
			},
		},
		{
			Title: "When PatchSchedulerAndCreateNewSchedulerVersionOperation return portsErrors.ErrUnexpected return 500",
			Input: Input{
				Request: &api.PatchSchedulerRequest{
					RoomsReplicas: &roomsReplicas,
				},
			},
			Mocks: Mocks{
				RequestFile:           "scheduler-patch.json",
				GetSchedulerReturn:    newValidScheduler(),
				GetSchedulerError:     nil,
				CreateOperationReturn: nil,
				CreateOperationError:  portsErrors.NewErrUnexpected("unexpected error"),
			},
			Output: Output{
				Response: nil,
				Status:   http.StatusInternalServerError,
			},
		},
	}

	err := validations.RegisterValidations()
	require.NoError(t, err)

	dirPath, _ := os.Getwd()
	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			currentScheduler := newValidScheduler()
			currentScheduler.PortRange = &entities.PortRange{Start: 1, End: 2}

			schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
			operationManager := mock.NewMockOperationManager(mockCtrl)
			schedulerManager := scheduler_manager.NewSchedulerManager(schedulerStorage, nil, operationManager, nil)

			schedulerStorage.EXPECT().
				GetScheduler(gomock.Any(), "scheduler-name-1").
				Return(testCase.Mocks.GetSchedulerReturn, testCase.Mocks.GetSchedulerError).
				AnyTimes()

			operationManager.EXPECT().
				CreateOperation(gomock.Any(), "scheduler-name-1", gomock.Any()).
				Return(testCase.Mocks.CreateOperationReturn, testCase.Mocks.CreateOperationError).
				AnyTimes()

			mux := runtime.NewServeMux()
			err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
			require.NoError(t, err)

			url := "/schedulers/scheduler-name-1"

			requestFile := fmt.Sprintf("%s/fixtures/request/%s", dirPath, testCase.Mocks.RequestFile)
			request, err := ioutil.ReadFile(requestFile)
			require.NoError(t, err)

			req, err := http.NewRequest("PATCH", url, bytes.NewReader(request))
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)
			require.Equal(t, testCase.Output.Status, rr.Code)
			if testCase.Output.Status == http.StatusOK {
				responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "schedulers_handler/patch_scheduler_response.json")
				require.Equal(t, expectedResponseBody, responseBody)
			}
		})
	}
}

func TestDeleteScheduler(t *testing.T) {
	t.Run("with valid request and persisted schedulers it returns success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerManager := mockports.NewMockSchedulerManager(mockCtrl)

		scheduler := newValidScheduler()
		scheduler.Autoscaling = &autoscaling.Autoscaling{
			Enabled: true,
			Min:     1,
			Max:     5,
		}

		schedulerManager.EXPECT().EnqueueDeleteSchedulerOperation(gomock.Any(), "scheduler-name-1").
			Return(&operation.Operation{ID: "some-id"}, nil)

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		url := fmt.Sprintf("/schedulers/%s", scheduler.Name)
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "schedulers_handler/delete_scheduler_success.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with valid request and no scheduler it returns not found", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerManager := mockports.NewMockSchedulerManager(mockCtrl)

		scheduler := newValidScheduler()
		scheduler.Autoscaling = &autoscaling.Autoscaling{
			Enabled: true,
			Min:     1,
			Max:     5,
		}

		schedulerManager.EXPECT().EnqueueDeleteSchedulerOperation(gomock.Any(), "scheduler-name-1").
			Return(nil, portsErrors.NewErrNotFound("scheduler not found"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		url := fmt.Sprintf("/schedulers/%s", scheduler.Name)
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "schedulers_handler/delete_scheduler_not_found.json")
		require.Equal(t, expectedResponseBody, responseBody)
	})

	t.Run("with unknown error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)

		schedulerManager := mockports.NewMockSchedulerManager(mockCtrl)

		scheduler := newValidScheduler()
		scheduler.Autoscaling = &autoscaling.Autoscaling{
			Enabled: true,
			Min:     1,
			Max:     5,
		}

		schedulerManager.EXPECT().EnqueueDeleteSchedulerOperation(gomock.Any(), "scheduler-name-1").
			Return(nil, portsErrors.NewErrUnexpected("some-error"))

		mux := runtime.NewServeMux()
		err := api.RegisterSchedulersServiceHandlerServer(context.Background(), mux, ProvideSchedulersHandler(schedulerManager))
		require.NoError(t, err)

		url := fmt.Sprintf("/schedulers/%s", scheduler.Name)
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		responseBody, expectedResponseBody := extractBodyForComparison(t, rr.Body.Bytes(), "schedulers_handler/delete_scheduler_error.json")
		require.Equal(t, expectedResponseBody, responseBody)
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
		RoomsReplicas:   30,
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
					ImagePullPolicy: "IfNotPresent",
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
