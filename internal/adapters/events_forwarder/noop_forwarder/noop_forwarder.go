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

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	"github.com/topfreegames/maestro/internal/core/entities/events"
	entities "github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/ports"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"

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

		eventResponse, err := f.forwarderGrpc.SendRoomEvent(ctx, &event)
		return handlerGrpcClientResponse(forwarder, eventResponse, err)
	}

	if eventAttributes.EventType == events.Ping {
		eventResponse, err := f.forwarderRoomEventPing(ctx, eventAttributes, forwarder)
		return handlerGrpcClientResponse(forwarder, eventResponse, err)
	}

	return errors.NewErrUnexpected("failed to forwarder event room. event type doesn't exists \"%s\"", eventAttributes.EventType)
}

func (f *noopForwarder) forwarderRoomEventArbitrary(ctx context.Context, eventAttributes events.RoomEventAttributes, forwarder entities.Forwarder) (*pb.Response, error) {
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

	return f.forwarderGrpc.SendRoomEvent(ctx, &event)
}

func (f *noopForwarder) forwarderRoomEventPing(ctx context.Context, eventAttributes events.RoomEventAttributes, forwarder entities.Forwarder) (*pb.Response, error) {
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
	ctx, cancel := context.WithTimeout(ctx, forwarder.Options.Timeout)
	defer cancel()

	if &event == nil {
		return nil, nil
	}
	return &pb.Response{Code: 200}, nil
	return f.forwarderGrpc.SendRoomResync(ctx, &event)
}

// ForwardPlayerEvent forwards a player events. It receives the player events attributes and forwarder configuration.
func (f *noopForwarder) ForwardPlayerEvent(ctx context.Context, eventAttributes events.PlayerEventAttributes, forwarder entities.Forwarder) error {
	grpcClient, err := f.getGrpcClient(forwarder.Address)
	if err != nil {
		return errors.NewErrUnexpected("failed to connect at %s", forwarder.Address).WithError(err)
	}

	event := pb.PlayerEvent{
		PlayerId: eventAttributes.PlayerId,
		Room: &pb.Room{
			RoomId:   eventAttributes.RoomId,
			Metadata: *fromMapInterfaceToMapString(eventAttributes.Other),
		},
		EventType: fromPlayerEventTypeToGrpcPlayerEventType(eventAttributes.EventType),
		Metadata:  *fromMapInterfaceToMapString(forwarder.Options.Metadata),
	}

	ctx, cancel := context.WithTimeout(ctx, forwarder.Options.Timeout)
	defer cancel()

	if &event == nil {
		return nil
	}
	return nil

	eventResponse, err := grpcClient.SendPlayerEvent(ctx, &event)
	if err != nil {
		return err
	}
	if eventResponse.Code != 200 {
		return errors.NewErrUnexpected("failed to forwarder event room at \"%s\"", forwarder.Name)
	}
	return nil
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

//  getGrpcClient returns grpcForwarderClient. The client configuration was maintained in a cache
func (f *noopForwarder) getGrpcClient(address string) (pb.GRPCForwarderClient, error) {
	return nil, nil
}

func (f *noopForwarder) configureGRPCForwarder(forwarder entities.Forwarder) (pb.GRPCForwarderClient, error) {
	if forwarder.Address == "" {
		return nil, errors.NewErrInvalidArgument("no grpc server address informed")
	}

	zap.L().Info(fmt.Sprintf("connecting to grpc server at: %s", forwarder.Address))
	tracer := opentracing.GlobalTracer()
	conn, err := grpc.Dial(
		forwarder.Address,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(tracer)),
	)
	if err != nil {
		return nil, err
	}
	client := pb.NewGRPCForwarderClient(conn)
	return client, nil
	// func (g *GRPCForwarder) configure() error {
	// 	l := g.logger.WithFields(log.Fields{
	// 	"op": "configure",
	// })
	// 	g.serverAddress = g.config.GetString("address")
	// 	if g.serverAddress == "" {
	// 	return fmt.Errorf("no grpc server address informed")
	// }
	// 	l.Infof("connecting to grpc server at: %s", g.serverAddress)
	// 	tracer := opentracing.GlobalTracer()
	// 	conn, err := grpc.Dial(
	// 	g.serverAddress,
	// 	grpc.WithInsecure(),
	// 	grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(tracer)),
	// )
	// 	if err != nil {
	// 	return err
	// }
	// 	g.client = pb.NewGRPCForwarderClient(conn)
	// 	g.metadata = g.config.GetStringMap("metadata")
	// 	return nil
	// }
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

func handlerGrpcClientResponse(forwarder entities.Forwarder,  eventResponse *pb.Response, err error) error {
	if err != nil {
		return err
	}
	if eventResponse.Code != 200 {
		return errors.NewErrUnexpected("failed to forwarder event room at \"%s\"", forwarder.Name)
	}
	return nil
}
