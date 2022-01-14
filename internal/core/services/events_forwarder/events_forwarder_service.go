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

	"github.com/topfreegames/maestro/internal/core/entities/events"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/services/interfaces"
	"go.uber.org/zap"
)

type EventsForwarderService struct {
	eventsForwarder ports.EventsForwarder
	logger          *zap.Logger
}

func NewEventsForwarderService(eventsForwarder ports.EventsForwarder) interfaces.EventsService {
	return &EventsForwarderService{
		eventsForwarder,
		zap.L().With(zap.String("service", "rooms_api")),
	}
}

func (es *EventsForwarderService) ProduceEvent(ctx context.Context, event *events.Event) error {
	room := &game_room.GameRoom{}
	instance := &game_room.Instance{}

	switch event.Name {
	case events.RoomEvent:
		return es.forwardRoomEvent(ctx, room, instance, map[string]interface{}{}, "")
	case events.PlayerEvent:
		return es.forwardPlayerEvent(ctx, room, map[string]interface{}{}, "")
	}
	return nil

}

func (es *EventsForwarderService) forwardRoomEvent(ctx context.Context, room *game_room.GameRoom, instance *game_room.Instance, attributes map[string]interface{}, options interface{}) error {
	err := es.eventsForwarder.ForwardRoomEvent(ctx, room, instance, attributes, options)
	if err != nil {
		reportRoomEventForwardingFailed(room.SchedulerID)
		es.logger.Error(fmt.Sprintf("Failed to forward room events for room %s and scheduler %s", room.ID, room.SchedulerID), zap.Error(err))
		return err
	}
	reportRoomEventForwardingSuccess(room.SchedulerID)
	return nil
}

func (es *EventsForwarderService) forwardPlayerEvent(ctx context.Context, room *game_room.GameRoom, attributes map[string]interface{}, options interface{}) error {
	err := es.eventsForwarder.ForwardPlayerEvent(ctx, room, attributes, options)
	if err != nil {
		reportPlayerEventForwardingFailed(room.SchedulerID)
		es.logger.Error(fmt.Sprintf("Failed to forward player events for room %s and scheduler %s", room.ID, room.SchedulerID), zap.Error(err))
		return err
	}
	reportPlayerEventForwardingSuccess(room.SchedulerID)
	return nil
}
