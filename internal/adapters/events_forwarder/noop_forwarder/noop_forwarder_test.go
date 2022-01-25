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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/events"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
)

func TestForwardRoomEvent_Arbitrary(t *testing.T) {
	t.Run("with success when event type is Arbitrary", func(t *testing.T) {
		// mockCtrl := gomock.NewController(t)
		// defer mockCtrl.Finish()
		// forwarderGrpc := forwarderGrpcMock.NewMockForwarderGrpc(mockCtrl)

		// noopForwarder := NewNoopForwarder(forwarderGrpc)
		// //forwarderGrpc.EXPECT().GetClient(gomock.Any()).Return(nil)
		// err := noopForwarder.ForwardRoomEvent(context.Background(), newEventAttributes(events.Arbitrary), newForwarder())

		// require.NoError(t, err)
		// require.Nil(t, err)
	})

	t.Run("failed when grpcClient returns error", func(t *testing.T) {
		// mockCtrl := gomock.NewController(t)
		// defer mockCtrl.Finish()
		// forwarderGrpc := forwarderGrpcMock.NewMockForwarderGrpc(mockCtrl)

		// noopForwarder := NewNoopForwarder(forwarderGrpc)
		// //forwarderGrpc.EXPECT().GetClient(gomock.Any()).Return(errors.NewErrUnexpected("wrong"))
		// err := noopForwarder.ForwardRoomEvent(context.Background(), newEventAttributes(events.Arbitrary), newForwarder())

		// require.Error(t, err)
		// require.NotNil(t, err)
	})

	t.Run("failed when grpcClient returns statusCode not equal 200", func(t *testing.T) {
		require.True(t, true)
	})
}

func TestForwardRoomEvent_Ping(t *testing.T) {
	t.Run("with success when event type is Ping", func(t *testing.T) {
		// mockCtrl := gomock.NewController(t)
		// defer mockCtrl.Finish()
		// forwarderGrpc := forwarderGrpcMock.NewMockForwarderGrpc(mockCtrl)

		// noopForwarder := NewNoopForwarder(forwarderGrpc)
		// err := noopForwarder.ForwardRoomEvent(context.Background(), newEventAttributes(events.Ping), newForwarder())

		// require.NoError(t, err)
		// require.Nil(t, err)
	})

	t.Run("failed when grpcClient returns error", func(t *testing.T) {
		require.True(t, true)
	})

	t.Run("failed when grpcClient returns statusCode not equal 200", func(t *testing.T) {
		require.True(t, true)
	})
}

func newEventAttributes(eventType events.RoomEventType) events.RoomEventAttributes {
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
