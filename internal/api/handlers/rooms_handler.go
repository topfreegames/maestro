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

	"github.com/topfreegames/maestro/internal/core/ports"

	portsErrors "github.com/topfreegames/maestro/internal/core/ports/errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/services/room_manager"

	api "github.com/topfreegames/maestro/pkg/api/v1"
)

type RoomsHandler struct {
	roomManager     *room_manager.RoomManager
	eventsForwarder ports.EventsForwarder
	api.UnimplementedRoomsServiceServer
}

func ProvideRoomsHandler(roomManager *room_manager.RoomManager, eventsForwarder ports.EventsForwarder) *RoomsHandler {
	return &RoomsHandler{
		roomManager:     roomManager,
		eventsForwarder: eventsForwarder,
	}
}

func (h *RoomsHandler) ForwardRoomEvent(ctx context.Context, message *api.ForwardRoomEventRequest) (*api.ForwardRoomEventResponse, error) {
	room := &game_room.GameRoom{ID: message.RoomName, SchedulerID: message.SchedulerName, Metadata: message.Metadata.AsMap()}
	if message.Metadata != nil {
		room.Metadata["eventType"] = message.Event
	} else {
		room.Metadata = map[string]interface{}{
			"eventType": message.Event,
		}
	}

	err := h.eventsForwarder.ForwardRoomEvent(room, ctx, "roomEvent", "", room.Metadata)
	if err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}
	return &api.ForwardRoomEventResponse{Success: true, Message: ""}, nil
}

func (h *RoomsHandler) UpdateRoomWithPing(ctx context.Context, message *api.UpdateRoomWithPingRequest) (*api.UpdateRoomWithPingResponse, error) {
	gameRoom, err := h.fromApiUpdateRoomRequestToEntity(message)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	err = h.roomManager.UpdateRoom(ctx, gameRoom)
	if err != nil {
		if errors.Is(err, portsErrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}

		// TODO(gabrielcorado): should we fail when the status transition fails?
		return nil, status.Error(codes.Unknown, err.Error())
	}

	return &api.UpdateRoomWithPingResponse{Success: true}, nil
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
