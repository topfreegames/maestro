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

package noop_forwarder

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities/events"
	entities "github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"

	pb "github.com/topfreegames/protos/maestro/grpc/generated"
)

type noopForwarder struct {
	forwarderGrpc ports.ForwarderGrpc
	GrpcClient    pb.GRPCForwarderClient
}

func NewNoopForwarder(forwarderGrpc ports.ForwarderGrpc) *noopForwarder {
	return &noopForwarder{
		forwarderGrpc: forwarderGrpc,
	}
}

// ForwardRoomEvent forwards room events. It receives the room event attributes and forwarder configuration.
func (f *noopForwarder) ForwardRoomEvent(ctx context.Context, eventAttributes events.RoomEventAttributes, forwarder entities.Forwarder) error {
	if eventAttributes.EventType == events.Arbitrary {
		event := pb.RoomEvent{
			Room: &pb.Room{
				Game:     eventAttributes.Game,
				RoomId:   eventAttributes.RoomId,
				Host:     eventAttributes.Host,
				Port:     eventAttributes.Port,
				Metadata: *fromMapInterfaceToMapString(eventAttributes.Attributes),
			},
			EventType: entities.FromForwardTypeToString(forwarder.ForwardType),
			Metadata:  *fromMapInterfaceToMapString(forwarder.Options.Metadata),
		}

		eventResponse, err := f.forwarderGrpc.SendRoomEvent(ctx, forwarder, &event)
		return handlerGrpcClientResponse(forwarder, eventResponse, err)
	}

	if eventAttributes.EventType == events.Ping {
		event := pb.RoomStatus{
			Room: &pb.Room{
				Game:     eventAttributes.Game,
				RoomId:   eventAttributes.RoomId,
				Host:     eventAttributes.Host,
				Port:     eventAttributes.Port,
				Metadata: *fromMapInterfaceToMapString(eventAttributes.Attributes),
			},
			StatusType: fromRoomPingEventTypeToRoomStatusType(*eventAttributes.PingType),
		}

		eventResponse, err := f.forwarderGrpc.SendRoomReSync(ctx, forwarder, &event)
		return handlerGrpcClientResponse(forwarder, eventResponse, err)
	}

	return errors.NewErrUnexpected("failed to forwarder event room. event type doesn't exists \"%s\"", eventAttributes.EventType)
}

// ForwardPlayerEvent forwards a player events. It receives the player events attributes and forwarder configuration.
func (f *noopForwarder) ForwardPlayerEvent(ctx context.Context, eventAttributes events.PlayerEventAttributes, forwarder entities.Forwarder) error {
	event := pb.PlayerEvent{
		PlayerId: eventAttributes.PlayerId,
		Room: &pb.Room{
			RoomId:   eventAttributes.RoomId,
			Metadata: *fromMapInterfaceToMapString(eventAttributes.Other),
		},
		EventType: fromPlayerEventTypeToGrpcPlayerEventType(eventAttributes.EventType),
		Metadata:  *fromMapInterfaceToMapString(forwarder.Options.Metadata),
	}

	eventResponse, err := f.forwarderGrpc.SendPlayerEvent(ctx, forwarder, &event)
	return handlerGrpcClientResponse(forwarder, eventResponse, err)
}

// ForwardRoomEventObsolete forwards room events. It receives the game room, its instance, and additional attributes.
func (*noopForwarder) ForwardRoomEventObsolete(ctx context.Context, gameRoom *game_room.GameRoom, instance *game_room.Instance, attributes map[string]interface{}, options interface{}) error {
	return nil
}

// ForwardPlayerEventObsolete forwards a player events. It receives the game room and additional attributes.
func (*noopForwarder) ForwardPlayerEventObsolete(ctx context.Context, gameRoom *game_room.GameRoom, attributes map[string]interface{}, options interface{}) error {
	return nil
}

// Name returns the forwarder name. This name should be unique among other events forwarders.
func (*noopForwarder) Name() string {
	return "noop_forwarder"
}

func fromMapInterfaceToMapString(mapInterface map[string]interface{}) *map[string]string {
	mapString := make(map[string]string)
	for key, value := range mapInterface {
		if v, ok := value.(string); ok {
			mapString[key] = v
		} else {
			mapString[key] = fmt.Sprintf("%v", value)
		}
	}
	return &mapString
}

func fromRoomPingEventTypeToRoomStatusType(eventType events.RoomPingEventType) pb.RoomStatus_RoomStatusType {
	switch eventType {
	case events.RoomPingReady:
		return pb.RoomStatus_ready
	case events.RoomPingOccupied:
		return pb.RoomStatus_occupied
	case events.RoomPingTerminating:
		return pb.RoomStatus_terminating
	case events.RoomPingTerminated:
		return pb.RoomStatus_terminated
	default:
		return pb.RoomStatus_terminated
	}
}

func fromPlayerEventTypeToGrpcPlayerEventType(eventType events.PlayerEventType) pb.PlayerEvent_PlayerEventType {
	switch eventType {
	case events.PlayerLeft:
		return pb.PlayerEvent_PLAYER_LEFT
	case events.PlayerJoin:
		return pb.PlayerEvent_PLAYER_JOINED
	default:
		return pb.PlayerEvent_PLAYER_JOINED
	}
}

func handlerGrpcClientResponse(forwarder entities.Forwarder, eventResponse *pb.Response, err error) error {
	if err != nil {
		return err
	}
	if eventResponse.Code != 200 {
		return errors.NewErrUnexpected("failed to forwarder event room at \"%s\"", forwarder.Name)
	}
	return nil
}
