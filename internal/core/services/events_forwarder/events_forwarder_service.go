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

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"
	"go.uber.org/zap"
)

type EventsForwarderService struct {
	eventsForwarder ports.EventsForwarder
	logger          *zap.Logger
}

func NewEventsForwarderService(eventsForwader ports.EventsForwarder) *EventsForwarderService {
	return &EventsForwarderService{
		eventsForwader,
		zap.L().With(zap.String("service", "rooms_api")),
	}
}

func (es *EventsForwarderService) ForwardRoomEvent(ctx context.Context, room *game_room.GameRoom, eventType, status string) error {
	err := es.eventsForwarder.ForwardRoomEvent(room, ctx, status, eventType, room.Metadata)
	if err != nil {
		reportRoomEventForwardingFailed(room.SchedulerID)
		es.logger.Error("Failed to forward room event", zap.Error(err))
		return err
	}
	return nil
}

func (es EventsForwarderService) ForwardPlayerEvent(ctx context.Context, room *game_room.GameRoom, event string) error {
	err := es.eventsForwarder.ForwardPlayerEvent(room, ctx, event, room.Metadata)
	if err != nil {
		reportRoomEventForwardingFailed(room.SchedulerID)
		es.logger.Error("Failed to forward player event", zap.Error(err))
		return err
	}
	return nil
}
