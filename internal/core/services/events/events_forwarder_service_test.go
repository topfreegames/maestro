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

package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	eventsservice "github.com/topfreegames/maestro/internal/core/services/events"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/stretchr/testify/require"

	"github.com/golang/mock/gomock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/events"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	mockports "github.com/topfreegames/maestro/internal/core/ports/mock"
)

func TestEventsForwarderService_ProduceEvent(t *testing.T) {
	forwarderEnabled := &forwarder.Forwarder{
		Name:        "fwd",
		Enabled:     true,
		ForwardType: forwarder.TypeGrpc,
		Address:     "address",
		Options: &forwarder.ForwardOptions{
			Timeout:  time.Second * 5,
			Metadata: nil,
		},
	}
	forwarderDisabled := &forwarder.Forwarder{
		Name:        "fwd",
		Enabled:     false,
		ForwardType: forwarder.TypeGrpc,
		Address:     "address",
		Options: &forwarder.ForwardOptions{
			Timeout:  time.Second * 5,
			Metadata: nil,
		},
	}
	forwarders := []*forwarder.Forwarder{forwarderEnabled}

	expectedScheduler := &entities.Scheduler{
		Name:            "scheduler",
		Game:            "game",
		State:           "",
		RollbackVersion: "",
		Spec:            game_room.Spec{},
		PortRange:       nil,
		CreatedAt:       time.Time{},
		MaxSurge:        "",
		Forwarders:      forwarders,
	}

	port := game_room.Port{
		Name:     "clientPort",
		Port:     8080,
		Protocol: "TCP",
	}
	port2 := game_room.Port{
		Name:     "notClientPort",
		Port:     8081,
		Protocol: "TCP",
	}
	ports := []game_room.Port{port, port2}
	expectedGameRoomInstance := &game_room.Instance{
		ID:          "instance",
		SchedulerID: "scheduler",
		Address: &game_room.Address{
			Host:  "host",
			Ports: ports,
		},
	}

	t.Run("should succeed when scheduler do not have forwarders configured", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, _, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.PlayerEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "playerLeft",
				"playerId":  "player",
			},
		}

		scheduler := &entities.Scheduler{
			Name:            "scheduler",
			Game:            "game",
			State:           "",
			RollbackVersion: "",
			Spec:            game_room.Spec{},
			PortRange:       nil,
			CreatedAt:       time.Time{},
			MaxSurge:        "",
			Forwarders:      nil,
		}

		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(scheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil).Times(0)
		schedulerCache.EXPECT().SetScheduler(context.Background(), scheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardPlayerEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil).Times(0)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
	})

	t.Run("should succeed when scheduler have forwarder configured but disabled", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, _, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.PlayerEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "playerLeft",
				"playerId":  "player",
			},
		}

		scheduler := &entities.Scheduler{
			Name:            "scheduler",
			Game:            "game",
			State:           "",
			RollbackVersion: "",
			Spec:            game_room.Spec{},
			PortRange:       nil,
			CreatedAt:       time.Time{},
			MaxSurge:        "",
			Forwarders:      []*forwarder.Forwarder{forwarderDisabled},
		}

		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(scheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil).Times(0)
		schedulerCache.EXPECT().SetScheduler(context.Background(), scheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardPlayerEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil).Times(0)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
	})

	t.Run("should succeed when event is PlayerEvent", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.PlayerEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "playerLeft",
				"playerId":  "player",
			},
		}

		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: false,
		}

		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil).Times(0)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardPlayerEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil)
		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)
		roomStorage.EXPECT().UpdateRoom(gomock.Any(), gomock.Any()).Return(nil)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
		require.Empty(t, event.Attributes["ports"])
	})

	t.Run("should increase player count for PlayerEvent", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.PlayerEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "playerJoin",
				"playerId":  "player",
			},
		}

		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: false,
			Metadata: map[string]interface{}{
				"players": 5,
			},
		}

		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil).Times(0)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardPlayerEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil)
		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)

		room.Metadata["players"] = 6
		roomStorage.EXPECT().UpdateRoom(gomock.Any(), room).Return(nil)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
		require.Empty(t, event.Attributes["ports"])
	})

	t.Run("should decrease player count for PlayerEvent", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.PlayerEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "playerLeft",
				"playerId":  "player",
			},
		}

		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: true,
		}

		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil).Times(0)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardPlayerEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil)
		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)
		roomStorage.EXPECT().UpdateRoom(gomock.Any(), room).MaxTimes(0)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
		require.Empty(t, event.Attributes["ports"])
	})

	t.Run("should ignore player state update for PlayerEvent", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.PlayerEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "playerLeft",
				"playerId":  "player",
			},
		}

		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: false,
			Metadata: map[string]interface{}{
				"players": 5,
			},
		}

		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil).Times(0)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardPlayerEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil)
		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)

		room.Metadata["players"] = 4
		roomStorage.EXPECT().UpdateRoom(gomock.Any(), room).Return(nil)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
		require.Empty(t, event.Attributes["ports"])
	})

	t.Run("should succeed when event is RoomEvent", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "resync",
				"pingType":  "ready",
			},
		}
		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: false,
		}

		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil)
		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardRoomEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
		require.NotEmpty(t, event.Attributes["ports"])

		portsUnmarshalled := make([]map[string]interface{}, 0)
		err = json.Unmarshal([]byte(event.Attributes["ports"].(string)), &portsUnmarshalled)
		require.NoError(t, err)

		for _, _port := range portsUnmarshalled {
			require.NotNil(t, _port["name"])
			require.NotNil(t, _port["protocol"])
			require.NotNil(t, _port["port"])
		}
	})

	t.Run("should succeed when event is RoomEvent of type status and capital ping type", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "status",
				"pingType":  "Ready",
			},
		}
		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: false,
		}
		roomStatusReady := events.RoomStatusReady

		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil)
		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardRoomEvent(context.Background(), events.RoomEventAttributes{
			Game:           "game",
			RoomId:         "room",
			Host:           "host",
			Port:           8080,
			EventType:      "status",
			RoomStatusType: &roomStatusReady,
			Other: map[string]interface{}{
				"eventType": "status",
				"pingType":  "Ready",
				"ports":     `[{"name":"clientPort","port":8080,"protocol":"TCP"},{"name":"notClientPort","port":8081,"protocol":"TCP"}]`,
			},
		}, gomock.Any()).Return(nil)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
		require.NotEmpty(t, event.Attributes["ports"])

		portsUnmarshalled := make([]map[string]interface{}, 0)
		err = json.Unmarshal([]byte(event.Attributes["ports"].(string)), &portsUnmarshalled)
		require.NoError(t, err)

		for _, _port := range portsUnmarshalled {
			require.NotNil(t, _port["name"])
			require.NotNil(t, _port["protocol"])
			require.NotNil(t, _port["port"])
		}
	})

	t.Run("should succeed when event is RoomEvent and game room status is unreliable", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "roomEvent",
				"roomEvent": "terminating",
			},
		}

		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(nil, errors.New("room not found"))
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(nil, errors.New("instance not found"))
		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardRoomEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
		require.NotEmpty(t, event.Attributes["ports"])

		portsUnmarshalled := make([]map[string]interface{}, 0)
		err = json.Unmarshal([]byte(event.Attributes["ports"].(string)), &portsUnmarshalled)
		require.NoError(t, err)

		for _, _port := range portsUnmarshalled {
			require.NotNil(t, _port["name"])
			require.NotNil(t, _port["protocol"])
			require.NotNil(t, _port["port"])
		}
	})

	t.Run("should succeed when event is RoomEvent and ping status is unreliable", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "resync",
				"pingType":  "terminating",
			},
		}

		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(nil, errors.New("room not found"))
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(nil, errors.New("instance not found"))
		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardRoomEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
		require.NotEmpty(t, event.Attributes["ports"])

		portsUnmarshalled := make([]map[string]interface{}, 0)
		err = json.Unmarshal([]byte(event.Attributes["ports"].(string)), &portsUnmarshalled)
		require.NoError(t, err)

		for _, _port := range portsUnmarshalled {
			require.NotNil(t, _port["name"])
			require.NotNil(t, _port["protocol"])
			require.NotNil(t, _port["port"])
		}
	})

	t.Run("should succeed but not produce event when room type is validation and room status is unreliable", func(t *testing.T) {
		eventsForwarderService, config, _, schedulerStorage, roomStorage, _, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "roomEvent",
				"roomEvent": "terminating",
			},
		}

		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: true,
		}

		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
	})

	t.Run("should fail when event type is not in eventAttributes", func(t *testing.T) {
		eventsForwarderService, _, _, _, _, _, _ := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes:  map[string]interface{}{},
		}

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("should fail when event is RoomEvent but address has no ports", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "resync",
				"pingType":  "ready",
			},
		}

		instance := &game_room.Instance{
			ID:          "instance",
			SchedulerID: "scheduler",
			Address: &game_room.Address{
				Host:  "host",
				Ports: nil,
			},
		}
		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: false,
		}

		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)
		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(instance, nil)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardRoomEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil).Times(0)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("should fail when event is PlayerEvent but playerId is not found", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, _, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.PlayerEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "playerLeft",
			},
		}

		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil).Times(0)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardPlayerEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil).Times(0)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("should fail when event is RoomEvent, type is ping but pingType is not found", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "resync",
			},
		}
		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: false,
		}

		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)
		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardRoomEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil).Times(0)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("should fail when scheduler not found", func(t *testing.T) {
		eventsForwarderService, _, eventsForwarder, schedulerStorage, _, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "resync",
			},
		}

		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, errors.New("scheduler not found"))
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil).Times(0)
		eventsForwarder.EXPECT().ForwardRoomEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil).Times(0)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("should fail when instance needed but not found", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "resync",
			},
		}

		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: false,
		}

		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(nil, errors.New("error"))
		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardRoomEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil).Times(0)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("should fail when forwardRoomEvent fails", func(t *testing.T) {
		eventsForwarderService, _, eventsForwarder, _, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "resync",
				"pingType":  "ready",
			},
		}

		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: false,
		}

		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)
		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil)
		eventsForwarder.EXPECT().ForwardRoomEvent(context.Background(), gomock.Any(), gomock.Any()).Return(errors.New("error"))

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("should fail when forwardPlayerEvent fails", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, _, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.PlayerEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "playerLeft",
				"playerId":  "player",
			},
		}

		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil).Times(0)
		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(nil)
		eventsForwarder.EXPECT().ForwardPlayerEvent(context.Background(), gomock.Any(), gomock.Any()).Return(errors.New("error"))

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("should succeed even though SetScheduler to cache method fails", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "resync",
				"pingType":  "ready",
			},
		}

		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: false,
		}

		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil)
		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, nil)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(errors.New("error"))
		eventsForwarder.EXPECT().ForwardRoomEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
	})

	t.Run("should succeed even though GetScheduler from cache method fails", func(t *testing.T) {
		eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "resync",
				"pingType":  "ready",
			},
		}

		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: false,
		}

		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)
		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, errors.New("error"))
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(errors.New("error"))
		eventsForwarder.EXPECT().ForwardRoomEvent(context.Background(), gomock.Any(), gomock.Any()).Return(nil)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
	})

	t.Run("should succeed but not produce event when room type is validation", func(t *testing.T) {
		eventsForwarderService, _, _, _, roomStorage, _, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "resync",
				"pingType":  "ready",
			},
		}

		room := &game_room.GameRoom{
			ID:               event.RoomID,
			SchedulerID:      event.SchedulerID,
			IsValidationRoom: true,
		}

		schedulerCache.EXPECT().GetScheduler(gomock.Any(), event.SchedulerID).Return(expectedScheduler, nil)
		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(room, nil)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
	})

	t.Run("should fail if room cannot be found since we would be producing event for non-registered room", func(t *testing.T) {
		eventsForwarderService, config, _, schedulerStorage, roomStorage, _, schedulerCache := testSetup(t)

		event := &events.Event{
			Name:        events.RoomEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      "room",
			Attributes: map[string]interface{}{
				"eventType": "resync",
				"pingType":  "ready",
			},
		}

		roomStorage.EXPECT().GetRoom(gomock.Any(), event.SchedulerID, event.RoomID).Return(nil, errors.New("error"))
		schedulerCache.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(nil, errors.New("error"))
		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		schedulerCache.EXPECT().SetScheduler(context.Background(), expectedScheduler, config.SchedulerCacheTtl).Return(errors.New("error"))

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.Error(t, err)
	})

}

func testSetup(t *testing.T) (ports.EventsService, eventsservice.EventsForwarderConfig, *mockports.MockEventsForwarder, *mockports.MockSchedulerStorage, *mockports.MockRoomStorage, *mockports.MockGameRoomInstanceStorage, *mockports.MockSchedulerCache) {
	mockCtrl := gomock.NewController(t)

	eventsForwarder := mockports.NewMockEventsForwarder(mockCtrl)
	schedulerStorage := mockports.NewMockSchedulerStorage(mockCtrl)
	instanceStorage := mockports.NewMockGameRoomInstanceStorage(mockCtrl)
	schedulerCache := mockports.NewMockSchedulerCache(mockCtrl)
	roomStorage := mockports.NewMockRoomStorage(mockCtrl)
	config := eventsservice.EventsForwarderConfig{SchedulerCacheTtl: time.Minute}

	eventsForwarderService := eventsservice.NewEventsForwarderService(eventsForwarder, schedulerStorage, instanceStorage, roomStorage, schedulerCache, config)

	return eventsForwarderService, config, eventsForwarder, schedulerStorage, roomStorage, instanceStorage, schedulerCache
}
