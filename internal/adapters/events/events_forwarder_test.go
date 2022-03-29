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

//go:build unit
// +build unit

package events

import (
	"context"
	"testing"
	"time"

	"github.com/topfreegames/maestro/internal/core/ports/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/events"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	pb "github.com/topfreegames/protos/maestro/grpc/generated"
)

func TestForwardRoomEvent_Arbitrary(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("with success when event type is Arbitrary", func(t *testing.T) {
		// arrange
		forwarderClientMock, eventsForwarderAdapter := basicArrange(mockCtrl)
		forwarderClientMock.EXPECT().SendRoomEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 200}, nil)

		// act
		err := eventsForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.Arbitrary, nil), newStaticForwarder())

		// assert
		require.NoError(t, err)
		require.Nil(t, err)
	})

	t.Run("failed when roomEvent is not provided", func(t *testing.T) {
		// arrange
		_, eventsForwarderAdapter := basicArrange(mockCtrl)

		// act
		err := eventsForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.Arbitrary, map[string]interface{}{"roomType": "red", "ping": true}), newStaticForwarder())

		// assert
		require.Error(t, err)
	})

	t.Run("failed when grpcClient returns error", func(t *testing.T) {
		// arrange
		forwarderClientMock, eventsForwarderAdapter := basicArrange(mockCtrl)
		forwarderClientMock.EXPECT().SendRoomEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.NewErrNotFound("an error occurred"))

		// act
		err := eventsForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.Arbitrary, nil), newStaticForwarder())

		// assert
		require.Error(t, err)
		require.NotNil(t, err)
	})

	t.Run("failed when grpcClient returns statusCode not equal 200", func(t *testing.T) {
		forwarderClientMock, eventsForwarderAdapter := basicArrange(mockCtrl)
		forwarderClientMock.EXPECT().SendRoomEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 404}, nil)

		err := eventsForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.Arbitrary, nil), newStaticForwarder())

		require.Error(t, err)
		require.NotNil(t, err)
	})
}

func TestForwardRoomEvent_Ping(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("with success when event type is Ping", func(t *testing.T) {
		// arrange
		forwarderClientMock, eventsForwarderAdapter := basicArrange(mockCtrl)
		forwarderClientMock.EXPECT().SendRoomReSync(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 200}, nil)

		// act
		err := eventsForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.Ping, nil), newStaticForwarder())

		// assert
		require.NoError(t, err)
		require.Nil(t, err)
	})

	t.Run("failed when grpcClient returns error", func(t *testing.T) {
		// arrange
		forwarderClientMock, eventsForwarderAdapter := basicArrange(mockCtrl)
		forwarderClientMock.EXPECT().SendRoomReSync(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.NewErrNotFound("an error occurred"))

		// act
		err := eventsForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.Ping, nil), newStaticForwarder())

		// assert
		require.Error(t, err)
		require.NotNil(t, err)
	})

	t.Run("failed when grpcClient returns statusCode not equal 200", func(t *testing.T) {
		forwarderClientMock, eventsForwarderAdapter := basicArrange(mockCtrl)
		forwarderClientMock.EXPECT().SendRoomReSync(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 404}, nil)

		err := eventsForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.Ping, nil), newStaticForwarder())

		require.Error(t, err)
		require.NotNil(t, err)
	})
}

func TestForwardRoomEvent(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("failed when EventType doesn't exists", func(t *testing.T) {
		// arrange
		_, eventsForwarderAdapter := basicArrange(mockCtrl)

		// act
		err := eventsForwarderAdapter.ForwardRoomEvent(context.Background(), newRoomEventAttributes(events.RoomEventType("Unknown"), nil), newStaticForwarder())

		// assert
		require.Error(t, err)
		require.NotNil(t, err)
	})
}

func TestForwardPlayerEvent(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("with success", func(t *testing.T) {
		// arrange
		forwarderClientMock, eventsForwarderAdapter := basicArrange(mockCtrl)
		forwarderClientMock.EXPECT().SendPlayerEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 200}, nil)

		// act
		err := eventsForwarderAdapter.ForwardPlayerEvent(context.Background(), newPlayerEventAttributes(), newStaticForwarder())

		// assert
		require.NoError(t, err)
		require.Nil(t, err)
	})

	t.Run("failed when grpcClient returns error", func(t *testing.T) {
		// arrange
		forwarderClientMock, eventsForwarderAdapter := basicArrange(mockCtrl)
		forwarderClientMock.EXPECT().SendPlayerEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.NewErrNotFound("an error occurred"))

		// act
		err := eventsForwarderAdapter.ForwardPlayerEvent(context.Background(), newPlayerEventAttributes(), newStaticForwarder())

		// assert
		require.Error(t, err)
		require.NotNil(t, err)
	})

	t.Run("failed when grpcClient returns statusCode not equal 200", func(t *testing.T) {
		// arrange
		forwarderClientMock, eventsForwarderAdapter := basicArrange(mockCtrl)
		forwarderClientMock.EXPECT().SendPlayerEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 404}, nil)

		// act
		err := eventsForwarderAdapter.ForwardPlayerEvent(context.Background(), newPlayerEventAttributes(), newStaticForwarder())

		require.Error(t, err)
		require.NotNil(t, err)
	})
}

func TestMergeInfos(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("with success", func(t *testing.T) {
		_, eventsForwarderAdapter := basicArrange(mockCtrl)
		infoA := map[string]interface{}{"roomType": "red", "ping": true}
		infoB := map[string]interface{}{"gameId": "3123", "roomId": "3123"}
		totalInfos := len(infoA) + len(infoB)

		mergedInfos := eventsForwarderAdapter.mergeInfos(infoA, infoB)
		require.NotNil(t, mergedInfos)
		require.Equal(t, totalInfos, len(mergedInfos))
	})
}

func newRoomEventAttributes(eventType events.RoomEventType, eventMetadata map[string]interface{}) events.RoomEventAttributes {
	pingType := events.RoomPingReady
	other := map[string]interface{}{"roomType": "red", "ping": true, "roomEvent": "ready"}
	if eventMetadata != nil {
		other = eventMetadata
	}

	return events.RoomEventAttributes{
		Game:      "game-test",
		RoomId:    "123",
		Host:      "host.com",
		Port:      5050,
		EventType: eventType,
		PingType:  &pingType,
		Other:     other,
	}
}

func newPlayerEventAttributes() events.PlayerEventAttributes {
	return events.PlayerEventAttributes{
		RoomId:    "123",
		PlayerId:  "123",
		EventType: events.PlayerLeft,
		Other:     map[string]interface{}{"roomType": "red", "ping": true},
	}
}

func newStaticForwarder() forwarder.Forwarder {
	return forwarder.Forwarder{
		Name:        "matchmaking",
		Enabled:     true,
		ForwardType: forwarder.TypeGrpc,
		Address:     "matchmaker-service:8080",
		Options: &forwarder.ForwardOptions{
			Timeout:  time.Duration(10),
			Metadata: map[string]interface{}{"roomType": "red", "ping": true, "roomEvent": "ready"},
		},
	}
}

func basicArrange(controller *gomock.Controller) (forwarderClientMock *mock.MockForwarderClient, eventsForwarderAdapter *eventsForwarder) {
	forwarderClientMock = mock.NewMockForwarderClient(controller)
	eventsForwarderAdapter = NewEventsForwarder(forwarderClientMock)
	return forwarderClientMock, eventsForwarderAdapter
}
