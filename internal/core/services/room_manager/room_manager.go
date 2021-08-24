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

package room_manager

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"
)

type RoomManager struct {
	clock           ports.Clock
	portAllocator   ports.PortAllocator
	roomStorage     ports.RoomStorage
	instanceStorage ports.GameRoomInstanceStorage
	runtime         ports.Runtime
}

func NewRoomManager(clock ports.Clock, portAllocator ports.PortAllocator, roomStorage ports.RoomStorage, instanceStorage ports.GameRoomInstanceStorage, runtime ports.Runtime) *RoomManager {
	return &RoomManager{
		clock:           clock,
		portAllocator:   portAllocator,
		roomStorage:     roomStorage,
		instanceStorage: instanceStorage,
		runtime:         runtime,
	}
}

func (m *RoomManager) CreateRoom(ctx context.Context, scheduler entities.Scheduler) (*game_room.GameRoom, *game_room.Instance, error) {
	numberOfPorts := 0
	for _, container := range scheduler.Spec.Containers {
		numberOfPorts += len(container.Ports)
	}
	allocatedPorts, err := m.portAllocator.Allocate(scheduler.PortRange, numberOfPorts)
	if err != nil {
		return nil, nil, err
	}
	portIndex := 0
	for _, container := range scheduler.Spec.Containers {
		for i := range container.Ports {
			container.Ports[i].HostPort = int(allocatedPorts[portIndex])
			portIndex++
		}
	}
	instance, err := m.runtime.CreateGameRoomInstance(ctx, scheduler.Name, scheduler.Spec)
	if err != nil {
		return nil, nil, err
	}

	room := &game_room.GameRoom{
		ID:          instance.ID,
		SchedulerID: scheduler.Name,
		Status:      game_room.GameStatusPending,
		LastPingAt:  m.clock.Now(),
	}

	err = m.roomStorage.CreateRoom(ctx, room)
	if err != nil {
		return nil, nil, err
	}

	return room, instance, err
}

func (m *RoomManager) DeleteRoom(ctx context.Context, gameRoom *game_room.GameRoom) error {
	instance, err := m.instanceStorage.GetInstance(ctx, gameRoom.SchedulerID, gameRoom.ID)
	if err != nil {
		// TODO(gabriel.corado): deal better with instance not found.
		return fmt.Errorf("unable to fetch game room instance from storage: %w", err)
	}

	err = m.runtime.DeleteGameRoomInstance(ctx, instance)
	if err != nil {
		// TODO(gabriel.corado): deal better with instance not found.
		return fmt.Errorf("failed to delete instance on the runtime: %w", err)
	}

	newGameRoom := *gameRoom
	newGameRoom.Status = game_room.GameStatusTerminating

	err = m.validateRoomStatusTransition(gameRoom.Status, newGameRoom.Status)
	if err != nil {
		return fmt.Errorf("failed when validating game room status transition: %w", err)
	}

	err = m.roomStorage.UpdateRoom(ctx, &newGameRoom)
	if err != nil {
		return fmt.Errorf("failed when updating game room in storage: %w", err)
	}

	return nil
}

func (m *RoomManager) UpdateRoom(ctx context.Context, gameRoom *game_room.GameRoom) error {
	currentGameRoom, err := m.roomStorage.GetRoom(ctx, gameRoom.SchedulerID, gameRoom.ID)
	if err != nil {
		return fmt.Errorf("unable to fetch game room from storage: %w", err)
	}

	err = m.validateRoomStatusTransition(currentGameRoom.Status, gameRoom.Status)
	if err != nil {
		return fmt.Errorf("failed when validating game room status transition: %w", err)
	}

	err = m.roomStorage.UpdateRoom(ctx, gameRoom)
	if err != nil {
		return fmt.Errorf("failed when updating game room in storage with incoming ping data: %w", err)
	}

	return nil
}
