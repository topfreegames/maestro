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

package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/topfreegames/maestro/internal/core/logs"

	"go.uber.org/zap"

	"github.com/topfreegames/maestro/internal/core/ports"

	"github.com/topfreegames/maestro/internal/core/entities/events"

	portsErrors "github.com/topfreegames/maestro/internal/core/ports/errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	api "github.com/topfreegames/maestro/pkg/api/v1"
)

type RoomsHandler struct {
	roomManager   ports.RoomManager
	eventsService ports.EventsService
	logger        *zap.Logger
	api.UnimplementedRoomsServiceServer
}

func ProvideRoomsHandler(roomManager ports.RoomManager, eventsService ports.EventsService) *RoomsHandler {
	return &RoomsHandler{
		roomManager:   roomManager,
		eventsService: eventsService,
		logger: zap.L().
			With(zap.String(logs.LogFieldComponent, "handler"), zap.String(logs.LogFieldHandlerName, "rooms_handler")),
	}
}

func (h *RoomsHandler) ForwardRoomEvent(ctx context.Context, message *api.ForwardRoomEventRequest) (*api.ForwardRoomEventResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, message.SchedulerName), zap.String(logs.LogFieldRoomID, message.RoomName))
	eventMetadata := message.Metadata.AsMap()
	eventMetadata["eventType"] = events.FromRoomEventTypeToString(events.Arbitrary)
	eventMetadata["roomEvent"] = message.Event

	err := h.eventsService.ProduceEvent(ctx, events.NewRoomEvent(message.SchedulerName, message.RoomName, eventMetadata))
	if err != nil {
		handlerLogger.Error("error forwarding room event", zap.Any("event_message", message), zap.Error(err))
		return &api.ForwardRoomEventResponse{Success: false, Message: err.Error()}, nil
	}
	return &api.ForwardRoomEventResponse{Success: true, Message: ""}, nil
}

func (h *RoomsHandler) ForwardPlayerEvent(ctx context.Context, message *api.ForwardPlayerEventRequest) (*api.ForwardPlayerEventResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, message.SchedulerName), zap.String(logs.LogFieldRoomID, message.RoomName))
	eventMetadata := message.Metadata.AsMap()
	eventMetadata["eventType"] = message.Event

	err := h.eventsService.ProduceEvent(ctx, events.NewPlayerEvent(message.SchedulerName, message.RoomName, eventMetadata))
	if err != nil {
		handlerLogger.Error("error forwarding player event", zap.Any("event_message", message), zap.Error(err))
		return &api.ForwardPlayerEventResponse{Success: false, Message: err.Error()}, nil
	}
	return &api.ForwardPlayerEventResponse{Success: true, Message: ""}, nil
}

func (h *RoomsHandler) UpdateRoomWithPing(ctx context.Context, message *api.UpdateRoomWithPingRequest) (*api.UpdateRoomWithPingResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, message.SchedulerName), zap.String(logs.LogFieldRoomID, message.RoomName))
	gameRoom, err := h.fromApiUpdateRoomRequestToEntity(message)
	handlerLogger.Info("handling room ping request", zap.Any("message", message))
	if err != nil {
		handlerLogger.Error("error parsing ping request", zap.Any("ping", message), zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	// TODO(caio.rodrigues): receive only scheduler, room and metadata. Fetch room from storage on manager before producing event
	err = h.roomManager.UpdateRoom(ctx, gameRoom)
	if err != nil {
		handlerLogger.Error("error updating room with ping", zap.Any("ping", message), zap.Error(err))
		if errors.Is(err, portsErrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}

		// TODO(gabrielcorado): should we fail when the status transition fails?
		return nil, status.Error(codes.Unknown, err.Error())
	}

	handlerLogger.Info("Room updated with ping successfully")
	return &api.UpdateRoomWithPingResponse{Success: true}, nil
}

// UpdateRoomStatus was only implemented to keep compatibility with previous maestro version (v9), it has no inner execution since the
// ping event is already forwarding the incoming rooms status to matchmaker
func (h *RoomsHandler) UpdateRoomStatus(ctx context.Context, message *api.UpdateRoomStatusRequest) (*api.UpdateRoomStatusResponse, error) {
	return &api.UpdateRoomStatusResponse{Success: true}, nil
}

func (h *RoomsHandler) fromApiUpdateRoomRequestToEntity(request *api.UpdateRoomWithPingRequest) (*game_room.GameRoom, error) {
	status, err := game_room.FromStringToGameRoomPingStatus(request.GetStatus())
	if err != nil {
		return nil, err
	}

	return &game_room.GameRoom{
		ID:          request.GetRoomName(),
		SchedulerID: request.GetSchedulerName(),
		PingStatus:  status,
		Metadata:    request.Metadata.AsMap(),
		LastPingAt:  time.Unix(request.GetTimestamp(), 0),
	}, nil
}
