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
	Clock           ports.Clock
	PortAllocator   ports.PortAllocator
	RoomStorage     ports.RoomStorage
	InstanceStorage ports.GameRoomInstanceStorage
	Runtime         ports.Runtime
	EventsService   ports.EventsService
	Config          RoomManagerConfig
	Logger          *zap.Logger
}

var _ ports.RoomManager = (*RoomManager)(nil)

func New(clock ports.Clock, portAllocator ports.PortAllocator, roomStorage ports.RoomStorage, instanceStorage ports.GameRoomInstanceStorage, runtime ports.Runtime, eventsService ports.EventsService, config RoomManagerConfig) ports.RoomManager {
	return &RoomManager{
		Clock:           clock,
		PortAllocator:   portAllocator,
		RoomStorage:     roomStorage,
		InstanceStorage: instanceStorage,
		Runtime:         runtime,
		EventsService:   eventsService,
		Config:          config,
		Logger:          zap.L().With(zap.String("component", "service"), zap.String("service", "room_manager")),
	}
}

func (m *RoomManager) CreateRoomAndWaitForReadiness(ctx context.Context, scheduler entities.Scheduler) (*game_room.GameRoom, *game_room.Instance, error) {
	numberOfPorts := 0
	for _, container := range scheduler.Spec.Containers {
		numberOfPorts += len(container.Ports)
	}
	allocatedPorts, err := m.PortAllocator.Allocate(scheduler.PortRange, numberOfPorts)
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
	instance, err := m.Runtime.CreateGameRoomInstance(ctx, scheduler.Name, scheduler.Spec)
	if err != nil {
		return nil, nil, err
	}

	err = m.InstanceStorage.UpsertInstance(ctx, instance)
	if err != nil {
		return nil, nil, err
	}

	room := &game_room.GameRoom{
		ID:          instance.ID,
		SchedulerID: scheduler.Name,
		Version:     scheduler.Spec.Version,
		Status:      game_room.GameStatusPending,
		LastPingAt:  m.Clock.Now(),
	}
	err = m.RoomStorage.CreateRoom(ctx, room)
	if err != nil {
		return nil, nil, err
	}

	// TODO: let each scheduler parametrize its timeout and use this config as fallback if the scheduler timeout value
	// is absent.
	duration := m.Config.RoomInitializationTimeout
	timeoutContext, cancelFunc := context.WithTimeout(ctx, duration)

	err = m.WaitRoomStatus(timeoutContext, room, game_room.GameStatusReady)
	defer cancelFunc()

	if err != nil {
		_ = m.DeleteRoomAndWaitForRoomTerminated(ctx, room)
		return nil, nil, err
	}

	return room, instance, err
}

func (m *RoomManager) DeleteRoomAndWaitForRoomTerminated(ctx context.Context, gameRoom *game_room.GameRoom) error {
	instance, err := m.InstanceStorage.GetInstance(ctx, gameRoom.SchedulerID, gameRoom.ID)
	if err != nil {
		// TODO(gabriel.corado): deal better with instance not found.
		return fmt.Errorf("unable to fetch game room instance from storage: %w", err)
	}

	err = m.Runtime.DeleteGameRoomInstance(ctx, instance)
	if err != nil {
		// TODO(gabriel.corado): deal better with instance not found.
		return fmt.Errorf("failed to delete instance on the runtime: %w", err)
	}

	duration := m.Config.RoomDeletionTimeout
	timeoutContext, cancelFunc := context.WithTimeout(ctx, duration)
	defer cancelFunc()
	err = m.WaitRoomStatus(timeoutContext, gameRoom, game_room.GameStatusTerminating)
	if err != nil {
		return fmt.Errorf("got timeout while waiting game room status to be terminating: %w", err)
	}

	return nil
}

func (m *RoomManager) UpdateRoom(ctx context.Context, gameRoom *game_room.GameRoom) error {
	gameRoom.LastPingAt = m.Clock.Now()
	err := m.RoomStorage.UpdateRoom(ctx, gameRoom)
	if err != nil {
		return fmt.Errorf("failed when updating game room in storage with incoming ping data: %w", err)
	}

	err = m.UpdateGameRoomStatus(ctx, gameRoom.SchedulerID, gameRoom.ID)
	if err != nil {
		return fmt.Errorf("failed to update game room status: %w", err)
	}

	gameRoom.Metadata["eventType"] = events.FromRoomEventTypeToString(events.Ping)
	gameRoom.Metadata["pingType"] = gameRoom.PingStatus.String()
	err = m.EventsService.ProduceEvent(ctx, events.NewRoomEvent(gameRoom.SchedulerID, gameRoom.ID, gameRoom.Metadata))
	if err != nil {
		m.Logger.Error(fmt.Sprintf("Failed to forward ping event, error details: %s", err.Error()), zap.Error(err))
		reportPingForwardingFailed(gameRoom.SchedulerID)
	}

	return nil
}

func (m *RoomManager) UpdateRoomInstance(ctx context.Context, gameRoomInstance *game_room.Instance) error {
	if gameRoomInstance == nil {
		return fmt.Errorf("cannot update room instance since it is nil")
	}
	m.Logger.Sugar().Infof("Updating room instance. ID: %v", gameRoomInstance.ID)
	err := m.InstanceStorage.UpsertInstance(ctx, gameRoomInstance)
	if err != nil {
		return fmt.Errorf("failed when updating the game room instance on storage: %w", err)
	}

	err = m.UpdateGameRoomStatus(ctx, gameRoomInstance.SchedulerID, gameRoomInstance.ID)
	if err != nil {
		return fmt.Errorf("failed to update game room status: %w", err)
	}

	m.Logger.Info("Updating room success")
	return nil
}

func (m *RoomManager) CleanRoomState(ctx context.Context, schedulerName, roomId string) error {
	m.Logger.Sugar().Infof("Cleaning room \"%v\", scheduler \"%v\"", roomId, schedulerName)
	err := m.RoomStorage.DeleteRoom(ctx, schedulerName, roomId)
	if err != nil && !errors.Is(porterrors.ErrNotFound, err) {
		return fmt.Errorf("failed to delete room state: %w", err)
	}

	err = m.InstanceStorage.DeleteInstance(ctx, schedulerName, roomId)
	if err != nil && !errors.Is(porterrors.ErrNotFound, err) {
		return fmt.Errorf("failed to delete room state: %w", err)
	}

	m.Logger.Info("cleaning room success")
	return nil
}

func (m *RoomManager) ListRoomsWithDeletionPriority(ctx context.Context, schedulerName, ignoredVersion string, amount int, roomsBeingReplaced *sync.Map) ([]*game_room.GameRoom, error) {

	var schedulerRoomsIDs []string
	onErrorRoomIDs, err := m.RoomStorage.GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusError)
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms on error: %w", err)
	}

	oldLastPingRoomIDs, err := m.RoomStorage.GetRoomIDsByLastPing(ctx, schedulerName, time.Now().Add(m.Config.RoomPingTimeout*-1))
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms with old last ping datetime: %w", err)
	}

	pendingRoomIDs, err := m.RoomStorage.GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms on pending status: %w", err)
	}

	readyRoomIDs, err := m.RoomStorage.GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusReady)
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms on ready status: %w", err)
	}

	occupiedRoomIDs, err := m.RoomStorage.GetRoomIDsByStatus(ctx, schedulerName, game_room.GameStatusOccupied)
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
		room, err := m.RoomStorage.GetRoom(ctx, schedulerName, roomID)
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
	roomsNum, err := m.RoomStorage.GetRoomCount(ctx, scheduler.Name)
	if err != nil {
		return -1, fmt.Errorf("failed to count current number of game rooms: %w", err)
	}

	absoluteNum := math.Round((float64(roomsNum) / 100) * float64(maxSurgeNum))
	return int(math.Max(minSchedulerMaxSurge, absoluteNum)), nil
}

func (m *RoomManager) UpdateGameRoomStatus(ctx context.Context, schedulerId, gameRoomId string) error {
	gameRoom, err := m.RoomStorage.GetRoom(ctx, schedulerId, gameRoomId)
	if err != nil {
		return fmt.Errorf("failed to get game room: %w", err)
	}

	instance, err := m.InstanceStorage.GetInstance(ctx, schedulerId, gameRoomId)
	if err != nil {
		return fmt.Errorf("failed to get game room instance: %w", err)
	}

	newStatus, err := gameRoom.RoomComposedStatus(instance.Status.Type)
	if err != nil {
		return fmt.Errorf("failed to generate new game room status: %w", err)
	}

	// nothing changed
	if newStatus == gameRoom.Status {
		return nil
	}

	if err := gameRoom.ValidateRoomStatusTransition(newStatus); err != nil {
		return fmt.Errorf("state transition is invalid: %w", err)
	}

	err = m.RoomStorage.UpdateRoomStatus(ctx, schedulerId, gameRoomId, newStatus)
	if err != nil {
		return fmt.Errorf("failed to update game room status: %w", err)
	}

	return nil
}

func (m *RoomManager) WaitRoomStatus(ctx context.Context, gameRoom *game_room.GameRoom, status game_room.GameRoomStatus) error {
	var err error
	watcher, err := m.RoomStorage.WatchRoomStatus(ctx, gameRoom)
	if err != nil {
		return fmt.Errorf("failed to start room status watcher: %w", err)
	}

	defer watcher.Stop()

	fromStorage, err := m.RoomStorage.GetRoom(ctx, gameRoom.SchedulerID, gameRoom.ID)
	if err != nil {
		return fmt.Errorf("error while retrieving current game room status: %w", err)
	}

	// the room has the desired state already
	if fromStorage.Status == status {
		return nil
	}

watchLoop:
	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break watchLoop
		case gameRoomEvent := <-watcher.ResultChan():
			if gameRoomEvent.Status == status {
				break watchLoop
			}
		}
	}

	if err != nil {
		return fmt.Errorf("failed to wait until room has desired status: %w", err)
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
