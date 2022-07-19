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

package healthcontroller_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"

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
	type executionPlan struct {
		planMocks func(
			roomStorage *mockports.MockRoomStorage,
			instanceStorage *ismock.MockGameRoomInstanceStorage,
			schedulerStorage *mockports.MockSchedulerStorage,
			operationManager *mockports.MockOperationManager,
			autoscaler *mockports.MockAutoscaler,
		)
		shouldFail bool
	}

	autoscalingDisabled := autoscaling.Autoscaling{Enabled: false, Min: 1, Max: 10, Policy: autoscaling.Policy{
		Type:       autoscaling.RoomOccupancy,
		Parameters: autoscaling.PolicyParameters{},
	}}

	autoscalingEnabled := autoscaling.Autoscaling{Enabled: true, Min: 1, Max: 10, Policy: autoscaling.Policy{
		Type:       autoscaling.RoomOccupancy,
		Parameters: autoscaling.PolicyParameters{},
	}}

	genericDefinition := &healthcontroller.SchedulerHealthControllerDefinition{}
	genericSchedulerNoAutoscaling := newValidScheduler(nil)
	genericSchedulerAutoscalingDisabled := newValidScheduler(&autoscalingDisabled)
	genericSchedulerAutoscalingEnabled := newValidScheduler(&autoscalingEnabled)

	genericOperation := operation.New(genericSchedulerNoAutoscaling.Name, genericDefinition.Name(), nil)

	testCases := []struct {
		title string
		executionPlan
	}{
		{
			title: "nothing to do, no operations enqueued",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					readyInstance := &game_room.Instance{
						Status: game_room.InstanceStatus{
							Type: game_room.InstanceReady,
						},
					}

					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), genericSchedulerNoAutoscaling.Name).Return([]string{}, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), genericSchedulerNoAutoscaling.Name).Return([]*game_room.Instance{readyInstance}, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), genericSchedulerNoAutoscaling.Name).Return(genericSchedulerNoAutoscaling, nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
				},
			},
		},
		{
			title: "game room status pending with no initialization timeout found, considered available, so nothing to do, no operations enqueued",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1", "existent-pending-2"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						}, {
							ID: "existent-pending-2",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}

					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					gameRoomPending := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusPending,
						LastPingAt:  time.Now(),
						CreatedAt:   time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomPending, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
				},
			},
		},
		{
			title: "game room status pending with initialization timeout found, considered expired, remove room operation enqueued",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1", "existent-pending-2"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						}, {
							ID: "existent-pending-2",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					expiredCreatedAt := time.Now().Add(5 * -time.Minute)
					gameRoomPending := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusPending,
						LastPingAt:  time.Now(),
						CreatedAt:   expiredCreatedAt,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomPending, nil)

					op := operation.New(genericSchedulerNoAutoscaling.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove_rooms.RemoveRoomsDefinition{RoomsIDs: []string{gameRoomIDs[1]}}).Return(op, nil)

					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add_rooms.AddRoomsDefinition{Amount: 1}).Return(op, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
				},
			},
		},
		{
			title: "game room status unready with initialization timeout found, considered expired, remove room operation enqueued",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1", "existent-pending-2"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						}, {
							ID: "existent-pending-2",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					expiredCreatedAt := time.Now().Add(5 * -time.Minute)
					gameRoomUnready := &game_room.GameRoom{
						ID:          gameRoomIDs[1],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusUnready,
						LastPingAt:  time.Now(),
						CreatedAt:   expiredCreatedAt,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomUnready, nil)

					op := operation.New(genericSchedulerNoAutoscaling.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove_rooms.RemoveRoomsDefinition{RoomsIDs: []string{gameRoomIDs[1]}}).Return(op, nil)

					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add_rooms.AddRoomsDefinition{Amount: 1}).Return(op, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
				},
			},
		},
		{
			title: "game room status unready and not expired found, considered available, so nothing to do, no operations enqueued",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1", "existent-unready-2"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						}, {
							ID: "existent-unready-2",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
						CreatedAt:   time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					gameRoomUnready := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusUnready,
						LastPingAt:  time.Now(),
						CreatedAt:   time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomUnready, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
				},
			},
		},
		{
			title: "nonexistent game room IDs found, deletes from storage",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1", "nonexistent-1"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), genericSchedulerNoAutoscaling.Name).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), genericSchedulerNoAutoscaling.Name).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), genericSchedulerNoAutoscaling.Name).Return(genericSchedulerNoAutoscaling, nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 1
					roomStorage.EXPECT().DeleteRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(nil)
				},
			},
		},
		{
			title: "nonexistent game room IDs found but fails on first, keeps trying to delete",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1", "nonexistent-1", "nonexistent-2"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 1
					roomStorage.EXPECT().DeleteRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(errors.New("error"))
					roomStorage.EXPECT().DeleteRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[2]).Return(nil)
				},
			},
		},
		{
			title: "expired room found, enqueue remove rooms with specified ID",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1", "expired-1"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
						{
							ID: "expired-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())

					// Find existent game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					// Find game room
					expiredGameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[1],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now().Add(-time.Minute * 60),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(expiredGameRoom, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 1
					op := operation.New(genericSchedulerNoAutoscaling.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove_rooms.RemoveRoomsDefinition{RoomsIDs: []string{gameRoomIDs[1]}}).Return(op, nil)
				},
			},
		},
		{
			title: "instance is still pending, do nothing",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"expired-1"}
					instances := []*game_room.Instance{
						{
							ID: "expired-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstancePending,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)

					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), "current amount of rooms is equal to desired amount, no changes needed")
				},
			},
		},
		{
			title: "expired room found, enqueue remove rooms with specified ID fails, continues operation and enqueue new add rooms",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1", "expired-1"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
						{
							ID: "expired-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)

					// Find existent game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					// Find game room
					expiredGameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[1],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now().Add(-time.Minute * 60),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(expiredGameRoom, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove_rooms.RemoveRoomsDefinition{RoomsIDs: []string{gameRoomIDs[1]}}).Return(nil, errors.New("error"))

					op := operation.New(genericSchedulerNoAutoscaling.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add_rooms.AddRoomsDefinition{Amount: 1}).Return(op, nil)
				},
			},
		},
		{
			title: "autoscaling not configured, have less available rooms than expected, enqueue add rooms",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					var gameRoomIDs []string
					var instances []*game_room.Instance
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
					op := operation.New(genericSchedulerNoAutoscaling.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add_rooms.AddRoomsDefinition{Amount: 2}).Return(op, nil)
				},
			},
		},
		{
			title: "autoscaling configured but disabled, have less available rooms than expected, enqueue add rooms",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					var gameRoomIDs []string
					var instances []*game_room.Instance
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerAutoscalingDisabled, nil)

					genericSchedulerAutoscalingDisabled.RoomsReplicas = 2
					op := operation.New(genericSchedulerAutoscalingDisabled.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerAutoscalingDisabled.Name, &add_rooms.AddRoomsDefinition{Amount: 2}).Return(op, nil)
				},
			},
		},
		{
			title: "autoscaling configured and enabled, have less available rooms than expected, enqueue add rooms",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					var gameRoomIDs []string
					var instances []*game_room.Instance
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerAutoscalingEnabled, nil)
					autoscaler.EXPECT().CalculateDesiredNumberOfRooms(gomock.Any(), genericSchedulerAutoscalingEnabled).Return(2, nil)

					op := operation.New(genericSchedulerAutoscalingEnabled.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerAutoscalingEnabled.Name, &add_rooms.AddRoomsDefinition{Amount: 2}).Return(op, nil)
				},
			},
		},
		{
			title: "enqueue add rooms fails, finish operation",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					var gameRoomIDs []string
					var instances []*game_room.Instance
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add_rooms.AddRoomsDefinition{Amount: 2}).Return(nil, errors.New("error"))
				},
				shouldFail: true,
			},
		},
		{
			title: "autoscaling not configured, have more available rooms than expected, enqueue remove rooms",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 0
					op := operation.New(genericSchedulerNoAutoscaling.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove_rooms.RemoveRoomsDefinition{Amount: 1}).Return(op, nil)
				},
			},
		},
		{
			title: "autoscaling configured but disabled, have more available rooms than expected, enqueue remove rooms",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerAutoscalingDisabled, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerAutoscalingDisabled.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerAutoscalingDisabled.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericSchedulerAutoscalingDisabled.RoomsReplicas = 0
					op := operation.New(genericSchedulerAutoscalingDisabled.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerAutoscalingDisabled.Name, &remove_rooms.RemoveRoomsDefinition{Amount: 1}).Return(op, nil)
				},
			},
		},
		{
			title: "autoscaling configured and enabled, have more available rooms than expected, enqueue remove rooms",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerAutoscalingEnabled, nil)
					autoscaler.EXPECT().CalculateDesiredNumberOfRooms(gomock.Any(), genericSchedulerAutoscalingEnabled).Return(0, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerAutoscalingEnabled.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerAutoscalingEnabled.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					op := operation.New(genericSchedulerAutoscalingEnabled.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerAutoscalingEnabled.Name, &remove_rooms.RemoveRoomsDefinition{Amount: 1}).Return(op, nil)
				},
			},
		},
		{
			title: "enqueue remove rooms fails, finish operation with error",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 0
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove_rooms.RemoveRoomsDefinition{Amount: 1}).Return(nil, errors.New("error"))
				},
				shouldFail: true,
			},
		},
		{
			title: "fails loading rooms, stops operation",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))
				},
				shouldFail: true,
			},
		},
		{
			title: "fails loading instances, stops operation",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return([]string{}, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))
				},
				shouldFail: true,
			},
		},
		{
			title: "fails loading scheduler, stops operation",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return([]string{}, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return([]*game_room.Instance{}, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))
				},
				shouldFail: true,
			},
		},
		{
			title: "fails calculating the desired number of rooms with autoscaler, stops operation and return error",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerAutoscalingEnabled, nil)
					autoscaler.EXPECT().CalculateDesiredNumberOfRooms(gomock.Any(), genericSchedulerAutoscalingEnabled).Return(0, errors.New("error in autoscaler"))

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerAutoscalingEnabled.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerAutoscalingEnabled.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
				shouldFail: true,
			},
		},
		{
			title: "fails on getRoom, consider room ignored and enqueues new add rooms",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1", "existent-2"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						}, {
							ID: "existent-2",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(nil, errors.New("error"))

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
					op := operation.New(genericSchedulerNoAutoscaling.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add_rooms.AddRoomsDefinition{Amount: 1}).Return(op, nil)
				},
			},
		},
		{
			title: "room with status error, consider room ignored and enqueues new add rooms",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1", "existent-with-error-2"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						}, {
							ID: "existent-with-error-2",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					gameRoomError := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusError,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomError, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
					op := operation.New(genericSchedulerNoAutoscaling.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add_rooms.AddRoomsDefinition{Amount: 1}).Return(op, nil)
				},
			},
		},
		{
			title: "room with status terminating, and considered valid, consider room ignored and enqueues new add rooms",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1", "existent-terminating-2"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						}, {
							ID: "existent-terminating-2",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					gameRoomTerminating := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusTerminating,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomTerminating, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
					op := operation.New(genericSchedulerNoAutoscaling.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add_rooms.AddRoomsDefinition{Amount: 1}).Return(op, nil)
				},
			},
		},
		{
			title: "game room status terminating with initialization timeout found, considered expired, remove room operation enqueued",
			executionPlan: executionPlan{
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					instanceStorage *ismock.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"existent-1", "existent-terminating-2"}
					instances := []*game_room.Instance{
						{
							ID: "existent-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						}, {
							ID: "existent-terminating-2",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerNoAutoscaling, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					expiredCreatedAt := time.Now().Add(5 * -time.Minute)
					gameRoomTerminating := &game_room.GameRoom{
						ID:          gameRoomIDs[1],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusTerminating,
						LastPingAt:  time.Now().Add(5 * -time.Minute),
						CreatedAt:   expiredCreatedAt,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomTerminating, nil)

					op := operation.New(genericSchedulerNoAutoscaling.Name, genericDefinition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove_rooms.RemoveRoomsDefinition{RoomsIDs: []string{gameRoomIDs[1]}}).Return(op, nil)

					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add_rooms.AddRoomsDefinition{Amount: 1}).Return(op, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
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
			autoscaler := mockports.NewMockAutoscaler(mockCtrl)
			config := healthcontroller.Config{
				RoomPingTimeout:           2 * time.Minute,
				RoomInitializationTimeout: 4 * time.Minute,
				RoomDeletionTimeout:       4 * time.Minute,
			}
			executor := healthcontroller.NewExecutor(roomsStorage, instanceStorage, schedulerStorage, operationManager, autoscaler, config)

			testCase.executionPlan.planMocks(roomsStorage, instanceStorage, schedulerStorage, operationManager, autoscaler)

			ctx := context.Background()
			err := executor.Execute(ctx, genericOperation, genericDefinition)
			if testCase.executionPlan.shouldFail {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func newValidScheduler(autoscaling *autoscaling.Autoscaling) *entities.Scheduler {
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
		Forwarders:  forwarders,
		Autoscaling: autoscaling,
	}
}
