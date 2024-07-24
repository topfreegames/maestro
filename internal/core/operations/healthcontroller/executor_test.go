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

	"github.com/topfreegames/maestro/internal/core/operations/rooms/add"
	"github.com/topfreegames/maestro/internal/core/operations/rooms/remove"

	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/entities/port"
	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
)

func TestSchedulerHealthController_Execute(t *testing.T) {
	type executionPlan struct {
		planMocks func(
			roomStorage *mockports.MockRoomStorage,
			roomManager *mockports.MockRoomManager,
			instanceStorage *mockports.MockGameRoomInstanceStorage,
			schedulerStorage *mockports.MockSchedulerStorage,
			operationManager *mockports.MockOperationManager,
			autoscaler *mockports.MockAutoscaler,
		)
		tookAction bool
		shouldFail bool
	}

	autoscalingDisabled := autoscaling.Autoscaling{Enabled: false, Min: 1, Max: 10, Policy: autoscaling.Policy{
		Type:       autoscaling.RoomOccupancy,
		Parameters: autoscaling.PolicyParameters{},
	}}

	autoscalingEnabled := autoscaling.Autoscaling{Enabled: true, Min: 1, Max: 10, Cooldown: 60, Policy: autoscaling.Policy{
		Type: autoscaling.RoomOccupancy,
		Parameters: autoscaling.PolicyParameters{
			RoomOccupancy: &autoscaling.RoomOccupancyParams{
				DownThreshold: 0.99,
			},
		},
	}}

	definition := &healthcontroller.Definition{}
	genericSchedulerNoAutoscaling := newValidScheduler(nil)
	genericSchedulerAutoscalingDisabled := newValidScheduler(&autoscalingDisabled)
	genericSchedulerAutoscalingEnabled := newValidScheduler(&autoscalingEnabled)

	genericOperation := operation.New(genericSchedulerNoAutoscaling.Name, definition.Name(), nil)

	testCases := []struct {
		title      string
		definition *healthcontroller.Definition
		executionPlan
	}{
		{
			title:      "nothing to do, no operations enqueued",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: false,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {

					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), genericSchedulerNoAutoscaling.Name).Return([]string{}, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), genericSchedulerNoAutoscaling.Name).Return([]*game_room.Instance{}, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), genericSchedulerNoAutoscaling.Name).Return(genericSchedulerNoAutoscaling, nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
				},
			},
		},
		{
			title:      "game room status pending with no initialization timeout found, considered available, so nothing to do, no operations enqueued",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: false,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					gameRoomPending := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusPending,
						LastPingAt:  time.Now(),
						CreatedAt:   time.Now(),
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomPending, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomPending, nil)
				},
			},
		},
		{
			title:      "game room status pending with initialization timeout found, considered expired, remove room operation enqueued",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					expiredCreatedAt := time.Now().Add(5 * -time.Minute)
					gameRoomPending := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusPending,
						LastPingAt:  time.Now(),
						CreatedAt:   expiredCreatedAt,
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomPending, nil)

					op := operation.New(genericSchedulerNoAutoscaling.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove.Definition{RoomsIDs: []string{gameRoomIDs[1]}, Reason: remove.Expired}).Return(op, nil)

					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add.Definition{Amount: 1}).Return(op, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "game room status unready with initialization timeout found, considered expired, remove room operation enqueued",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					expiredCreatedAt := time.Now().Add(5 * -time.Minute)
					gameRoomUnready := &game_room.GameRoom{
						ID:          gameRoomIDs[1],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusUnready,
						LastPingAt:  time.Now(),
						CreatedAt:   expiredCreatedAt,
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomUnready, nil)

					op := operation.New(genericSchedulerNoAutoscaling.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove.Definition{RoomsIDs: []string{gameRoomIDs[1]}, Reason: remove.Expired}).Return(op, nil)

					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add.Definition{Amount: 1}).Return(op, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "game room status unready and not expired found, considered available, so nothing to do, no operations enqueued",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: false,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					gameRoomUnready := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusUnready,
						LastPingAt:  time.Now(),
						CreatedAt:   time.Now(),
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomUnready, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomUnready, nil)
				},
			},
		},
		{
			title:      "nonexistent game room IDs found, deletes from game room and instance storage",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: false,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 1
					roomStorage.EXPECT().DeleteRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(nil)
					instanceStorage.EXPECT().DeleteInstance(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(nil)

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "nonexistent game room IDs found but fails on first, keeps trying to delete",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: false,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 1
					roomStorage.EXPECT().DeleteRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(errors.New("error"))
					roomStorage.EXPECT().DeleteRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[2]).Return(nil)
					instanceStorage.EXPECT().DeleteInstance(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(errors.New("error"))
					instanceStorage.EXPECT().DeleteInstance(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[2]).Return(nil)

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "expired room found, enqueue remove rooms with specified ID",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					// Find game room
					expiredGameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[1],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now().Add(-time.Minute * 60),
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(expiredGameRoom, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 1
					op := operation.New(genericSchedulerNoAutoscaling.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove.Definition{RoomsIDs: []string{gameRoomIDs[1]}, Reason: remove.Expired}).Return(op, nil)

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "instance is still pending, do nothing",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: false,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerAutoscalingEnabled.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
						Version:     genericSchedulerAutoscalingEnabled.Spec.Version,
					}
					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(genericSchedulerAutoscalingEnabled, nil)

					// Ensure current scheduler version
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerAutoscalingEnabled.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					autoscaler.EXPECT().CalculateDesiredNumberOfRooms(gomock.Any(), genericSchedulerAutoscalingEnabled).Return(1, nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
				},
			},
		},
		{
			title:      "expired room found, enqueue remove rooms with specified ID fails, continues operation and enqueue new add rooms",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					// Find game room
					expiredGameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[1],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now().Add(-time.Minute * 60),
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(expiredGameRoom, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove.Definition{RoomsIDs: []string{gameRoomIDs[1]}, Reason: remove.Expired}).Return(nil, errors.New("error"))

					op := operation.New(genericSchedulerNoAutoscaling.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add.Definition{Amount: 1}).Return(op, nil)

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "autoscaling not configured, have less available rooms than expected, enqueue add rooms",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
					op := operation.New(genericSchedulerNoAutoscaling.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add.Definition{Amount: 2}).Return(op, nil)

				},
			},
		},
		{
			title:      "autoscaling configured but disabled, have less available rooms than expected, enqueue add rooms",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
					op := operation.New(genericSchedulerAutoscalingDisabled.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerAutoscalingDisabled.Name, &add.Definition{Amount: 2}).Return(op, nil)
				},
			},
		},
		{
			title:      "autoscaling configured and enabled, have less available rooms than expected, enqueue add rooms",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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

					op := operation.New(genericSchedulerAutoscalingEnabled.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerAutoscalingEnabled.Name, &add.Definition{Amount: 2}).Return(op, nil)
				},
			},
		},
		{
			title:      "enqueue add rooms fails, finish operation",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add.Definition{Amount: 2}).Return(nil, errors.New("error"))

				},
				shouldFail: true,
			},
		},
		{
			title:      "autoscaling not configured, have more available rooms than expected, enqueue remove rooms",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
					schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Times(1)
					autoscaler.EXPECT().CanDownscale(gomock.Any(), genericSchedulerNoAutoscaling).Return(true, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 0
					op := operation.New(genericSchedulerNoAutoscaling.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove.Definition{Amount: 1, Reason: remove.ScaleDown}).Return(op, nil)

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "autoscaling configured but disabled, have more available rooms than expected, enqueue remove rooms",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
					schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Times(1)
					autoscaler.EXPECT().CanDownscale(gomock.Any(), genericSchedulerAutoscalingDisabled).Return(true, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerAutoscalingDisabled.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
						Version:     genericSchedulerAutoscalingDisabled.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerAutoscalingDisabled.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericSchedulerAutoscalingDisabled.RoomsReplicas = 0
					op := operation.New(genericSchedulerAutoscalingDisabled.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerAutoscalingDisabled.Name, &remove.Definition{Amount: 1, Reason: remove.ScaleDown}).Return(op, nil)

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerAutoscalingDisabled.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "autoscaling configured and enabled, have more available rooms than expected, enqueue remove rooms",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
					schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Times(1)
					autoscaler.EXPECT().CalculateDesiredNumberOfRooms(gomock.Any(), genericSchedulerAutoscalingEnabled).Return(0, nil)
					autoscaler.EXPECT().CanDownscale(gomock.Any(), genericSchedulerAutoscalingEnabled).Return(true, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerAutoscalingEnabled.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
						Version:     genericSchedulerAutoscalingEnabled.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerAutoscalingEnabled.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					op := operation.New(genericSchedulerAutoscalingEnabled.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerAutoscalingEnabled.Name, &remove.Definition{Amount: 1, Reason: remove.ScaleDown}).Return(op, nil)

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerAutoscalingEnabled.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "autoscaling configured and enabled, have more available rooms than expected, occupation bellow threshold, do not enqueue remove rooms",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: false,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
					autoscaler.EXPECT().CanDownscale(gomock.Any(), genericSchedulerAutoscalingEnabled).Return(false, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerAutoscalingEnabled.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
						Version:     genericSchedulerAutoscalingEnabled.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerAutoscalingEnabled.Name, gameRoomIDs[0]).Return(gameRoom, nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerAutoscalingEnabled.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "enqueue remove rooms fails, finish operation with error",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
					schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Times(1)
					autoscaler.EXPECT().CanDownscale(gomock.Any(), genericSchedulerNoAutoscaling).Return(true, nil)

					// Find game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 0
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove.Definition{Amount: 1, Reason: remove.ScaleDown}).Return(nil, errors.New("error"))

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
				shouldFail: true,
			},
		},
		{
			title:      "fails loading rooms, stops operation",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
			title:      "fails loading instances, stops operation",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
			title:      "fails loading scheduler, stops operation",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
			title:      "fails calculating the desired number of rooms with autoscaler, stops operation and return error",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
						Version:     genericSchedulerAutoscalingEnabled.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerAutoscalingEnabled.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
				shouldFail: true,
			},
		},
		{
			title:      "fails on getRoom, consider room ignored and enqueues new add rooms",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(nil, errors.New("error"))

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
					op := operation.New(genericSchedulerNoAutoscaling.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add.Definition{Amount: 1}).Return(op, nil)

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "room with status error, consider room ignored and enqueues new add rooms",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					gameRoomError := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusError,
						LastPingAt:  time.Now(),
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomError, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
					op := operation.New(genericSchedulerNoAutoscaling.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add.Definition{Amount: 1}).Return(op, nil)

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "room with status terminating, and considered valid, consider room ignored and enqueues new add rooms",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					gameRoomTerminating := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusTerminating,
						LastPingAt:  time.Now(),
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomTerminating, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2
					op := operation.New(genericSchedulerNoAutoscaling.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add.Definition{Amount: 1}).Return(op, nil)

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "game room status terminating with terminating timeout found, considered expired, remove room operation enqueued",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
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
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)

					expiredCreatedAt := time.Now().Add(5 * -time.Minute)
					gameRoomTerminating := &game_room.GameRoom{
						ID:          gameRoomIDs[1],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusTerminating,
						LastPingAt:  time.Now().Add(5 * -time.Minute),
						CreatedAt:   expiredCreatedAt,
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(gameRoomTerminating, nil)

					op := operation.New(genericSchedulerNoAutoscaling.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove.Definition{RoomsIDs: []string{gameRoomIDs[1]}, Reason: remove.Expired}).Return(op, nil)

					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &add.Definition{Amount: 1}).Return(op, nil)

					genericSchedulerNoAutoscaling.RoomsReplicas = 2

					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil)
				},
			},
		},
		{
			title:      "room scheduler version do not match current scheduler, start rolling update and not autoscale",
			definition: &healthcontroller.Definition{},
			executionPlan: executionPlan{
				tookAction: true,
				planMocks: func(
					roomStorage *mockports.MockRoomStorage,
					roomManager *mockports.MockRoomManager,
					instanceStorage *mockports.MockGameRoomInstanceStorage,
					schedulerStorage *mockports.MockSchedulerStorage,
					operationManager *mockports.MockOperationManager,
					autoscaler *mockports.MockAutoscaler,
				) {
					gameRoomIDs := []string{"room-1", "room-2"}
					instances := []*game_room.Instance{
						{
							ID: "room-1",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
						{
							ID: "room-2",
							Status: game_room.InstanceStatus{
								Type: game_room.InstanceReady,
							},
						},
					}
					newScheduler := newValidScheduler(&autoscalingEnabled)
					newScheduler.Spec.Version = "v2"
					gameRoom1 := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerAutoscalingEnabled.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
						CreatedAt:   time.Now(),
						Version:     genericSchedulerAutoscalingEnabled.Spec.Version,
					}
					gameRoom2 := &game_room.GameRoom{
						ID:          gameRoomIDs[1],
						SchedulerID: genericSchedulerAutoscalingEnabled.Name,
						Status:      game_room.GameStatusOccupied,
						LastPingAt:  time.Now(),
						Version:     genericSchedulerAutoscalingEnabled.Spec.Version,
					}

					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)

					// findAvailableAndExpiredRooms
					roomStorage.EXPECT().GetRoom(gomock.Any(), newScheduler.Name, gameRoomIDs[0]).Return(gameRoom1, nil)
					roomStorage.EXPECT().GetRoom(gomock.Any(), newScheduler.Name, gameRoomIDs[1]).Return(gameRoom2, nil)

					// getDesiredNumberOfRooms
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(newScheduler, nil)
					autoscaler.EXPECT().CalculateDesiredNumberOfRooms(gomock.Any(), newScheduler).Return(2, nil)

					// Check for rolling update
					roomStorage.EXPECT().GetRoom(gomock.Any(), newScheduler.Name, gameRoomIDs[0]).Return(gameRoom1, nil)
					roomStorage.EXPECT().GetRoom(gomock.Any(), newScheduler.Name, gameRoomIDs[1]).Return(gameRoom2, nil)

					op := operation.New(newScheduler.Name, definition.Name(), nil)

					// Perform rolling update
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), newScheduler.Name, &add.Definition{Amount: 1}).Return(op, nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
					roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), newScheduler.Name, game_room.GameStatusOccupied).Return(gameRoomIDs, nil)
					roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), newScheduler.Name, game_room.GameStatusReady).Return(gameRoomIDs, nil)
					operationManager.EXPECT().CreateOperation(gomock.Any(), newScheduler.Name, &remove.Definition{RoomsIDs: gameRoomIDs, Reason: remove.RollingUpdateReplace}).Return(op, nil)

					// Shouldn't call autoscale
					schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Times(0)
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.title, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			roomsStorage := mockports.NewMockRoomStorage(mockCtrl)
			roomManager := mockports.NewMockRoomManager(mockCtrl)
			instanceStorage := mockports.NewMockGameRoomInstanceStorage(mockCtrl)
			schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
			operationManager := mockports.NewMockOperationManager(mockCtrl)
			autoscaler := mockports.NewMockAutoscaler(mockCtrl)
			config := healthcontroller.Config{
				RoomPingTimeout:           2 * time.Minute,
				RoomInitializationTimeout: 4 * time.Minute,
				RoomDeletionTimeout:       4 * time.Minute,
			}
			executor := healthcontroller.NewExecutor(roomsStorage, roomManager, instanceStorage, schedulerStorage, operationManager, autoscaler, config)

			testCase.executionPlan.planMocks(roomsStorage, roomManager, instanceStorage, schedulerStorage, operationManager, autoscaler)

			ctx := context.Background()
			err := executor.Execute(ctx, genericOperation, testCase.definition)
			if testCase.executionPlan.shouldFail {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, testCase.executionPlan.tookAction, *testCase.definition.TookAction)
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
		PortRange: &port.PortRange{
			Start: 40000,
			End:   60000,
		},
		Forwarders:  forwarders,
		Autoscaling: autoscaling,
	}
}

func TestComputeRollingSurgeVariants(t *testing.T) {
	testCases := []struct {
		name                 string
		maxSurge             string
		desiredNumberOfRooms int
		totalRoomsAmount     int
		expectedSurgeAmount  int
		expectError          bool
	}{
		{
			name:                 "Standard surge calculation",
			maxSurge:             "10",
			desiredNumberOfRooms: 20,
			totalRoomsAmount:     15,
			expectedSurgeAmount:  10,
			expectError:          false,
		},
		{
			name:                 "High amount of rooms above 1000",
			maxSurge:             "30%",
			desiredNumberOfRooms: 1500,
			totalRoomsAmount:     1000,
			expectedSurgeAmount:  450, // It can surge up to 950 but we cap to the maxSurge of 450
			expectError:          false,
		},
		{
			name:                 "Relative maxSurge with rooms above 1000",
			maxSurge:             "25%",
			desiredNumberOfRooms: 1200,
			totalRoomsAmount:     1000,
			expectedSurgeAmount:  300, // 25% of 1200 is 300
			expectError:          false,
		},
		{
			name:                 "Invalid maxSurge string format",
			maxSurge:             "twenty%", // Invalid because it's not a number
			desiredNumberOfRooms: 300,
			totalRoomsAmount:     250,
			expectedSurgeAmount:  1, // Expected to be 1 because the function should return an error
			expectError:          true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheduler := &entities.Scheduler{
				MaxSurge: tc.maxSurge,
			}

			surgeAmount, err := healthcontroller.ComputeRollingSurge(scheduler, tc.desiredNumberOfRooms, tc.totalRoomsAmount)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if surgeAmount != tc.expectedSurgeAmount {
					t.Errorf("expected surge amount to be %d, but got %d", tc.expectedSurgeAmount, surgeAmount)
				}
			}
		})
	}
}
