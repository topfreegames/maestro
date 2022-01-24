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
	"fmt"
	"strconv"

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
	roomStorage      ports.RoomStorage
	instanceStorage  ports.GameRoomInstanceStorage
}

func NewEventsForwarderService(eventsForwarder ports.EventsForwarder, schedulerStorage ports.SchedulerStorage, rommStorage ports.RoomStorage, instanceStorage ports.GameRoomInstanceStorage) interfaces.EventsService {
	return &EventsForwarderService{
		eventsForwarder,
		zap.L().With(zap.String("service", "rooms_api")),
		schedulerStorage,
		rommStorage,
		instanceStorage,
	}
}

func (es *EventsForwarderService) ProduceEvent(ctx context.Context, event *events.Event) error {
	scheduler, err := es.schedulerStorage.GetScheduler(ctx, event.SchedulerID)
	if err != nil {
		es.logger.Error(fmt.Sprintf("Failed to get scheduler \"%v\" info", event.SchedulerID), zap.Error(err))
		return err
	}
	room, err := es.roomStorage.GetRoom(ctx, event.SchedulerID, event.RoomID)
	if err != nil {
		es.logger.Error(fmt.Sprintf("Failed to get room \"%v\" from scheduler \"%v\" info", event.RoomID, event.SchedulerID), zap.Error(err))
		return err
	}
	instance, err := es.instanceStorage.GetInstance(ctx, event.SchedulerID, event.RoomID)
	if err != nil {
		es.logger.Error(fmt.Sprintf("Failed to get instance for room \"%v\" from scheduler \"%v\" info", event.RoomID, event.SchedulerID), zap.Error(err))
		return err
	}
	forwarderList := scheduler.Forwarders
	for _, _forwarder := range forwarderList {
		switch event.Name {
		case events.RoomEvent:
			roomAttributes := events.RoomEventAttributes{
				Game:       scheduler.Game,
				RoomId:     event.RoomID,
				Host:       instance.Address.Host,
				Port:       strconv.Itoa(int(instance.Address.Ports[0].Port)),
				EventType:  events.RoomEventType(room.Metadata["eventType"].(string)),
				PingType:   nil,
				Attributes: nil,
			}
			return es.forwardRoomEvent(ctx, event, roomAttributes, _forwarder)
		case events.PlayerEvent:
			playerAttributes := events.PlayerEventAttributes{
				RoomId:    event.RoomID,
				PlayerId:  room.Metadata["playerId"].(string),
				EventType: events.PlayerEventType(room.Metadata["eventType"].(string)),
				Other:     nil,
			}
			return es.forwardPlayerEvent(ctx, event, playerAttributes, _forwarder)
		}
	}

	return nil
}

func (es *EventsForwarderService) forwardRoomEvent(ctx context.Context, event *events.Event, eventAttributes events.RoomEventAttributes, _forwarder *forwarder.Forwarder) error {
	err := es.eventsForwarder.ForwardRoomEvent(ctx, eventAttributes, *_forwarder)
	if err != nil {
		reportRoomEventForwardingFailed(event.SchedulerID)
		es.logger.Error(fmt.Sprintf("Failed to forward room events for room %s and scheduler %s", eventAttributes.RoomId, event.SchedulerID), zap.Error(err))
		return err
	}
	reportRoomEventForwardingSuccess(event.SchedulerID)
	return nil
}

func (es *EventsForwarderService) forwardPlayerEvent(ctx context.Context, event *events.Event, eventAttributes events.PlayerEventAttributes, _forwarder *forwarder.Forwarder) error {
	err := es.eventsForwarder.ForwardPlayerEvent(ctx, eventAttributes, *_forwarder)
	if err != nil {
		reportPlayerEventForwardingFailed(event.SchedulerID)
		es.logger.Error(fmt.Sprintf("Failed to forward player events for room %s and scheduler %s", eventAttributes.RoomId, event.SchedulerID), zap.Error(err))
		return err
	}
	reportPlayerEventForwardingSuccess(event.SchedulerID)
	return nil
}
