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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/filters"
	"github.com/topfreegames/maestro/internal/core/operations/rooms/add"
	"github.com/topfreegames/maestro/internal/core/operations/rooms/remove"
	"go.uber.org/zap"

	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/entities/port"
	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"
	"github.com/topfreegames/maestro/internal/core/ports/mock"
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

	autoscalingEnabled := autoscaling.Autoscaling{Enabled: true, Min: 1, Max: 1000, Cooldown: 60, Policy: autoscaling.Policy{
		Type: autoscaling.RoomOccupancy,
		Parameters: autoscaling.PolicyParameters{
			RoomOccupancy: &autoscaling.RoomOccupancyParams{
				ReadyTarget:   0.5,
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

					// Find existent game room
					gameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[0],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now(),
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[0]).Return(gameRoom, nil).MinTimes(1)

					// Find game room
					expiredGameRoom := &game_room.GameRoom{
						ID:          gameRoomIDs[1],
						SchedulerID: genericSchedulerNoAutoscaling.Name,
						Status:      game_room.GameStatusReady,
						LastPingAt:  time.Now().Add(-time.Minute * 60),
						Version:     genericSchedulerNoAutoscaling.Spec.Version,
					}
					roomStorage.EXPECT().GetRoom(gomock.Any(), genericSchedulerNoAutoscaling.Name, gameRoomIDs[1]).Return(expiredGameRoom, nil).MinTimes(1)
					genericSchedulerNoAutoscaling.RoomsReplicas = 1
					op := operation.New(genericSchedulerNoAutoscaling.Name, definition.Name(), nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any())
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), genericSchedulerNoAutoscaling.Name, &remove.Definition{RoomsIDs: []string{gameRoomIDs[1]}, Reason: remove.Expired}).Return(op, nil)
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
			title:      "do not start rolling update if there are only rooms with a minor difference in version",
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
					// Simulate change in autoscaling, its considered minor
					newScheduler := newValidScheduler(&autoscaling.Autoscaling{
						Enabled:  true,
						Min:      2,
						Max:      1000,
						Cooldown: 60,
						Policy: autoscaling.Policy{
							Type: autoscaling.RoomOccupancy,
							Parameters: autoscaling.PolicyParameters{
								RoomOccupancy: &autoscaling.RoomOccupancyParams{
									ReadyTarget:   0.5,
									DownThreshold: 0.99,
								},
							},
						},
					})
					newScheduler.Spec.Version = "v1.1"
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
						SchedulerID: newScheduler.Name,
						Status:      game_room.GameStatusOccupied,
						LastPingAt:  time.Now(),
						Version:     newScheduler.Spec.Version,
					}

					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)

					// findAvailableAndExpiredRooms
					roomStorage.EXPECT().GetRoom(gomock.Any(), newScheduler.Name, gameRoomIDs[0]).Return(gameRoom1, nil)
					roomStorage.EXPECT().GetRoom(gomock.Any(), newScheduler.Name, gameRoomIDs[1]).Return(gameRoom2, nil)

					// GetDesiredNumberOfRooms
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(newScheduler, nil)
					autoscaler.EXPECT().CalculateDesiredNumberOfRooms(gomock.Any(), newScheduler).Return(2, nil)

					// Check for rolling update
					roomStorage.EXPECT().GetRoom(gomock.Any(), newScheduler.Name, gameRoomIDs[0]).Return(gameRoom1, nil)
					roomStorage.EXPECT().GetRoom(gomock.Any(), newScheduler.Name, gameRoomIDs[1]).Return(gameRoom2, nil)
					schedulerStorage.EXPECT().GetSchedulerWithFilter(gomock.Any(), &filters.SchedulerFilter{
						Name:    genericSchedulerAutoscalingEnabled.Name,
						Version: genericSchedulerAutoscalingEnabled.Spec.Version,
					}).Return(genericSchedulerAutoscalingEnabled, nil)

					_ = operation.New(newScheduler.Name, definition.Name(), nil)

					// Ensure desired amount of instances shouldn't queue new operations as desired == current
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), newScheduler.Name, &add.Definition{Amount: 0}).Times(0)
					roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), newScheduler.Name, game_room.GameStatusOccupied).Times(0)
					roomStorage.EXPECT().GetRoomIDsByStatus(gomock.Any(), newScheduler.Name, game_room.GameStatusReady).Times(0)
				},
			},
		},
		{
			title:      "run a complete rolling update if there are rooms with a major difference in version",
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
					// TerminationGracePeriod field is considered a major change
					newScheduler.Spec.TerminationGracePeriod += 1
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
						Version:     newScheduler.Spec.Version,
					}

					// load
					roomStorage.EXPECT().GetAllRoomIDs(gomock.Any(), gomock.Any()).Return(gameRoomIDs, nil)
					instanceStorage.EXPECT().GetAllInstances(gomock.Any(), gomock.Any()).Return(instances, nil)

					// findAvailableAndExpiredRooms
					roomStorage.EXPECT().GetRoom(gomock.Any(), newScheduler.Name, gameRoomIDs[0]).Return(gameRoom1, nil).MinTimes(1)
					roomStorage.EXPECT().GetRoom(gomock.Any(), newScheduler.Name, gameRoomIDs[1]).Return(gameRoom2, nil).MinTimes(1)

					// GetDesiredNumberOfRooms
					schedulerStorage.EXPECT().GetScheduler(gomock.Any(), gomock.Any()).Return(newScheduler, nil)
					autoscaler.EXPECT().CalculateDesiredNumberOfRooms(gomock.Any(), newScheduler).Return(2, nil)

					// Check for rolling update
					schedulerStorage.EXPECT().GetSchedulerWithFilter(gomock.Any(), &filters.SchedulerFilter{
						Name:    genericSchedulerAutoscalingEnabled.Name,
						Version: genericSchedulerAutoscalingEnabled.Spec.Version,
					}).Return(genericSchedulerAutoscalingEnabled, nil)

					op := operation.New(newScheduler.Name, definition.Name(), nil)

					// Ensure desired amount of instances
					operationManager.EXPECT().CreatePriorityOperation(gomock.Any(), newScheduler.Name, &add.Definition{Amount: 1}).Return(op, nil)
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
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

func TestCompleteRollingUpdate(t *testing.T) {
	// Consider maxSurge 25% and readyTarget 0.5
	readyTarget := 0.5
	autoscalingEnabled := autoscaling.Autoscaling{
		Enabled:  true,
		Min:      1,
		Max:      1000,
		Cooldown: 0,
		Policy: autoscaling.Policy{
			Type: autoscaling.RoomOccupancy,
			Parameters: autoscaling.PolicyParameters{
				RoomOccupancy: &autoscaling.RoomOccupancyParams{
					ReadyTarget:   readyTarget,
					DownThreshold: 0.99,
				},
			},
		},
	}
	schedulerV1 := newValidScheduler(&autoscalingEnabled)
	schedulerV1.MaxSurge = "25%"
	schedulerV2 := newValidScheduler(&autoscalingEnabled)
	schedulerV2.MaxSurge = "25%"
	schedulerV2.Spec.TerminationGracePeriod = schedulerV1.Spec.TerminationGracePeriod + 1
	schedulerV2.Spec.Version = "v2"

	type RollingUpdateExecutionPlan struct {
		currentTotalNumberOfRooms     int
		currentRoomsInActiveVersion   int
		autoscaleDesiredNumberOfRooms int
		desiredNumberOfRoomsWithSurge int
		expectedSurgeAmount           int
		tookAction                    bool
	}

	completeRollingUpdatePlan := map[string][]RollingUpdateExecutionPlan{
		"Occupancy stable during the update": {
			{
				// Add 25 rooms on newer version
				currentTotalNumberOfRooms:     100,
				currentRoomsInActiveVersion:   0,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Remove 25 rooms
				currentTotalNumberOfRooms:     125,
				currentRoomsInActiveVersion:   25,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 25 rooms
				currentTotalNumberOfRooms:     100,
				currentRoomsInActiveVersion:   25,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Remove 25 rooms
				currentTotalNumberOfRooms:     125,
				currentRoomsInActiveVersion:   50,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 25 rooms
				currentTotalNumberOfRooms:     100,
				currentRoomsInActiveVersion:   50,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Remove 25 rooms
				currentTotalNumberOfRooms:     125,
				currentRoomsInActiveVersion:   75,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 25 rooms
				currentTotalNumberOfRooms:     100,
				currentRoomsInActiveVersion:   75,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Remove 25 rooms
				currentTotalNumberOfRooms:     125,
				currentRoomsInActiveVersion:   100,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 25 rooms
				currentTotalNumberOfRooms:     100,
				currentRoomsInActiveVersion:   100,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    false,
			},
		},
		"Occupancy increased during update": {
			{
				// Add 25 rooms on newer version
				currentTotalNumberOfRooms:     100,
				currentRoomsInActiveVersion:   0,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Occupancy changed, autoscale desired to 200, add 125 rooms
				currentTotalNumberOfRooms:     125,
				currentRoomsInActiveVersion:   25,
				autoscaleDesiredNumberOfRooms: 200,
				desiredNumberOfRoomsWithSurge: 250,
				expectedSurgeAmount:           50,
				tookAction:                    true,
			},
			{
				// Remove 50 rooms
				currentTotalNumberOfRooms:     250,
				currentRoomsInActiveVersion:   150,
				autoscaleDesiredNumberOfRooms: 200,
				desiredNumberOfRoomsWithSurge: 250,
				expectedSurgeAmount:           50,
				tookAction:                    true,
			},
			{
				// Occupancy changed, autoscale desired to 400, add 300 rooms
				currentTotalNumberOfRooms:     200,
				currentRoomsInActiveVersion:   150,
				autoscaleDesiredNumberOfRooms: 400,
				desiredNumberOfRoomsWithSurge: 500,
				expectedSurgeAmount:           100,
				tookAction:                    true,
			},
			{
				// Remove 100 rooms
				currentTotalNumberOfRooms:     500,
				currentRoomsInActiveVersion:   450,
				autoscaleDesiredNumberOfRooms: 400,
				desiredNumberOfRoomsWithSurge: 500,
				expectedSurgeAmount:           100,
				tookAction:                    true,
			},
			{
				currentTotalNumberOfRooms:     400,
				currentRoomsInActiveVersion:   400,
				autoscaleDesiredNumberOfRooms: 400,
				desiredNumberOfRoomsWithSurge: 500,
				expectedSurgeAmount:           100,
				tookAction:                    false,
			},
		},
		"Occupancy decreased during update": {
			{
				// Add 25 rooms on newer version
				currentTotalNumberOfRooms:     100,
				currentRoomsInActiveVersion:   0,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Occupancy changed, autoscale desired to 40, remove 85 rooms (15 old)
				currentTotalNumberOfRooms:     125,
				currentRoomsInActiveVersion:   25,
				autoscaleDesiredNumberOfRooms: 40,
				desiredNumberOfRoomsWithSurge: 50,
				expectedSurgeAmount:           10,
				tookAction:                    true,
			},
			{
				// Add 10 rooms
				currentTotalNumberOfRooms:     40,
				currentRoomsInActiveVersion:   25,
				autoscaleDesiredNumberOfRooms: 40,
				desiredNumberOfRoomsWithSurge: 50,
				expectedSurgeAmount:           10,
				tookAction:                    true,
			},
			{
				// Remove 10 rooms
				currentTotalNumberOfRooms:     50,
				currentRoomsInActiveVersion:   35,
				autoscaleDesiredNumberOfRooms: 40,
				desiredNumberOfRoomsWithSurge: 50,
				expectedSurgeAmount:           10,
				tookAction:                    true,
			},
			{
				// Add 10 rooms
				currentTotalNumberOfRooms:     40,
				currentRoomsInActiveVersion:   35,
				autoscaleDesiredNumberOfRooms: 40,
				desiredNumberOfRoomsWithSurge: 50,
				expectedSurgeAmount:           10,
				tookAction:                    true,
			},
			{
				// Remove 5 rooms, only 5 left from old versions, so this is the last update iteration
				currentTotalNumberOfRooms:     50,
				currentRoomsInActiveVersion:   45,
				autoscaleDesiredNumberOfRooms: 40,
				desiredNumberOfRoomsWithSurge: 50,
				expectedSurgeAmount:           10,
				tookAction:                    true,
			},
			{
				// All rooms rolled out
				currentTotalNumberOfRooms:     40,
				currentRoomsInActiveVersion:   40,
				autoscaleDesiredNumberOfRooms: 40,
				desiredNumberOfRoomsWithSurge: 50,
				expectedSurgeAmount:           10,
				tookAction:                    false,
			},
		},
		"Occupancy stable but only 75% of new rooms become active": {
			{
				// Add 25 rooms on newer version, only 19 will become active
				currentTotalNumberOfRooms:     100,
				currentRoomsInActiveVersion:   0,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 6 rooms, only 5 become active
				currentTotalNumberOfRooms:     119,
				currentRoomsInActiveVersion:   19,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 1 rooms, reach limit, start removing on next iteration
				currentTotalNumberOfRooms:     124,
				currentRoomsInActiveVersion:   24,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Remove 25 rooms
				currentTotalNumberOfRooms:     125,
				currentRoomsInActiveVersion:   25,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 25 rooms on newer version, only 19 will become active
				currentTotalNumberOfRooms:     100,
				currentRoomsInActiveVersion:   25,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 6 rooms, only 5 become active
				currentTotalNumberOfRooms:     119,
				currentRoomsInActiveVersion:   44,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 1 rooms, reach limit, start removing on next iteration
				currentTotalNumberOfRooms:     124,
				currentRoomsInActiveVersion:   49,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Remove 25 rooms
				currentTotalNumberOfRooms:     125,
				currentRoomsInActiveVersion:   50,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 25 rooms on newer version, only 19 will become active
				currentTotalNumberOfRooms:     100,
				currentRoomsInActiveVersion:   50,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 6 rooms, only 5 become active
				currentTotalNumberOfRooms:     119,
				currentRoomsInActiveVersion:   69,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 1 rooms, reach limit, start removing on next iteration
				currentTotalNumberOfRooms:     124,
				currentRoomsInActiveVersion:   74,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Remove 25 rooms
				currentTotalNumberOfRooms:     125,
				currentRoomsInActiveVersion:   75,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 25 rooms on newer version, only 19 will become active
				currentTotalNumberOfRooms:     100,
				currentRoomsInActiveVersion:   75,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 6 rooms, only 5 become active
				currentTotalNumberOfRooms:     119,
				currentRoomsInActiveVersion:   94,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Add 1 rooms, reach limit, start removing on next iteration
				currentTotalNumberOfRooms:     124,
				currentRoomsInActiveVersion:   99,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				// Remove 25 rooms
				currentTotalNumberOfRooms:     125,
				currentRoomsInActiveVersion:   100,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    true,
			},
			{
				currentTotalNumberOfRooms:     100,
				currentRoomsInActiveVersion:   100,
				autoscaleDesiredNumberOfRooms: 100,
				desiredNumberOfRoomsWithSurge: 125,
				expectedSurgeAmount:           25,
				tookAction:                    false,
			},
		},
	}

	createRoomMocks := func(roomsStorage *mock.MockRoomStorage, totalRooms, desiredAmount, roomsInOldVersions int) ([]string, []*game_room.Instance, []*game_room.GameRoom) {
		var roomIDs []string
		var instances []*game_room.Instance
		var gameRooms []*game_room.GameRoom
		occupiedAmount := int(float64(desiredAmount) * (1 - readyTarget))
		for i := 0; i < totalRooms; i++ {
			id := fmt.Sprintf("room-%d", i)
			roomIDs = append(roomIDs, id)
			instanceStatus := game_room.InstanceStatus{Type: game_room.InstanceReady}
			grStatus := game_room.GameStatusReady
			if i < occupiedAmount {
				grStatus = game_room.GameStatusOccupied
			}
			roomVersion := schedulerV1.Spec.Version
			if i < roomsInOldVersions {
				roomVersion = "v2"
			}
			instances = append(instances, &game_room.Instance{
				ID:          id,
				Status:      instanceStatus,
				SchedulerID: schedulerV1.Name,
			})
			gameRooms = append(gameRooms, &game_room.GameRoom{
				ID:          id,
				SchedulerID: schedulerV1.Name,
				Version:     roomVersion,
				Status:      grStatus,
				LastPingAt:  time.Now(),
			})
			roomsStorage.EXPECT().GetRoom(gomock.Any(), schedulerV1.Name, id).Return(gameRooms[i], nil).MinTimes(1)
		}
		return roomIDs, instances, gameRooms
	}

	for description, executionPlan := range completeRollingUpdatePlan {
		for id, cycle := range executionPlan {
			t.Run(fmt.Sprintf("%s %d", description, id+1), func(t *testing.T) {
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
				definition := &healthcontroller.Definition{}
				op := operation.New(schedulerV2.Name, definition.Name(), nil)

				roomIDs, instances, gameRooms := createRoomMocks(
					roomsStorage,
					cycle.currentTotalNumberOfRooms,
					cycle.autoscaleDesiredNumberOfRooms,
					cycle.currentRoomsInActiveVersion,
				)

				roomsStorage.EXPECT().GetAllRoomIDs(gomock.Any(), schedulerV2.Name).Return(roomIDs, nil).MinTimes(1)
				instanceStorage.EXPECT().GetAllInstances(gomock.Any(), schedulerV2.Name).Return(instances, nil).MinTimes(1)
				schedulerStorage.EXPECT().GetScheduler(gomock.Any(), schedulerV2.Name).Return(schedulerV2, nil).MinTimes(1)
				autoscaler.EXPECT().CalculateDesiredNumberOfRooms(gomock.Any(), schedulerV2).Return(cycle.autoscaleDesiredNumberOfRooms, nil).MinTimes(1)
				if cycle.tookAction {
					operationManager.EXPECT().AppendOperationEventToExecutionHistory(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
				}

				if cycle.currentRoomsInActiveVersion != cycle.currentTotalNumberOfRooms {
					schedulerStorage.EXPECT().GetSchedulerWithFilter(gomock.Any(), &filters.SchedulerFilter{
						Name:    gameRooms[len(gameRooms)-1].SchedulerID,
						Version: gameRooms[len(gameRooms)-1].Version,
					}).Return(schedulerV1, nil).MinTimes(1)
					if cycle.currentTotalNumberOfRooms < cycle.desiredNumberOfRoomsWithSurge {
						// Add
						operationManager.EXPECT().CreatePriorityOperation(
							gomock.Any(),
							schedulerV2.Name,
							&add.Definition{Amount: int32(cycle.desiredNumberOfRoomsWithSurge - cycle.currentTotalNumberOfRooms)},
						).Return(op, nil)
					} else {
						// Remove
						autoscaler.EXPECT().CanDownscale(gomock.Any(), schedulerV2).Return(true, nil).Times(1)
						schedulerStorage.EXPECT().UpdateScheduler(gomock.Any(), gomock.Any()).Times(1)
						reason := remove.ScaleDown
						if cycle.currentRoomsInActiveVersion < cycle.currentTotalNumberOfRooms {
							reason = remove.RollingUpdateReplace
						}
						removeAmount := int(cycle.currentTotalNumberOfRooms - cycle.autoscaleDesiredNumberOfRooms)
						rommsLeft := cycle.currentTotalNumberOfRooms - cycle.currentRoomsInActiveVersion
						if rommsLeft < removeAmount {
							removeAmount = rommsLeft
						}
						operationManager.EXPECT().CreatePriorityOperation(
							gomock.Any(),
							schedulerV2.Name,
							&remove.Definition{
								Amount: removeAmount,
								Reason: reason,
							},
						).Return(op, nil)
					}
				}
				ctx := context.Background()
				err := executor.Execute(ctx, op, definition)
				assert.Nil(t, err)
				assert.Equal(t, cycle.tookAction, *definition.TookAction)
			})
		}

	}
}

// TODO: Refactor this tests to SchedulerController executor once it's implemented
func TestComputeMaxSurgeVariants(t *testing.T) {
	testCases := []struct {
		name                 string
		maxSurge             string
		desiredNumberOfRooms int
		expectedSurgeAmount  int
		expectError          bool
	}{
		{
			name:                 "Standard surge calculation",
			maxSurge:             "10",
			desiredNumberOfRooms: 20,
			expectedSurgeAmount:  10,
			expectError:          false,
		},
		{
			name:                 "High amount of rooms",
			maxSurge:             "500",
			desiredNumberOfRooms: 1500,
			expectedSurgeAmount:  500,
			expectError:          false,
		},
		{
			name:                 "Relative maxSurge with rooms above 1000",
			maxSurge:             "25%",
			desiredNumberOfRooms: 1200,
			expectedSurgeAmount:  300, // 25% of 1200 is 300
			expectError:          false,
		},
		{
			name:                 "Invalid maxSurge string format",
			maxSurge:             "twenty%", // Invalid because it's not a number
			desiredNumberOfRooms: 300,
			expectedSurgeAmount:  1, // Expected to be 1 because the function should return an error
			expectError:          true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheduler := &entities.Scheduler{
				MaxSurge: tc.maxSurge,
			}

			surgeAmount, err := healthcontroller.ComputeMaxSurge(scheduler, tc.desiredNumberOfRooms)
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

func TestIsRollingUpdate(t *testing.T) {
	autoscalingEnabled := autoscaling.Autoscaling{Enabled: true, Min: 1, Max: 1000, Cooldown: 60, Policy: autoscaling.Policy{
		Type: autoscaling.RoomOccupancy,
		Parameters: autoscaling.PolicyParameters{
			RoomOccupancy: &autoscaling.RoomOccupancyParams{
				ReadyTarget:   0.5,
				DownThreshold: 0.99,
			},
		},
	}}
	schedulerV1 := newValidScheduler(&autoscalingEnabled)
	schedulerV1_1 := newValidScheduler(&autoscalingEnabled)
	schedulerV1_1.RoomsReplicas = schedulerV1.RoomsReplicas + 1
	schedulerV1_1.Spec.Version = "v1.1"
	schedulerV2 := newValidScheduler(&autoscalingEnabled)
	schedulerV2.Spec.TerminationGracePeriod = schedulerV1.Spec.TerminationGracePeriod + 1
	schedulerV2.Spec.Version = "v2"
	schedulerV2_1 := newValidScheduler(&autoscalingEnabled)
	schedulerV2_1.Spec.TerminationGracePeriod = schedulerV2.Spec.TerminationGracePeriod
	schedulerV2_1.RoomsReplicas = schedulerV2.RoomsReplicas + 1
	schedulerV2_1.Spec.Version = "v2.1"
	schedulerMap := map[string]*entities.Scheduler{
		"v1":   schedulerV1,
		"v1.1": schedulerV1_1,
		"v2":   schedulerV2,
		"v2.1": schedulerV2_1,
	}
	testCases := []struct {
		name              string
		activeScheduler   *entities.Scheduler
		availableRoomsIDs []string
		rooms             map[string]*game_room.GameRoom
		expectedReturn    bool
	}{
		{
			name:            "No rooms, return false",
			activeScheduler: schedulerV1,
			rooms:           map[string]*game_room.GameRoom{},
			expectedReturn:  false,
		},
		{
			name:              "All rooms with same version, return false",
			activeScheduler:   schedulerV1,
			availableRoomsIDs: []string{"room1", "room1_also_on_v1"},
			rooms: map[string]*game_room.GameRoom{
				"room1": {
					ID:          "room1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room1_also_on_v1": {
					ID:          "room1_also_on_v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
			},
			expectedReturn: false,
		},
		{
			name:              "Rooms with different minor versions, return false",
			activeScheduler:   schedulerV1_1,
			availableRoomsIDs: []string{"room1", "room1.1"},
			rooms: map[string]*game_room.GameRoom{
				"room1": {
					ID:          "room1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room1.1": {
					ID:          "room1.1",
					SchedulerID: schedulerV1_1.Name,
					Version:     schedulerV1_1.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
			},
			expectedReturn: false,
		},
		{
			name:              "Rooms with different major versions, return true",
			activeScheduler:   schedulerV2,
			availableRoomsIDs: []string{"room1", "room1.1", "room2"},
			rooms: map[string]*game_room.GameRoom{
				"room1": {
					ID:          "room1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room1.1": {
					ID:          "room1.1",
					SchedulerID: schedulerV1_1.Name,
					Version:     schedulerV1_1.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room2": {
					ID:          "room2",
					SchedulerID: schedulerV2.Name,
					Version:     schedulerV2.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
			},
			expectedReturn: true,
		},
		{
			name:              "Rooms with different major and minor versions, return true",
			activeScheduler:   schedulerV2_1,
			availableRoomsIDs: []string{"room1", "room1.1", "room2", "room2.1"},
			rooms: map[string]*game_room.GameRoom{
				"room1": {
					ID:          "room1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room1.1": {
					ID:          "room1.1",
					SchedulerID: schedulerV1_1.Name,
					Version:     schedulerV1_1.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room2": {
					ID:          "room2",
					SchedulerID: schedulerV2.Name,
					Version:     schedulerV2.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room2.1": {
					ID:          "room2.1",
					SchedulerID: schedulerV2_1.Name,
					Version:     schedulerV2_1.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
			},
			expectedReturn: true,
		},
	}

	mockCtrl := gomock.NewController(t)
	roomsStorage := mockports.NewMockRoomStorage(mockCtrl)
	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			for _, id := range tc.availableRoomsIDs {
				room := tc.rooms[id]
				roomsStorage.EXPECT().GetRoom(ctx, tc.activeScheduler.Name, id).Return(room, nil)
				if room.Version != tc.activeScheduler.Spec.Version {
					schedulerStorage.EXPECT().GetSchedulerWithFilter(ctx, &filters.SchedulerFilter{
						Name:    room.SchedulerID,
						Version: room.Version,
					}).Return(schedulerMap[room.Version], nil)
					curMajorVersion := strings.Split(room.Version, ".")[0]
					desiredMajorVersion := strings.Split(tc.activeScheduler.Spec.Version, ".")[0]
					if curMajorVersion != desiredMajorVersion {
						break
					}
				}
			}
			actualReturn := healthcontroller.IsRollingUpdating(
				ctx,
				roomsStorage,
				schedulerStorage,
				zap.NewNop(),
				tc.activeScheduler,
				tc.availableRoomsIDs,
			)
			if actualReturn != tc.expectedReturn {
				t.Errorf("expected IsRollingUpdating to return %v, but got %v", tc.expectedReturn, actualReturn)
			}
		})
	}
}

func TestGetDesiredNumberOfRooms(t *testing.T) {
	autoscalingEnabled := autoscaling.Autoscaling{Enabled: true, Min: 1, Max: 1000, Cooldown: 60, Policy: autoscaling.Policy{
		Type: autoscaling.RoomOccupancy,
		Parameters: autoscaling.PolicyParameters{
			RoomOccupancy: &autoscaling.RoomOccupancyParams{
				ReadyTarget:   0.5,
				DownThreshold: 0.99,
			},
		},
	}}
	autoscalerDisabled := autoscalingEnabled
	autoscalerDisabled.Enabled = false
	schedulerV1 := newValidScheduler(&autoscalingEnabled)
	schedulerV1.MaxSurge = "50%"
	schedulerV2 := newValidScheduler(&autoscalingEnabled)
	schedulerV2.MaxSurge = "50%"
	schedulerV2.Spec.TerminationGracePeriod = schedulerV1.Spec.TerminationGracePeriod + 1
	schedulerV2.Spec.Version = "v2"
	schedulerWithoutAutoScaling := newValidScheduler(&autoscalerDisabled)
	schedulerWithoutAutoScaling.RoomsReplicas = 1
	schedulerWithoutAutoScaling.MaxSurge = "50%"
	schedulerV2WithoutAutoScaling := newValidScheduler(&autoscalerDisabled)
	schedulerV2WithoutAutoScaling.RoomsReplicas = 1
	schedulerV2WithoutAutoScaling.MaxSurge = "50%"
	schedulerV2WithoutAutoScaling.Spec.TerminationGracePeriod = schedulerWithoutAutoScaling.Spec.TerminationGracePeriod + 1
	schedulerV2WithoutAutoScaling.Spec.Version = "v2"
	schedulerInvalidMaxSurge := newValidScheduler(&autoscalingEnabled)
	schedulerInvalidMaxSurge.MaxSurge = "0"
	schedulerInvalidMaxSurge.Spec.TerminationGracePeriod = schedulerV1.Spec.TerminationGracePeriod + 1
	schedulerInvalidMaxSurge.Spec.Version = "v2"
	schedulerMap := map[string]*entities.Scheduler{
		"v1": schedulerV1,
		"v2": schedulerV2,
	}
	testCases := []struct {
		name                string
		desiredByAutoscaler int
		activeScheduler     *entities.Scheduler
		availableRoomsIDs   []string
		rooms               map[string]*game_room.GameRoom
		expectedDesired     int
		expectError         bool
	}{
		{
			name:                "No autoscaling, no rolling update, return rooms replica",
			activeScheduler:     schedulerWithoutAutoScaling,
			availableRoomsIDs:   []string{},
			rooms:               map[string]*game_room.GameRoom{},
			desiredByAutoscaler: schedulerWithoutAutoScaling.RoomsReplicas,
			expectedDesired:     schedulerWithoutAutoScaling.RoomsReplicas,
			expectError:         false,
		},
		{
			name:              "No autoscaling, on rolling update, return rooms replica + maxSurge",
			activeScheduler:   schedulerV2WithoutAutoScaling,
			availableRoomsIDs: []string{"room1v1"},
			rooms: map[string]*game_room.GameRoom{
				"room1v1": {
					ID:          "room1v1",
					SchedulerID: schedulerWithoutAutoScaling.Name,
					Version:     schedulerWithoutAutoScaling.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
			},
			desiredByAutoscaler: schedulerV2WithoutAutoScaling.RoomsReplicas,
			expectedDesired:     schedulerV2WithoutAutoScaling.RoomsReplicas + 1,
			expectError:         false,
		},
		{
			name:              "Autoscaling, no rolling update, return desired by autoscaling",
			activeScheduler:   schedulerV1,
			availableRoomsIDs: []string{"room1v1", "room2v1", "room3v1"},
			rooms: map[string]*game_room.GameRoom{
				"room1v1": {
					ID:          "room1v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
				"room2v1": {
					ID:          "room2v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
				"room3v1": {
					ID:          "room3v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
			},
			desiredByAutoscaler: 6,
			expectedDesired:     6,
			expectError:         false,
		},
		{
			name:              "Autoscaling, rolling update, return desired by autoscaling + maxSurge",
			activeScheduler:   schedulerV2,
			availableRoomsIDs: []string{"room1v1", "room2v1", "room3v1", "room4v1"},
			rooms: map[string]*game_room.GameRoom{
				"room1v1": {
					ID:          "room1v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
				"room2v1": {
					ID:          "room2v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
				"room3v1": {
					ID:          "room3v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
				"room4v1": {
					ID:          "room4v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
			},
			desiredByAutoscaler: 8,
			expectedDesired:     12, // 8 by autoscale + 4 of maxSurge (50%) and current != desired
			expectError:         false,
		},
		{
			name:              "Autoscaling, rolling update, invalid surge use default value of 1, return desired by autoscaling + maxSurge",
			activeScheduler:   schedulerInvalidMaxSurge,
			availableRoomsIDs: []string{"room1v1", "room2v1"},
			rooms: map[string]*game_room.GameRoom{
				"room1v1": {
					ID:          "room1v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
				"room2v1": {
					ID:          "room2v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
			},
			desiredByAutoscaler: 4,
			expectedDesired:     5, // 4 by autoscale + 1 of default maxSurge (set to 0 on spec)
			expectError:         false,
		},
		{
			name:              "Autoscaling, rolling update, current already at the limit, set desired to autoscaling",
			activeScheduler:   schedulerV1,
			availableRoomsIDs: []string{"room1v1", "room2v1", "room3v1", "room4v1", "room5v2", "room6v2", "room7v2", "room8v2", "room9v2", "room10v2", "room11v2", "room12v2"},
			rooms: map[string]*game_room.GameRoom{
				"room1v1": {
					ID:          "room1v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
				"room2v1": {
					ID:          "room2v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
				"room3v1": {
					ID:          "room3v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
				"room4v1": {
					ID:          "room4v1",
					SchedulerID: schedulerV1.Name,
					Version:     schedulerV1.Spec.Version,
					Status:      game_room.GameStatusOccupied,
					LastPingAt:  time.Now(),
				},
				"room5v2": {
					ID:          "room5v2",
					SchedulerID: schedulerV2.Name,
					Version:     schedulerV2.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room6v2": {
					ID:          "room6v2",
					SchedulerID: schedulerV2.Name,
					Version:     schedulerV2.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room7v2": {
					ID:          "room7v2",
					SchedulerID: schedulerV2.Name,
					Version:     schedulerV2.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room8v2": {
					ID:          "room8v2",
					SchedulerID: schedulerV2.Name,
					Version:     schedulerV2.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room9v2": {
					ID:          "room9v2",
					SchedulerID: schedulerV2.Name,
					Version:     schedulerV2.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room10v2": {
					ID:          "room10v2",
					SchedulerID: schedulerV2.Name,
					Version:     schedulerV2.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room11v2": {
					ID:          "room11v2",
					SchedulerID: schedulerV2.Name,
					Version:     schedulerV2.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
				"room12v2": {
					ID:          "room12v2",
					SchedulerID: schedulerV2.Name,
					Version:     schedulerV2.Spec.Version,
					Status:      game_room.GameStatusReady,
					LastPingAt:  time.Now(),
				},
			},
			desiredByAutoscaler: 8,
			expectedDesired:     8, // We are already at upper limit of 12, so set 8 by autoscale
			expectError:         false,
		},
	}

	mockCtrl := gomock.NewController(t)
	roomsStorage := mockports.NewMockRoomStorage(mockCtrl)
	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
	autoscaler := mockports.NewMockAutoscaler(mockCtrl)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Set the expected calls made by IsRollingUpdate
			for _, id := range tc.availableRoomsIDs {
				room := tc.rooms[id]
				roomsStorage.EXPECT().GetRoom(ctx, tc.activeScheduler.Name, id).Return(room, nil)
				if room.Version != tc.activeScheduler.Spec.Version {
					schedulerStorage.EXPECT().GetSchedulerWithFilter(ctx, &filters.SchedulerFilter{
						Name:    room.SchedulerID,
						Version: room.Version,
					}).Return(schedulerMap[room.Version], nil)
					curMajorVersion := strings.Split(room.Version, ".")[0]
					desiredMajorVersion := strings.Split(tc.activeScheduler.Spec.Version, ".")[0]
					if curMajorVersion != desiredMajorVersion {
						break
					}
				}
			}

			if tc.activeScheduler.Autoscaling.Enabled {
				autoscaler.EXPECT().CalculateDesiredNumberOfRooms(ctx, tc.activeScheduler).Return(tc.desiredByAutoscaler, nil)
				if tc.expectedDesired < len(tc.availableRoomsIDs) {
					autoscaler.EXPECT().CanDownscale(ctx, tc.activeScheduler).Return(true, nil)
					schedulerStorage.EXPECT().UpdateScheduler(ctx, tc.activeScheduler).Times(1)
				}
			}

			actualDesiredReturned, _, err := healthcontroller.GetDesiredNumberOfRooms(
				ctx,
				autoscaler,
				roomsStorage,
				schedulerStorage,
				zap.NewNop(),
				tc.activeScheduler,
				tc.availableRoomsIDs,
			)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if actualDesiredReturned != tc.expectedDesired {
					t.Errorf("expected GetDesiredNumberOfRooms to return %d, but got %d", tc.expectedDesired, actualDesiredReturned)
				}
			}
		})
	}
}
