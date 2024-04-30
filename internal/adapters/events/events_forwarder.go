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

package events

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/topfreegames/maestro/internal/core/entities/events"
	entities "github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/ports/errors"

	pb "github.com/topfreegames/protos/maestro/grpc/generated"
)

const UnknownCode = -1

type eventsForwarder struct {
	forwarderClient ports.ForwarderClient
}

func NewEventsForwarder(forwarderClient ports.ForwarderClient) *eventsForwarder {
	return &eventsForwarder{
		forwarderClient: forwarderClient,
	}
}

// ForwardRoomEvent forwards room events. It receives the room event attributes and forwarder configuration.
func (f *eventsForwarder) ForwardRoomEvent(ctx context.Context, eventAttributes events.RoomEventAttributes, forwarder entities.Forwarder) (codes.Code, error) {
	switch eventAttributes.EventType {
	case events.Arbitrary:
		if roomEvent, ok := eventAttributes.Other["roomEvent"].(string); ok {
			event := f.buildRoomEventMessage(eventAttributes, forwarder, roomEvent)
			eventResponse, err := f.forwarderClient.SendRoomEvent(ctx, forwarder, &event)
			return handlerGrpcClientResponse(forwarder, eventResponse, err)
		}
		return codes.InvalidArgument, errors.NewErrInvalidArgument("invalid or missing eventAttributes.Other['roomEvent'] field")

	case events.Ping:
		event, err := f.buildRoomStatusMessage(eventAttributes, forwarder)
		if err != nil {
			return codes.InvalidArgument, errors.NewErrInvalidArgument("failed to build room status message: %s", err)
		}

		eventResponse, err := f.forwarderClient.SendRoomReSync(ctx, forwarder, &event)

		return handlerGrpcClientResponse(forwarder, eventResponse, err)
	case events.Status:
		event, err := f.buildRoomStatusMessage(eventAttributes, forwarder)
		if err != nil {
		}

		eventResponse, err := f.forwarderClient.SendRoomStatus(ctx, forwarder, &event)

		return handlerGrpcClientResponse(forwarder, eventResponse, err)
	}

	return codes.NotFound, errors.NewErrUnexpected("failed to forward event room. event type doesn't exists \"%s\"", eventAttributes.EventType)
}

// ForwardPlayerEvent forwards a player events. It receives the player events attributes and forwarder configuration.
func (f *eventsForwarder) ForwardPlayerEvent(ctx context.Context, eventAttributes events.PlayerEventAttributes, forwarder entities.Forwarder) (codes.Code, error) {
	event := f.buildPlayerEventMessage(eventAttributes, forwarder)
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

func (f *eventsForwarder) buildRoomStatusMessage(eventAttributes events.RoomEventAttributes, forwarder entities.Forwarder) (pb.RoomStatus, error) {
	statusType, err := fromStatusToRoomStatusType(*eventAttributes.RoomStatusType)
	if err != nil {
		return pb.RoomStatus{}, fmt.Errorf("failed to convert status type: %w", err)
	}

	event := pb.RoomStatus{
		Room: &pb.Room{
			Game:     eventAttributes.Game,
			RoomId:   eventAttributes.RoomId,
			Host:     eventAttributes.Host,
			Port:     eventAttributes.Port,
			Metadata: f.mergeInfos(forwarder.Options.Metadata, eventAttributes.Other),
		},
		StatusType: statusType,
	}
	if roomType, ok := forwarder.Options.Metadata["roomType"].(string); ok {
		event.Room.RoomType = roomType
	}
	return event, nil
}

func (f *eventsForwarder) buildRoomEventMessage(eventAttributes events.RoomEventAttributes, forwarder entities.Forwarder, roomEvent string) pb.RoomEvent {
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
	return event
}

func (f *eventsForwarder) buildPlayerEventMessage(eventAttributes events.PlayerEventAttributes, forwarder entities.Forwarder) pb.PlayerEvent {
	event := pb.PlayerEvent{
		PlayerId: eventAttributes.PlayerId,
		Room: &pb.Room{
			Game:   eventAttributes.Game,
			RoomId: eventAttributes.RoomId,
		},
		EventType: fromPlayerEventTypeToGrpcPlayerEventType(eventAttributes.EventType),
		Metadata:  f.mergePlayerInfos(eventAttributes.Other, forwarder.Options.Metadata),
	}
	return event
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

func fromStatusToRoomStatusType(eventType events.RoomStatusType) (pb.RoomStatus_RoomStatusType, error) {
	switch eventType {
	case events.RoomStatusReady:
		return pb.RoomStatus_ready, nil
	case events.RoomStatusOccupied:
		return pb.RoomStatus_occupied, nil
	case events.RoomStatusTerminating:
		return pb.RoomStatus_terminating, nil
	case events.RoomStatusTerminated:
		return pb.RoomStatus_terminated, nil
	default:
		return 0, errors.NewErrInvalidArgument(
			"invalid event type %q, expected one of %v",
			eventType,
			[]events.RoomStatusType{
				events.RoomStatusReady,
				events.RoomStatusOccupied,
				events.RoomStatusTerminated,
				events.RoomStatusTerminating,
			})
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

// handlerGrpcClientResponse checks if the gRPC response had unexpected communication or network errors.
func handlerGrpcClientResponse(forwarder entities.Forwarder, eventResponse *pb.Response, err error) (codes.Code, error) {
	if err != nil {
		grpcStatus, ok := status.FromError(err)
		if !ok {
			return codes.Unknown, errors.NewErrUnexpected("failed to forward event room at \"%s\" with unknown grpc code: %s", forwarder.Name, err.Error())
		}

		return grpcStatus.Code(), errors.NewErrUnexpected("failed to forward event room at \"%s\" with code %d ", forwarder.Name, grpcStatus.Code())
	}

	// Was able to successfully forward, even though the forward response may contain errors sent by the receiver.
	return codes.Code(eventResponse.Code), nil
}
