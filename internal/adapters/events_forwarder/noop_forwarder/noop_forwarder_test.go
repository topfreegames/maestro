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
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	pb "github.com/topfreegames/protos/maestro/grpc/generated"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/adapters/forwarder_grpc/mock"
	"github.com/topfreegames/maestro/internal/core/entities/events"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
)

var (
	mockCtrl *gomock.Controller
	forwarderGrpcMock  *mock.MockForwarderGrpc
	noopForwarderAdapter *noopForwarder
)

func TestForwardRoomEvent_Arbitrary(t *testing.T) {
	t.Run("with success when event type is Arbitrary", func(t *testing.T) {
		// arr
		basicArrange(t)
		forwarderGrpcMock.EXPECT().SendRoomEvent(gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 200}, nil )

		// act
		err := noopForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.Arbitrary), newForwarder())

		// ass
		require.NoError(t, err)
		require.Nil(t, err)
	})

	t.Run("failed when grpcClient returns error", func(t *testing.T) {
		// arr
		basicArrange(t)
		forwarderGrpcMock.EXPECT().SendRoomEvent(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrNotFound("an error occurred"))

		// act
		err := noopForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.Arbitrary), newForwarder())

		// ass
		require.Error(t, err)
		require.NotNil(t, err)
	})

	t.Run("failed when grpcClient returns statusCode not equal 200", func(t *testing.T) {
		basicArrange(t)
		forwarderGrpcMock.EXPECT().SendRoomEvent(gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 404}, nil)

		err := noopForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.Arbitrary), newForwarder())

		require.Error(t, err)
		require.NotNil(t, err)
	})
}

func TestForwardRoomEvent_Ping(t *testing.T) {
	t.Run("with success when event type is Ping", func(t *testing.T) {
		// arr
		basicArrange(t)
		forwarderGrpcMock.EXPECT().SendRoomResync(gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 200}, nil )

		// act
		err := noopForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.Ping), newForwarder())

		// ass
		require.NoError(t, err)
		require.Nil(t, err)
	})

	t.Run("failed when grpcClient returns error", func(t *testing.T) {
		// arr
		basicArrange(t)
		forwarderGrpcMock.EXPECT().SendRoomResync(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrNotFound("an error occurred"))

		// act
		err := noopForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.Ping), newForwarder())

		// ass
		require.Error(t, err)
		require.NotNil(t, err)
	})

	t.Run("failed when grpcClient returns statusCode not equal 200", func(t *testing.T) {
		basicArrange(t)
		forwarderGrpcMock.EXPECT().SendRoomResync(gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 404}, nil)

		err := noopForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.Ping), newForwarder())

		require.Error(t, err)
		require.NotNil(t, err)
	})
}

func TestForwardPlayerEvent(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		// arr
		basicArrange(t)
		forwarderGrpcMock.EXPECT().SendPlayerEvent(gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 200}, nil )

		// act
		err := noopForwarderAdapter.ForwardPlayerEvent(context.Background(), newPlayerEventAttributes(), newForwarder())

		// ass
		require.NoError(t, err)
		require.Nil(t, err)
	})

	t.Run("failed when grpcClient returns error", func(t *testing.T) {
		// arr
		basicArrange(t)
		forwarderGrpcMock.EXPECT().SendPlayerEvent(gomock.Any(), gomock.Any()).Return(nil, errors.NewErrNotFound("an error occurred"))

		// act
		err := noopForwarderAdapter.ForwardPlayerEvent(context.Background(), newPlayerEventAttributes(), newForwarder())

		// ass
		require.Error(t, err)
		require.NotNil(t, err)
	})

	t.Run("failed when grpcClient returns statusCode not equal 200", func(t *testing.T) {
		// arr
		basicArrange(t)
		forwarderGrpcMock.EXPECT().SendPlayerEvent(gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 404}, nil)

		// act
		err := noopForwarderAdapter.ForwardPlayerEvent(context.Background(), newPlayerEventAttributes(), newForwarder())

		require.Error(t, err)
		require.NotNil(t, err)
	})
}

func newRoomEventAttributes(eventType events.RoomEventType) events.RoomEventAttributes {
	return events.RoomEventAttributes{
		Game:      "game-test",
		RoomId:    "123",
		Host:      "host.com",
		Port:      5050,
		EventType: eventType,
		PingType:  &events.RoomPingReady,
		Attributes: map[string]interface{}{
			"bacon": "delicious",
			"eggs": struct {
				source string
				price  float64
			}{"chicken", 1.75},
			"steak": true,
		},
	}
}

func newPlayerEventAttributes() events.PlayerEventAttributes {
	return events.PlayerEventAttributes{
		RoomId:    "123",
		PlayerId:  "123",
		EventType: events.PlayerLeft,
		Other:     map[string]interface{}{
			"bacon": "delicious",
			"eggs": struct {
				source string
				price  float64
			}{"chicken", 1.75},
			"steak": true,
		},
	}
}

func newForwarder() forwarder.Forwarder {
	return forwarder.Forwarder{
		Name:        "matchmaking",
		Enabled:     true,
		ForwardType: forwarder.TypeGrpc,
		Address:     "matchmaker-service:8080",
		Options: &forwarder.ForwardOptions{
			Timeout: time.Duration(10),
			Metadata: map[string]interface{}{
				"roomType": "red",
				"ping":     true,
			},
		},
	}
}

func basicArrange(t *testing.T){
	mockCtrl = gomock.NewController(t)
	defer mockCtrl.Finish()
	forwarderGrpcMock = mock.NewMockForwarderGrpc(mockCtrl)
	noopForwarderAdapter = NewNoopForwarder(forwarderGrpcMock)
}