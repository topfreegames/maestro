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

package healthcontroller_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/services/room_manager"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	ismock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations/add_rooms"
	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"
	"github.com/topfreegames/maestro/internal/core/operations/remove_rooms"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
)

func TestSchedulerHealthController_Execute(t *testing.T) {
	type ExecutionPlan struct {
		planMocks func(
			roomStorage *mockports.MockRoomStorage,
			instanceStorage *ismock.MockGameRoomInstanceStorage,
			schedulerStorage *mockports.MockSchedulerStorage,
			operationManager *mockports.MockOperationManager,
		)
		shouldFail bool
	}

	genericDefinition := &healthcontroller.SchedulerHealthControllerDefinition{}
	genericScheduler := newValidScheduler()
	genericOperation := operation.New(genericScheduler.Name, genericDefinition.Name(), nil)

	testCases := []struct {
		title string
		ExecutionPlan
	}{
		{
			title: "nothing to do, no operations enqueued",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), genericScheduler.Name).Return([]string{}, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), genericScheduler.Name).Return([]*game_room.Instance{}, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), genericScheduler.Name).Return(genericScheduler, nil)
				},
			},
		},
		{
			title: "nonexistent game room IDs found, deletes from storage",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					gameRoomIDs := []string{"existent-1", "nonexistent-1"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), genericScheduler.Name).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), genericScheduler.Name).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), genericScheduler.Name).Return(genericScheduler, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericScheduler.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericScheduler.RoomsReplicas = 1
					roomStorage.EXPECT().DeleteRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[1]).Return(nil)
				},
			},
		},
		{
			title: "nonexistent game room IDs found but fails on first, keeps trying to delete",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					gameRoomIDs := []string{"existent-1", "nonexistent-1", "nonexistent-2"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericScheduler, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericScheduler.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericScheduler.RoomsReplicas = 1
					roomStorage.EXPECT().DeleteRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[1]).Return(errors.New("error"))
					roomStorage.EXPECT().DeleteRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[2]).Return(nil)
				},
			},
		},
		{
			title: "expired room found, enqueue remove rooms with specified ID",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					gameRoomIDs := []string{"existent-1", "expired-1"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
						},
						{
							ID: "expired-1",
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericScheduler, nil)

					// Find existent game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericScheduler.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					// Find game room
					expiredGameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[1],
						SchedulerID: genericScheduler.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now().Add(-time.Minute * 60),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[1]).Return(expiredGameRoom, nil)

					genericScheduler.RoomsReplicas = 1
					op := operation.New(genericScheduler.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericScheduler.Name, &remove_rooms.RemoveRoomsDefinition{RoomsIDs: []string{gameRoomIDs[1]}}).Return(op, nil)
				},
			},
		},
		{
			title: "expired room found, enqueue remove rooms with specified ID fails, continues operation and enqueue new add rooms",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					gameRoomIDs := []string{"existent-1", "expired-1"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
						},
						{
							ID: "expired-1",
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericScheduler, nil)

					// Find existent game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericScheduler.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					// Find game room
					expiredGameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[1],
						SchedulerID: genericScheduler.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now().Add(-time.Minute * 60),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[1]).Return(expiredGameRoom, nil)

					genericScheduler.RoomsReplicas = 2
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericScheduler.Name, &remove_rooms.RemoveRoomsDefinition{RoomsIDs: []string{gameRoomIDs[1]}}).Return(nil, errors.New("error"))

					op := operation.New(genericScheduler.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericScheduler.Name, &add_rooms.AddRoomsDefinition{Amount: 1}).Return(op, nil)
				},
			},
		},
		{
			title: "have less available rooms than expected, enqueue add rooms",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					var gameRoomIDs []string
					var instances []*game_room.Instance
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericScheduler, nil)

					genericScheduler.RoomsReplicas = 2
					op := operation.New(genericScheduler.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericScheduler.Name, &add_rooms.AddRoomsDefinition{Amount: 2}).Return(op, nil)
				},
			},
		},
		{
			title: "enqueue add rooms fails, finish operation",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					var gameRoomIDs []string
					var instances []*game_room.Instance
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericScheduler, nil)

					genericScheduler.RoomsReplicas = 2
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericScheduler.Name, &add_rooms.AddRoomsDefinition{Amount: 2}).Return(nil, errors.New("error"))
				},
				shouldFail: true,
			},
		},
		{
			title: "have more available rooms than expected, enqueue remove rooms",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					gameRoomIDs := []string{"existent-1"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericScheduler, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericScheduler.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericScheduler.RoomsReplicas = 0
					op := operation.New(genericScheduler.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericScheduler.Name, &remove_rooms.RemoveRoomsDefinition{Amount: 1}).Return(op, nil)
				},
			},
		},
		{
			title: "enqueue remove rooms fails, finish operation with error",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					gameRoomIDs := []string{"existent-1"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericScheduler, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericScheduler.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericScheduler.RoomsReplicas = 0
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericScheduler.Name, &remove_rooms.RemoveRoomsDefinition{Amount: 1}).Return(nil, errors.New("error"))
				},
				shouldFail: true,
			},
		},
		{
			title: "fails loading rooms, stops operation",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))
				},
				shouldFail: true,
			},
		},
		{
			title: "fails loading instances, stops operation",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return([]string{}, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))
				},
				shouldFail: true,
			},
		},
		{
			title: "fails loading scheduler, stops operation",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return([]string{}, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return([]*game_room.Instance{}, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))
				},
				shouldFail: true,
			},
		},
		{
			title: "fails on getRoom, consider room ignored and enqueues new add rooms",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					gameRoomIDs := []string{"existent-1", "existent-2"}
					instances := []*game_room.Instance{{ID: "existent-1"}, {ID: "existent-2"}}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericScheduler, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericScheduler.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[0]).Return(gameRoom, nil)
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[1]).Return(nil, errors.New("error"))

					genericScheduler.RoomsReplicas = 2
					op := operation.New(genericScheduler.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericScheduler.Name, &add_rooms.AddRoomsDefinition{Amount: 1}).Return(op, nil)
				},
			},
		},
		{
			title: "room with status error, consider room ignored and enqueues new add rooms",
			ExecutionPlan: ExecutionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
				) {
					gameRoomIDs := []string{"existent-1", "existent-with-error-2"}
					instances := []*game_room.Instance{{ID: "existent-1"}, {ID: "existent-with-error-2"}}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericScheduler, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericScheduler.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					gameRoomError := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericScheduler.Name,
						Status:      game_room.GameStatusError,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericScheduler.Name, gameRoomIDs[1]).Return(gameRoomError, nil)

					genericScheduler.RoomsReplicas = 2
					op := operation.New(genericScheduler.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericScheduler.Name, &add_rooms.AddRoomsDefinition{Amount: 1}).Return(op, nil)
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.title, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			roomsStorage := mockports.NewMockRoomStorage(mockCtrl)
			instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
			schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
			operationManager := mockports.NewMockOperationManager(mockCtrl)
			config := room_manager.RoomManagerConfig{
				RoomPingTimeout: 900000,
			}
			executor := healthcontroller.NewExecutor(roomsStorage, instanceStorage, schedulerStorage, operationManager, config)

			testCase.ExecutionPlan.planMocks(roomsStorage, instanceStorage, schedulerStorage, operationManager)

			ctx := context.Background()
			err := executor.Execute(ctx, genericOperation, genericDefinition)
			if testCase.ExecutionPlan.shouldFail {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
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
		RoomsReplicas:   0,
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
