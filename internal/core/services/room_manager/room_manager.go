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
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/events"

	"github.com/topfreegames/maestro/internal/core/services/interfaces"

	"go.uber.org/zap"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
)

const (
	minSchedulerMaxSurge            = 1
	schedulerMaxSurgeRelativeSymbol = "%"
)

type RoomManager struct {
	clock           ports.Clock
	portAllocator   ports.PortAllocator
	roomStorage     ports.RoomStorage
	instanceStorage ports.GameRoomInstanceStorage
	runtime         ports.Runtime
	eventsService   interfaces.EventsService
	config          RoomManagerConfig
	logger          *zap.Logger
}

func NewRoomManager(clock ports.Clock, portAllocator ports.PortAllocator, roomStorage ports.RoomStorage, instanceStorage ports.GameRoomInstanceStorage, runtime ports.Runtime, eventsService interfaces.EventsService, config RoomManagerConfig) *RoomManager {
	return &RoomManager{
		clock:           clock,
		portAllocator:   portAllocator,
		roomStorage:     roomStorage,
		instanceStorage: instanceStorage,
		runtime:         runtime,
		eventsService:   eventsService,
		config:          config,
		logger:          zap.L().With(zap.String("service", "rooms_api")),
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
		Version:     scheduler.Spec.Version,
		Status:      game_room.GameStatusPending,
		LastPingAt:  m.clock.Now(),
	}

	err = m.roomStorage.CreateRoom(ctx, room)
	if err != nil {
		return nil, nil, err
	}

	// TODO: let each scheduler parametrize its timeout and use this config as fallback if the scheduler timeout value
	// is absent.
	duration := m.config.RoomInitializationTimeout
	timeoutContext, cancelFunc := context.WithTimeout(ctx, duration)

	err = m.WaitRoomStatus(timeoutContext, room, game_room.GameStatusReady)
	defer cancelFunc()

	if err != nil {
		_ = m.DeleteRoom(ctx, room)
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

	duration := m.config.RoomDeletionTimeout
	timeoutContext, cancelFunc := context.WithTimeout(ctx, duration)
	defer cancelFunc()
	err = m.WaitRoomStatus(timeoutContext, gameRoom, game_room.GameStatusTerminating)
	if err != nil {
		return fmt.Errorf("got timeout while waiting game room status to be terminating: %w", err)
	}

	return nil
}

func (m *RoomManager) UpdateRoom(ctx context.Context, gameRoom *game_room.GameRoom) error {
	gameRoom.LastPingAt = m.clock.Now()

	err := m.roomStorage.UpdateRoom(ctx, gameRoom)
	if err != nil {
		return fmt.Errorf("failed when updating game room in storage with incoming ping data: %w", err)
	}

	err = m.updateGameRoomStatus(ctx, gameRoom.SchedulerID, gameRoom.ID)
	if err != nil {
		return fmt.Errorf("failed to update game room status: %w", err)
	}

	gameRoom.Metadata["eventType"] = events.FromRoomEventTypeToString(events.Ping)
	gameRoom.Metadata["pingType"] = gameRoom.PingStatus.String()
	err = m.eventsService.ProduceEvent(ctx, events.NewRoomEvent(gameRoom.SchedulerID, gameRoom.ID, gameRoom.Metadata))
	if err != nil {
		m.logger.Error(fmt.Sprintf("Failed to forward ping event, error details: %s", err.Error()), zap.Error(err))
		reportPingForwardingFailed(gameRoom.SchedulerID)
	}

	return nil
}

// UpdateRoomInstance updates the instance information.
func (m *RoomManager) UpdateRoomInstance(ctx context.Context, gameRoomInstance *game_room.Instance) error {
	err := m.instanceStorage.UpsertInstance(ctx, gameRoomInstance)
	if err != nil {
		return fmt.Errorf("failed when updating the game room instance on storage: %w", err)
	}

	err = m.updateGameRoomStatus(ctx, gameRoomInstance.SchedulerID, gameRoomInstance.ID)
	if err != nil {
		return fmt.Errorf("failed to update game room status: %w", err)
	}

	return nil
}

// CleanRoomState cleans the remaining state of a room. This function is
// intended to be used after a `DeleteRoom`, where the room instance is
// signaled to terminate.
//
// It wouldn't return an error if the room was already cleaned.
func (m *RoomManager) CleanRoomState(ctx context.Context, schedulerName, roomId string) error {
	err := m.roomStorage.DeleteRoom(ctx, schedulerName, roomId)
	if err != nil && !errors.Is(porterrors.ErrNotFound, err) {
		return fmt.Errorf("failed to delete room state: %w", err)
	}

	err = m.instanceStorage.DeleteInstance(ctx, schedulerName, roomId)
	if err != nil && !errors.Is(porterrors.ErrNotFound, err) {
		return fmt.Errorf("failed to delete room state: %w", err)
	}

	return nil
}

// ListRoomsWithDeletionPriority returns a specified number of rooms, following
// the priority of it being deleted and filtering the ignored version,
// the function will return rooms discarding such filter option.
//
// The priority is:
//
// - On error rooms;
// - No ping received for x time rooms;
// - Pending rooms;
// - Ready rooms;
// - Occupied rooms;
//
// This function can return less rooms than the `amount` since it might not have
// enough rooms on the scheduler.
func (m *RoomManager) ListRoomsWithDeletionPriority(ctx context.Context, schedulerName, ignoredVersion string, amount int, roomsBeingReplaced *sync.Map) ([]*game_room.GameRoom, error) {

	var schedulerRoomsIDs []string
	onErrorRoomIDs, err := m.roomStorage.GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusError)
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms on error: %w", err)
	}

	oldLastPingRoomIDs, err := m.roomStorage.GetRoomIDsByLastPing(ctx, schedulerName, time.Now().Add(m.config.RoomPingTimeout*-1))
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms with old last ping datetime: %w", err)
	}

	pendingRoomIDs, err := m.roomStorage.GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms on pending status: %w", err)
	}

	readyRoomIDs, err := m.roomStorage.GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusReady)
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms on ready status: %w", err)
	}

	occupiedRoomIDs, err := m.roomStorage.GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusOccupied)
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms on occupied status: %w", err)
	}

	schedulerRoomsIDs = append(schedulerRoomsIDs, onErrorRoomIDs...)
	schedulerRoomsIDs = append(schedulerRoomsIDs, oldLastPingRoomIDs...)
	schedulerRoomsIDs = append(schedulerRoomsIDs, pendingRoomIDs...)
	schedulerRoomsIDs = append(schedulerRoomsIDs, readyRoomIDs...)
	schedulerRoomsIDs = append(schedulerRoomsIDs, occupiedRoomIDs...)
	schedulerRoomsIDs = removeDuplicateValues(schedulerRoomsIDs)

	var result []*game_room.GameRoom
	for _, roomID := range schedulerRoomsIDs {
		room, err := m.roomStorage.GetRoom(ctx, schedulerName, roomID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch room information: %w", err)
		}

		_, roomIsBeingReplaced := roomsBeingReplaced.Load(room.ID)

		if roomIsBeingReplaced {
			continue
		}

		if room.Status == game_room.GameStatusTerminating || (ignoredVersion != "" && ignoredVersion == room.Version) {
			continue
		}

		result = append(result, room)
		if len(result) == amount {
			break
		}
	}

	return result, nil
}

// SchedulerMaxSurge calculates the current scheduler max surge based on
// the number of rooms the scheduler has.
func (m *RoomManager) SchedulerMaxSurge(ctx context.Context, scheduler *entities.Scheduler) (int, error) {
	if scheduler.MaxSurge == "" {
		return minSchedulerMaxSurge, nil
	}

	isRelative := strings.HasSuffix(scheduler.MaxSurge, schedulerMaxSurgeRelativeSymbol)
	maxSurgeNum, err := strconv.Atoi(strings.TrimSuffix(scheduler.MaxSurge, schedulerMaxSurgeRelativeSymbol))
	if err != nil {
		return -1, fmt.Errorf("failed to parse max surge into a number: %w", err)
	}

	if !isRelative {
		if minSchedulerMaxSurge > maxSurgeNum {
			return minSchedulerMaxSurge, nil
		}

		return maxSurgeNum, nil
	}

	// TODO(gabriel.corado): should we count terminating and error rooms?
	roomsNum, err := m.roomStorage.GetRoomCount(ctx, scheduler.Name)
	if err != nil {
		return -1, fmt.Errorf("failed to count current number of game rooms: %w", err)
	}

	absoluteNum := math.Round((float64(roomsNum) / 100) * float64(maxSurgeNum))
	return int(math.Max(minSchedulerMaxSurge, absoluteNum)), nil
}

func (m *RoomManager) ValidateGameRoomCreation(ctx context.Context, scheduler *entities.Scheduler) error {
	gameRoom, _, err := m.CreateRoom(ctx, *scheduler)
	if err != nil {
		m.logger.Error("error creating new game room for validating new version")
		return fmt.Errorf("error creating new game room for validating new version: %w", err)
	}
	err = m.DeleteRoom(ctx, gameRoom)
	if err != nil {
		m.logger.Error("error deleting new game room created for validation", zap.Error(err))
	}
	return nil

}
func removeDuplicateValues(slice []string) []string {
	check := make(map[string]int)
	res := make([]string, 0)
	for _, val := range slice {
		if check[val] == 1 {
			continue
		}

		check[val] = 1
		res = append(res, val)
	}

	return res
}
