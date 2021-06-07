package room_manager

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"
)

type RoomManager struct {
	clock         ports.Clock
	portAllocator ports.PortAllocator
	roomStorage   ports.RoomStorage
	runtime       ports.Runtime
}

func NewRoomManager(clock ports.Clock, portAllocator ports.PortAllocator, roomStorage ports.RoomStorage, runtime ports.Runtime) *RoomManager {
	return &RoomManager{
		clock:         clock,
		portAllocator: portAllocator,
		roomStorage:   roomStorage,
		runtime:       runtime,
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
	instance, err := m.runtime.CreateGameRoomInstance(ctx, scheduler.ID, scheduler.Spec)
	if err != nil {
		return nil, nil, err
	}

	room := &game_room.GameRoom{
		ID:          instance.ID,
		SchedulerID: scheduler.ID,
		Status:      game_room.GameStatusPending,
		LastPingAt:  m.clock.Now(),
	}

	err = m.roomStorage.CreateRoom(ctx, room)
	if err != nil {
		return nil, nil, err
	}

	return room, instance, err
}
