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

package ports

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	pb "github.com/topfreegames/protos/maestro/grpc/generated"
	"google.golang.org/grpc/codes"

	"github.com/topfreegames/maestro/internal/core/entities/events"
)

// Primary ports (input, driving ports)

type EventsService interface {
	// ProduceEvent forwards an events. Internally it resolves the scheduler to fetch the forwarders configuration and calls the proper events forwarders.
	ProduceEvent(ctx context.Context, event *events.Event) error
}

// Secondary ports (output, driven ports)

type EventsForwarder interface {
	// ForwardRoomEvent forwards room events. It receives the room event attributes and forwarder configuration.
	ForwardRoomEvent(ctx context.Context, eventAttributes events.RoomEventAttributes, forwarder forwarder.Forwarder) (codes.Code, error)
	// ForwardPlayerEvent forwards a player events. It receives the player events attributes and forwarder configuration.
	ForwardPlayerEvent(ctx context.Context, eventAttributes events.PlayerEventAttributes, forwarder forwarder.Forwarder) (codes.Code, error)
	// Name returns the forwarder name. This name should be unique among other events forwarders.
	Name() string
}

type ForwarderClient interface {
	SendRoomEvent(ctx context.Context, forwarder forwarder.Forwarder, in *pb.RoomEvent) (*pb.Response, error)
	SendRoomReSync(ctx context.Context, forwarder forwarder.Forwarder, in *pb.RoomStatus) (*pb.Response, error)
	SendRoomStatus(ctx context.Context, forwarder forwarder.Forwarder, in *pb.RoomStatus) (*pb.Response, error)
	SendPlayerEvent(ctx context.Context, forwarder forwarder.Forwarder, in *pb.PlayerEvent) (*pb.Response, error)
	CacheFlush()
	CacheDelete(forwarderAddress string) error
}
