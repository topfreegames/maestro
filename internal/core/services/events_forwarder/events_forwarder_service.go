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

package events_forwarder

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/topfreegames/maestro/internal/core/entities/forwarder"

	"github.com/topfreegames/maestro/internal/core/entities/events"

	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/interfaces"
	"go.uber.org/zap"
)

type EventsForwarderService struct {
	eventsForwarder  ports.EventsForwarder
	logger           *zap.Logger
	schedulerStorage ports.SchedulerStorage
	instanceStorage  ports.GameRoomInstanceStorage
	schedulerCache   ports.SchedulerCache
}

func NewEventsForwarderService(
	eventsForwarder ports.EventsForwarder,
	schedulerStorage ports.SchedulerStorage,
	instanceStorage ports.GameRoomInstanceStorage,
	schedulerCache ports.SchedulerCache,
) interfaces.EventsService {
	return &EventsForwarderService{
		eventsForwarder,
		zap.L().With(zap.String("service", "rooms_api")),
		schedulerStorage,
		instanceStorage,
		schedulerCache,
	}
}

func (es *EventsForwarderService) ProduceEvent(ctx context.Context, event *events.Event) error {
	var eventType string
	if _, ok := event.Attributes["eventType"]; ok {
		eventType = event.Attributes["eventType"].(string)
	} else {
		return errors.New("eventAttributes must contain key \"eventType\"")
	}

	var scheduler *entities.Scheduler
	var err error
	scheduler, _ = es.schedulerCache.GetScheduler(ctx, event.SchedulerID)
	if scheduler == nil {
		scheduler, err = es.schedulerStorage.GetScheduler(ctx, event.SchedulerID)
		if err != nil {
			es.logger.Error(fmt.Sprintf("Failed to get scheduler \"%v\" info", event.SchedulerID), zap.Error(err))
			return err
		}
		if err := es.schedulerCache.SetScheduler(ctx, scheduler); err != nil {
			es.logger.Error(fmt.Sprintf("Failed to set scheduler \"%v\" in cache", event.SchedulerID), zap.Error(err))
		}
	}

	forwarderList := scheduler.Forwarders
	if len(forwarderList) > 0 {
		for _, _forwarder := range forwarderList {
			switch event.Name {
			case events.RoomEvent:
				instance, err := es.instanceStorage.GetInstance(ctx, event.SchedulerID, event.RoomID)
				if err != nil {
					es.logger.Error(fmt.Sprintf("Failed to get instance for room \"%v\" from scheduler \"%v\" info", event.RoomID, event.SchedulerID), zap.Error(err))
					return err
				}
				err = es.forwardRoomEvent(ctx, event, eventType, *instance, *scheduler, _forwarder)
				if err != nil {
					return err
				}
			case events.PlayerEvent:
				err = es.forwardPlayerEvent(ctx, event, eventType, _forwarder)
				if err != nil {
					return err
				}
			}
		}
	} else {
		es.logger.Info(fmt.Sprintf("scheduler \"%v\" do not have forwarders configured", event.SchedulerID))
	}

	return nil
}

func (es *EventsForwarderService) forwardRoomEvent(
	ctx context.Context,
	event *events.Event,
	eventType string,
	instance game_room.Instance,
	scheduler entities.Scheduler,
	_forwarder *forwarder.Forwarder,
) error {
	var selectedPort int32
	if len(instance.Address.Ports) > 0 {
		selectedPort = instance.Address.Ports[0].Port
	} else {
		return errors.New("no room port found to forward roomEvent")
	}

	roomEvent := events.RoomEventType(eventType)

	var pingType events.RoomPingEventType
	if roomEvent == events.Ping {
		if _type, ok := event.Attributes["pingType"]; ok {
			pingType = events.RoomPingEventType(_type.(string))
		} else {
			return errors.New("roomEvent of type ping must contain key \"pingType\" in eventAttributes")
		}

	}

	roomAttributes := events.RoomEventAttributes{
		Game:      scheduler.Game,
		RoomId:    event.RoomID,
		Host:      instance.Address.Host,
		Port:      strconv.Itoa(int(selectedPort)),
		EventType: roomEvent,
		PingType:  &pingType,
		Other:     event.Attributes,
	}
	err := es.eventsForwarder.ForwardRoomEvent(ctx, roomAttributes, *_forwarder)
	if err != nil {
		reportRoomEventForwardingFailed(event.SchedulerID)
		es.logger.Error(fmt.Sprintf("Failed to forward room events for room %s and scheduler %s", event.RoomID, event.SchedulerID), zap.Error(err))
		return err
	}

	reportRoomEventForwardingSuccess(event.SchedulerID)
	return nil
}

func (es *EventsForwarderService) forwardPlayerEvent(
	ctx context.Context,
	event *events.Event,
	eventType string,
	_forwarder *forwarder.Forwarder,
) error {
	var playerId string
	if _playerId, ok := event.Attributes["playerId"]; ok {
		playerId = _playerId.(string)
	} else {
		return errors.New("eventAttributes must contain key \"playerId\" in playerEvent events")
	}

	playerEvent := events.PlayerEventType(eventType)
	playerAttributes := events.PlayerEventAttributes{
		RoomId:    event.RoomID,
		PlayerId:  playerId,
		EventType: playerEvent,
		Other:     event.Attributes,
	}

	err := es.eventsForwarder.ForwardPlayerEvent(ctx, playerAttributes, *_forwarder)
	if err != nil {
		reportPlayerEventForwardingFailed(event.SchedulerID)
		es.logger.Error(fmt.Sprintf("Failed to forward player events for room %s and scheduler %s", event.RoomID, event.SchedulerID), zap.Error(err))
		return err
	}
	reportPlayerEventForwardingSuccess(event.SchedulerID)
	return nil
}
