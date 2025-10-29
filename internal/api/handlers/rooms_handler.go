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

	"github.com/topfreegames/maestro/internal/api/handlers/requestadapters"
	"github.com/topfreegames/maestro/internal/core/entities/events"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/ports"
	api "github.com/topfreegames/maestro/pkg/api/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	gameRoom, err := requestadapters.FromApiUpdateRoomRequestToEntity(message)
	handlerLogger.Debug("handling room ping request", zap.Any("message", message))
	if err != nil {
		handlerLogger.Error("error parsing ping request", zap.Any("ping", message), zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	err = h.roomManager.UpdateRoom(ctx, gameRoom)
	if err != nil {
		handlerLogger.Error("error updating room with ping", zap.Any("ping", message), zap.Error(err))
		return &api.UpdateRoomWithPingResponse{Success: false}, nil
	}

	handlerLogger.Debug("Room updated with ping successfully")
	return &api.UpdateRoomWithPingResponse{Success: true}, nil
}

// UpdateRoomStatus was only implemented to keep compatibility with previous maestro version (v9), it has no inner execution since the
// ping event is already forwarding the incoming rooms status to matchmaker
func (h *RoomsHandler) UpdateRoomStatus(ctx context.Context, message *api.UpdateRoomStatusRequest) (*api.UpdateRoomStatusResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, message.SchedulerName), zap.String(logs.LogFieldRoomID, message.RoomName))
	eventMetadata := message.Metadata.AsMap()
	eventMetadata["eventType"] = events.FromRoomEventTypeToString(events.Status)
	eventMetadata["pingType"] = message.Status

	err := h.eventsService.ProduceEvent(ctx, events.NewRoomEvent(message.SchedulerName, message.RoomName, eventMetadata))
	if err != nil {
		handlerLogger.Error("error forwarding room status event", zap.Any("event_message", message), zap.Error(err))
		return &api.UpdateRoomStatusResponse{Success: false}, nil
	}
	return &api.UpdateRoomStatusResponse{Success: true}, nil
}

func (h *RoomsHandler) GetRoomAddress(ctx context.Context, message *api.GetRoomAddressRequest) (*api.GetRoomAddressResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, message.SchedulerName), zap.String(logs.LogFieldRoomID, message.RoomName))
	instance, err := h.roomManager.GetRoomInstance(ctx, message.SchedulerName, message.RoomName)
	if err != nil {
		handlerLogger.Error("error getting room instance", zap.Any("message", message), zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}
	return requestadapters.FromInstanceEntityToGameRoomAddressResponse(instance), nil
}

func (h *RoomsHandler) AllocateRoom(ctx context.Context, message *api.AllocateRoomRequest) (*api.AllocateRoomResponse, error) {
	handlerLogger := h.logger.With(zap.String(logs.LogFieldSchedulerName, message.SchedulerName))
	handlerLogger.Debug("handling room allocation request", zap.Any("message", message))

	roomID, err := h.roomManager.AllocateRoom(ctx, message.SchedulerName)
	if err != nil {
		handlerLogger.Error("error allocating room", zap.Any("allocation_request", message), zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}

	// Get room instance to retrieve address information
	instance, err := h.roomManager.GetRoomInstance(ctx, message.SchedulerName, roomID)
	if err != nil {
		handlerLogger.Error("error getting allocated room instance", zap.String(logs.LogFieldRoomID, roomID), zap.Error(err))
		return nil, status.Error(codes.Unknown, err.Error())
	}

	// Convert instance to address response format and include in allocation response
	addressResponse := requestadapters.FromInstanceEntityToGameRoomAddressResponse(instance)

	handlerLogger.Debug("room allocated successfully", zap.String(logs.LogFieldRoomID, roomID))
	return &api.AllocateRoomResponse{
		RoomId:  roomID,
		Success: true,
		Message: "",
		Ports:   addressResponse.Ports,
		Host:    addressResponse.Host,
	}, nil
}
