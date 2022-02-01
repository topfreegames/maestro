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

	"github.com/topfreegames/maestro/internal/core/ports/forwarder"

	"github.com/topfreegames/maestro/internal/core/entities/events"
	entities "github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/ports/errors"

	pb "github.com/topfreegames/protos/maestro/grpc/generated"
)

type eventsForwarder struct {
	forwarderClient forwarder.ForwarderClient
}

func NewEventsForwarder(forwarderClient forwarder.ForwarderClient) *eventsForwarder {
	return &eventsForwarder{
		forwarderClient: forwarderClient,
	}
}

// ForwardRoomEvent forwards room events. It receives the room event attributes and forwarder configuration.
func (f *eventsForwarder) ForwardRoomEvent(ctx context.Context, eventAttributes events.RoomEventAttributes, forwarder entities.Forwarder) error {
	switch eventAttributes.EventType {
	case events.Arbitrary:
		if roomEvent, ok := eventAttributes.Other["roomEvent"].(string); ok {
			event := pb.RoomEvent{
				Room: &pb.Room{
					Game:     eventAttributes.Game,
					RoomId:   eventAttributes.RoomId,
					Host:     eventAttributes.Host,
					Port:     eventAttributes.Port,
					Metadata: f.mergeInfos(eventAttributes.Other, forwarder.Options.Metadata),
				},
				EventType: roomEvent,
			}

			eventResponse, err := f.forwarderClient.SendRoomEvent(ctx, forwarder, &event)
			return handlerGrpcClientResponse(forwarder, eventResponse, err)
		}
		return errors.NewErrInvalidArgument("invalid or missing eventAttributes.Other['roomEvent'] field")

	case events.Ping:
		event := pb.RoomStatus{
			Room: &pb.Room{
				Game:     eventAttributes.Game,
				RoomId:   eventAttributes.RoomId,
				Host:     eventAttributes.Host,
				Port:     eventAttributes.Port,
				Metadata: f.mergeInfos(forwarder.Options.Metadata, eventAttributes.Other),
			},
			StatusType: fromRoomPingEventTypeToRoomStatusType(*eventAttributes.PingType),
		}

		eventResponse, err := f.forwarderClient.SendRoomReSync(ctx, forwarder, &event)
		return handlerGrpcClientResponse(forwarder, eventResponse, err)
	}

	return errors.NewErrUnexpected("failed to forward event room. event type doesn't exists \"%s\"", eventAttributes.EventType)
}

// ForwardPlayerEvent forwards a player events. It receives the player events attributes and forwarder configuration.
func (f *eventsForwarder) ForwardPlayerEvent(ctx context.Context, eventAttributes events.PlayerEventAttributes, forwarder entities.Forwarder) error {
	event := pb.PlayerEvent{
		PlayerId: eventAttributes.PlayerId,
		Room: &pb.Room{
			RoomId: eventAttributes.RoomId,
		},
		EventType: fromPlayerEventTypeToGrpcPlayerEventType(eventAttributes.EventType),
		Metadata:  f.mergePlayerInfos(eventAttributes.Other, forwarder.Options.Metadata),
	}

	eventResponse, err := f.forwarderClient.SendPlayerEvent(ctx, forwarder, &event)
	return handlerGrpcClientResponse(forwarder, eventResponse, err)
}

// Name returns the forwarder name. This name should be unique among other events forwarders.
func (*eventsForwarder) Name() string {
	return "noop_forwarder"
}

func (*eventsForwarder) mergeInfos(mapA map[string]interface{}, mapB map[string]interface{}) map[string]string {
	mapStringA := *fromMapInterfaceToMapString(mapA)
	mapStringB := *fromMapInterfaceToMapString(mapB)

	for k, v := range mapStringB {
		mapStringA[k] = v
	}

	metadata := mapStringA
	return metadata
}

func (*eventsForwarder) mergePlayerInfos(eventMetadata, fwdMetadata map[string]interface{}) map[string]string {
	if fwdMetadata != nil {
		if roomType, ok := fwdMetadata["roomType"]; ok {
			if eventMetadata != nil {
				eventMetadata["roomType"] = roomType
			} else {
				eventMetadata = map[string]interface{}{"roomType": roomType}
			}
		}
	}
	m := make(map[string]string)
	for key, value := range eventMetadata {
		m[key] = fmt.Sprintf("%v", value)
	}

	return m
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
		return errors.NewErrUnexpected("failed to forward event room at \"%s\"", forwarder.Name)
	}
	return nil
}
