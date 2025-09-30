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

package rooms

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	serviceerrors "github.com/topfreegames/maestro/internal/core/services/errors"

	"github.com/topfreegames/maestro/internal/core/entities/events"
	"github.com/topfreegames/maestro/internal/core/logs"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

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
	Clock            ports.Clock
	PortAllocator    ports.PortAllocator
	RoomStorage      ports.RoomStorage
	SchedulerStorage ports.SchedulerStorage
	SchedulerCache   ports.SchedulerCache
	InstanceStorage  ports.GameRoomInstanceStorage
	Runtime          ports.Runtime
	EventsService    ports.EventsService
	Config           RoomManagerConfig
	Logger           *zap.Logger
}

var _ ports.RoomManager = (*RoomManager)(nil)

func New(clock ports.Clock, portAllocator ports.PortAllocator, roomStorage ports.RoomStorage, schedulerStorage ports.SchedulerStorage, schedulerCache ports.SchedulerCache, instanceStorage ports.GameRoomInstanceStorage, runtime ports.Runtime, eventsService ports.EventsService, config RoomManagerConfig) ports.RoomManager {
	return &RoomManager{
		Clock:            clock,
		PortAllocator:    portAllocator,
		RoomStorage:      roomStorage,
		SchedulerStorage: schedulerStorage,
		SchedulerCache:   schedulerCache,
		InstanceStorage:  instanceStorage,
		Runtime:          runtime,
		EventsService:    eventsService,
		Config:           config,
		Logger:           zap.L().With(zap.String(logs.LogFieldComponent, "service"), zap.String(logs.LogFieldServiceName, "room_manager")),
	}
}

func (m *RoomManager) CreateRoom(ctx context.Context, scheduler entities.Scheduler, isValidationRoom bool) (*game_room.GameRoom, *game_room.Instance, error) {
	return m.createRoomOnStorageAndRuntime(ctx, &scheduler, isValidationRoom)
}

func (m *RoomManager) GetRoomInstance(ctx context.Context, scheduler, roomID string) (*game_room.Instance, error) {
	instance, err := m.InstanceStorage.GetInstance(ctx, scheduler, roomID)
	if err != nil {
		return nil, fmt.Errorf("error getting instance: %w", err)
	}
	return instance, nil
}

func (m *RoomManager) DeleteRoom(ctx context.Context, gameRoom *game_room.GameRoom, reason string) error {
	instance, err := m.InstanceStorage.GetInstance(ctx, gameRoom.SchedulerID, gameRoom.ID)
	if err != nil {
		if errors.Is(err, porterrors.ErrNotFound) {
			errDeleteRoom := m.RoomStorage.DeleteRoom(ctx, gameRoom.SchedulerID, gameRoom.ID)
			if errDeleteRoom != nil {
				m.Logger.With(zap.Error(errDeleteRoom)).Error("failed to delete room in storage")
			}

			return nil
		}
		return fmt.Errorf("unable to fetch game room instance from storage: %w", err)
	}

	err = m.Runtime.DeleteGameRoomInstance(ctx, instance, reason)
	if err != nil {
		if errors.Is(err, porterrors.ErrNotFound) {
			m.Logger.With(zap.String("instance", instance.ID)).Warn("instance does not exist in runtime")

			errDeleteRoom := m.RoomStorage.DeleteRoom(ctx, gameRoom.SchedulerID, gameRoom.ID)
			if errDeleteRoom != nil {
				m.Logger.With(zap.Error(errDeleteRoom)).Error("failed to delete room in storage")
			}

			errDeleteInstance := m.InstanceStorage.DeleteInstance(ctx, gameRoom.SchedulerID, gameRoom.ID)
			if errDeleteInstance != nil {
				m.Logger.With(zap.Error(errDeleteInstance)).Error("failed to delete instance in storage")
			}

			return nil
		}
		return fmt.Errorf("failed to delete instance on the runtime: %w", err)
	}

	err = m.RoomStorage.UpdateRoomStatus(ctx, gameRoom.SchedulerID, gameRoom.ID, game_room.GameStatusTerminating)
	if err != nil && !errors.Is(err, porterrors.ErrNotFound) {
		return err
	}

	m.forwardStatusTerminatingEvent(ctx, gameRoom)

	return nil
}

func (m *RoomManager) UpdateRoom(ctx context.Context, gameRoom *game_room.GameRoom) error {
	gameRoom.LastPingAt = m.Clock.Now()
	err := m.RoomStorage.UpdateRoom(ctx, gameRoom)
	if err != nil {
		return fmt.Errorf("failed when updating game room in storage with incoming ping data: %w", err)
	}

	shouldForwardEvent, err := m.updateGameRoomStatus(ctx, gameRoom.SchedulerID, gameRoom.ID)
	if err != nil {
		return fmt.Errorf("failed to update game room status: %w", err)
	}

	m.Logger.Debug("Updating room success")

	if shouldForwardEvent {
		gameRoom.Metadata["eventType"] = events.FromRoomEventTypeToString(events.Ping)
		gameRoom.Metadata["pingType"] = gameRoom.PingStatus.String()

		err = m.EventsService.ProduceEvent(ctx, events.NewRoomEvent(gameRoom.SchedulerID, gameRoom.ID, gameRoom.Metadata))
		if err != nil {
			m.Logger.Error(fmt.Sprintf("Failed to forward ping event, error details: %s", err.Error()), zap.Error(err))
			reportPingForwardingFailed(gameRoom.SchedulerID)
		}
	}

	return nil
}

func (m *RoomManager) UpdateRoomInstance(ctx context.Context, gameRoomInstance *game_room.Instance) error {
	if gameRoomInstance == nil {
		return fmt.Errorf("cannot update room instance since it is nil")
	}
	m.Logger.Sugar().Debugf("Updating room instance. ID: %v", gameRoomInstance.ID)
	err := m.InstanceStorage.UpsertInstance(ctx, gameRoomInstance)
	if err != nil {
		return fmt.Errorf("failed when updating the game room instance on storage: %w", err)
	}

	err = m.UpdateGameRoomStatus(ctx, gameRoomInstance.SchedulerID, gameRoomInstance.ID)
	if err != nil {
		return fmt.Errorf("failed to update game room status: %w", err)
	}

	m.Logger.Debug("Updating room instance success")

	return nil
}

func (m *RoomManager) CleanRoomState(ctx context.Context, schedulerName, roomId string) error {
	m.Logger.Sugar().Infof("Cleaning room \"%v\", scheduler \"%v\"", roomId, schedulerName)
	err := m.RoomStorage.DeleteRoom(ctx, schedulerName, roomId)
	if err != nil && !errors.Is(err, porterrors.ErrNotFound) {
		return fmt.Errorf("failed to delete room state: %w", err)
	}

	err = m.InstanceStorage.DeleteInstance(ctx, schedulerName, roomId)
	if err != nil && !errors.Is(err, porterrors.ErrNotFound) {
		return fmt.Errorf("failed to delete room state: %w", err)
	}

	m.forwardStatusTerminatingEvent(ctx, &game_room.GameRoom{
		ID:          roomId,
		SchedulerID: schedulerName,
	})

	m.Logger.Info("cleaning room success")
	return nil
}

func (m *RoomManager) ListRoomsWithDeletionPriority(ctx context.Context, activeScheduler *entities.Scheduler, amount int) ([]*game_room.GameRoom, error) {

	var schedulerRoomsIDs []string
	onErrorRoomIDs, err := m.RoomStorage.GetRoomIDsByStatus(ctx, activeScheduler.Name, game_room.GameStatusError)
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms on error: %w", err)
	}

	oldLastPingRoomIDs, err := m.RoomStorage.GetRoomIDsByLastPing(ctx, activeScheduler.Name, time.Now().Add(m.Config.RoomPingTimeout*-1))
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms with old last ping datetime: %w", err)
	}

	pendingRoomIDs, err := m.RoomStorage.GetRoomIDsByStatus(ctx, activeScheduler.Name, game_room.GameStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms on pending status: %w", err)
	}

	readyRoomIDs, err := m.RoomStorage.GetRoomIDsByStatus(ctx, activeScheduler.Name, game_room.GameStatusReady)
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms on ready status: %w", err)
	}

	occupiedRoomIDs, err := m.RoomStorage.GetRoomIDsByStatus(ctx, activeScheduler.Name, game_room.GameStatusOccupied)
	if err != nil {
		return nil, fmt.Errorf("failed to list scheduler rooms on occupied status: %w", err)
	}

	schedulerRoomsIDs = append(schedulerRoomsIDs, onErrorRoomIDs...)
	schedulerRoomsIDs = append(schedulerRoomsIDs, oldLastPingRoomIDs...)
	schedulerRoomsIDs = append(schedulerRoomsIDs, pendingRoomIDs...)
	schedulerRoomsIDs = append(schedulerRoomsIDs, readyRoomIDs...)
	schedulerRoomsIDs = append(schedulerRoomsIDs, occupiedRoomIDs...)

	schedulerRoomsIDs = removeDuplicateValues(schedulerRoomsIDs)

	var activeVersionRoomPool []*game_room.GameRoom
	var toDeleteRooms []*game_room.GameRoom
	var terminatingRooms []*game_room.GameRoom
	for _, roomID := range schedulerRoomsIDs {
		room, err := m.RoomStorage.GetRoom(ctx, activeScheduler.Name, roomID)
		if err != nil {
			if !errors.Is(err, porterrors.ErrNotFound) {
				return nil, fmt.Errorf("failed to fetch room information: %w", err)
			}

			room = &game_room.GameRoom{ID: roomID, SchedulerID: activeScheduler.Name, Status: game_room.GameStatusError, Version: activeScheduler.Spec.Version}
		}

		// Select Terminating rooms to be re-deleted. This is useful for fixing any desync state.
		if room.Status == game_room.GameStatusTerminating {
			terminatingRooms = append(terminatingRooms, room)
			continue
		}

		isRoomActive := room.Status == game_room.GameStatusOccupied || room.Status == game_room.GameStatusReady || room.Status == game_room.GameStatusPending
		if isRoomActive && activeScheduler.IsSameMajorVersion(room.Version) {
			activeVersionRoomPool = append(activeVersionRoomPool, room)
		} else {
			toDeleteRooms = append(toDeleteRooms, room)
		}
	}
	toDeleteRooms = append(toDeleteRooms, activeVersionRoomPool...)

	m.Logger.Debug("toDeleteRooms",
		zap.Array("toDeleteRooms uncapped", zapcore.ArrayMarshalerFunc(func(enc zapcore.ArrayEncoder) error {
			for _, room := range toDeleteRooms {
				enc.AppendString(fmt.Sprintf("%s-%s-%s", room.ID, room.Version, room.Status.String()))
			}
			return nil
		})),
		zap.Int("amount", amount),
	)

	if len(toDeleteRooms) > amount {
		toDeleteRooms = toDeleteRooms[:amount]
	}

	result := append(toDeleteRooms, terminatingRooms...)

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

func (m *RoomManager) UpdateGameRoomStatus(ctx context.Context, schedulerID, gameRoomId string) error {
	if _, err := m.updateGameRoomStatus(ctx, schedulerID, gameRoomId); err != nil {
		return err
	}

	return nil
}

func (m *RoomManager) updateGameRoomStatus(ctx context.Context, schedulerID, gameRoomId string) (bool, error) {
	scheduler, err := m.getScheduler(ctx, schedulerID)
	if err != nil {
		return false, fmt.Errorf("failed to get scheduler from storage: %w", err)
	}

	gameRoom, err := m.RoomStorage.GetRoom(ctx, scheduler.Name, gameRoomId)
	if err != nil {
		return false, fmt.Errorf("failed to get game room: %w", err)
	}

	instance, err := m.InstanceStorage.GetInstance(ctx, gameRoom.SchedulerID, gameRoomId)
	if err != nil {
		return false, fmt.Errorf("failed to get game room instance: %w", err)
	}

	calculator := NewStatusCalculator(*scheduler, m.Config, m.Logger)
	newStatus, err := calculator.CalculateRoomStatus(*gameRoom, *instance)
	//newStatus, err := gameRoom.RoomComposedStatus(instance.Status.Type)
	if err != nil {
		return false, fmt.Errorf("failed to generate new game room status: %w", err)
	}

	// nothing changed
	if newStatus == gameRoom.Status {
		return true, nil
	}

	if err := gameRoom.ValidateRoomStatusTransition(newStatus); err != nil {
		return false, fmt.Errorf("state transition is invalid: %w", err)
	}

	err = m.RoomStorage.UpdateRoomStatus(ctx, gameRoom.SchedulerID, gameRoomId, newStatus)
	if err != nil {
		if !errors.Is(err, porterrors.ErrNotFound) && instance.Status.Type != game_room.InstanceTerminating {
			return false, fmt.Errorf("failed to update game room status: %w", err)
		}
	}

	if instance.Status.Type == game_room.InstanceTerminating {
		m.forwardStatusTerminatingEvent(ctx, &game_room.GameRoom{
			ID:          gameRoomId,
			SchedulerID: gameRoom.SchedulerID,
		})

		return false, nil // event already sent, prevent to send again
	}

	return true, nil
}

func (m *RoomManager) WaitRoomStatus(ctx context.Context, gameRoom *game_room.GameRoom, status []game_room.GameRoomStatus) (resultStatus game_room.GameRoomStatus, err error) {
	watcher, err := m.RoomStorage.WatchRoomStatus(ctx, gameRoom)
	if err != nil {
		return resultStatus, fmt.Errorf("failed to start room status watcher: %w", err)
	}

	defer watcher.Stop()

	fromStorage, err := m.RoomStorage.GetRoom(ctx, gameRoom.SchedulerID, gameRoom.ID)
	if err != nil {
		return resultStatus, fmt.Errorf("error while retrieving current game room status: %w", err)
	}

	// the room has the desired state already
	if contains(status, fromStorage.Status) {
		return fromStorage.Status, nil
	}

watchLoop:
	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break watchLoop
		case gameRoomEvent := <-watcher.ResultChan():
			if contains(status, gameRoomEvent.Status) {
				resultStatus = gameRoomEvent.Status
				break watchLoop
			}
		}
	}

	if err != nil {
		waitErr := fmt.Errorf("failed to wait until room has desired status: %s, reason: %w", status, err)
		if errors.Is(err, context.DeadlineExceeded) {
			return resultStatus, serviceerrors.NewErrGameRoomStatusWaitingTimeout("").WithError(waitErr)
		}
		return resultStatus, waitErr
	}

	return resultStatus, nil
}

func (m *RoomManager) createRoomOnStorageAndRuntime(ctx context.Context, scheduler *entities.Scheduler, isValidationRoom bool) (*game_room.GameRoom, *game_room.Instance, error) {
	roomName, err := m.Runtime.CreateGameRoomName(ctx, *scheduler)
	if err != nil {
		return nil, nil, err
	}

	room := &game_room.GameRoom{
		ID:               roomName,
		SchedulerID:      scheduler.Name,
		Version:          scheduler.Spec.Version,
		Status:           game_room.GameStatusPending,
		LastPingAt:       m.Clock.Now(),
		IsValidationRoom: isValidationRoom,
	}
	err = m.RoomStorage.CreateRoom(ctx, room)
	if err != nil {
		return nil, nil, err
	}

	spec, err := m.populateSpecWithHostPort(*scheduler)
	if err != nil {
		return nil, nil, err
	}

	instance, err := m.Runtime.CreateGameRoomInstance(ctx, scheduler, roomName, *spec)
	if err != nil {
		deleteRoomErr := m.RoomStorage.DeleteRoom(ctx, scheduler.Name, room.ID)
		if deleteRoomErr != nil {
			return nil, nil, errors.Join(fmt.Errorf("error creating game room and cleaning up room on storage"), err, deleteRoomErr)
		}
		return nil, nil, err
	}

	err = m.InstanceStorage.UpsertInstance(ctx, instance)
	if err != nil {
		return nil, nil, err
	}

	return room, instance, err
}

func (m *RoomManager) populateSpecWithHostPort(scheduler entities.Scheduler) (*game_room.Spec, error) {
	spec := scheduler.Spec.DeepCopy()

	// Backwards compatibility for legacy port range configuration
	if scheduler.PortRange != nil {
		numberOfPorts := 0
		for _, container := range spec.Containers {
			numberOfPorts += len(container.Ports)
		}

		allocatedPorts, err := m.PortAllocator.Allocate(scheduler.PortRange, numberOfPorts)
		if err != nil {
			return nil, err
		}

		portIndex := 0
		for _, container := range spec.Containers {
			for i := range container.Ports {
				container.Ports[i].HostPort = int(allocatedPorts[portIndex])
				portIndex++
			}
		}
	} else { // We should allow each Port to define its own range in order to avoid port conflicts between different protocols
		for _, container := range spec.Containers {
			for i := range container.Ports {
				allocatedPorts, err := m.PortAllocator.Allocate(container.Ports[i].HostPortRange, 1)
				if err != nil {
					return nil, err
				}

				container.Ports[i].HostPort = int(allocatedPorts[0])
			}
		}
	}
	return spec, nil
}

func (m *RoomManager) forwardStatusTerminatingEvent(ctx context.Context, room *game_room.GameRoom) {
	if room.Metadata == nil {
		room.Metadata = map[string]interface{}{}
	}
	room.Metadata["eventType"] = events.FromRoomEventTypeToString(events.Status)
	room.Metadata["pingType"] = game_room.GameRoomPingStatusTerminated.String()
	room.Metadata["roomEvent"] = game_room.GameRoomPingStatusTerminated.String()

	err := m.EventsService.ProduceEvent(ctx, events.NewRoomEvent(room.SchedulerID, room.ID, room.Metadata))
	if err != nil {
		m.Logger.Error("failed to forward terminating room event", zap.String(logs.LogFieldRoomID, room.ID), zap.Error(err))
	}
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

func contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func (m *RoomManager) getScheduler(ctx context.Context, schedulerName string) (*entities.Scheduler, error) {
	scheduler, err := m.SchedulerCache.GetScheduler(ctx, schedulerName)
	if err != nil {
		m.Logger.Warn(fmt.Sprintf("Failed to get scheduler \"%v\" from cache", schedulerName), zap.Error(err))
	}
	if scheduler == nil {
		scheduler, err = m.SchedulerStorage.GetScheduler(ctx, schedulerName)
		if err != nil {
			m.Logger.Error(fmt.Sprintf("Failed to get scheduler \"%v\" from storage", schedulerName), zap.Error(err))
			return nil, err
		}
		if err = m.SchedulerCache.SetScheduler(ctx, scheduler, m.Config.SchedulerCacheTtl); err != nil {
			m.Logger.Warn(fmt.Sprintf("Failed to set scheduler \"%v\" in cache", schedulerName), zap.Error(err))
		}
	}
	return scheduler, nil
}

func (m *RoomManager) AllocateRoom(ctx context.Context, schedulerName string) (string, error) {
	roomId, err := m.RoomStorage.AllocateRoom(ctx, schedulerName)
	if err != nil {
		m.Logger.Error("failed to allocate room from storage",
			zap.String(logs.LogFieldSchedulerName, schedulerName),
			zap.Error(err))
		return "", err
	}

	// Handle descriptive error strings from storage layer
	switch roomId {
	case "NO_ROOMS_AVAILABLE":
		m.Logger.Warn("no ready rooms available for allocation",
			zap.String(logs.LogFieldSchedulerName, schedulerName))
		return "", nil
	case "ALLOCATION_FAILED":
		m.Logger.Warn("room allocation failed due to race condition",
			zap.String(logs.LogFieldSchedulerName, schedulerName))
		return "", nil
	}

	if roomId != "" {
		m.Logger.Debug("room allocated successfully",
			zap.String(logs.LogFieldSchedulerName, schedulerName),
			zap.String(logs.LogFieldRoomID, roomId))
	}

	return roomId, nil
}
