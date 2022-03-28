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

//go:build integration
// +build integration

package events

import (
	"context"
	"os"
	"time"

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/test"
	pb "github.com/topfreegames/protos/maestro/grpc/generated"
)

var (
	forwarderClientAdapter *ForwarderClient
	grpcMockAddress        string
	httpInputMockAddress   string
)

func TestMain(m *testing.M) {
	var code int
	test.WithGrpcMockContainer(func(grpcAddress, httpInputAddress string) {
		grpcMockAddress = grpcAddress
		httpInputMockAddress = httpInputAddress
		code = m.Run()
	})
	os.Exit(code)
}

func TestSendRoomEvent(t *testing.T) {
	t.Run("success case should return response", func(t *testing.T) {
		err := test.AddStubRequestToMockedGrpcServer(
			httpInputMockAddress,
			"../../../test/data/events_mock/events-forwarder-grpc-send-room-event-success.json",
		)
		require.NoError(t, err)

		basicArrangeForwarderClient(t)
		event := newRoomEvent("cbfef643-4fed-4ca9-94f8-9ad424fd5624")

		response, err := forwarderClientAdapter.SendRoomEvent(context.Background(), newForwarder(grpcMockAddress), &event)
		require.NoError(t, err)

		assert.EqualValues(t, 200, response.Code)
		assert.Equal(t, "Event received with success", response.Message)
	})

	t.Run("failed when trying to send event", func(t *testing.T) {
		err := test.AddStubRequestToMockedGrpcServer(
			httpInputMockAddress,
			"../../../test/data/events_mock/events-forwarder-grpc-send-room-event-failure.json",
		)
		require.NoError(t, err)

		basicArrangeForwarderClient(t)
		event := newRoomEvent("d85b0160-2522-494d-9efe-79a6a3612849")

		response, err := forwarderClientAdapter.SendRoomEvent(context.Background(), newForwarder(grpcMockAddress), &event)
		require.NoError(t, err)

		assert.EqualValues(t, 500, response.Code)
		assert.Equal(t, "Internal server error from matchmaker", response.Message)
	})
}

func TestSendRoomReSync(t *testing.T) {
	t.Run("success case should return response", func(t *testing.T) {
		err := test.AddStubRequestToMockedGrpcServer(
			httpInputMockAddress,
			"../../../test/data/events_mock/events-forwarder-grpc-send-room-resync-success.json",
		)
		require.NoError(t, err)

		basicArrangeForwarderClient(t)
		event := newRoomStatusEvent("65e810a0-bb81-4633-93c9-826414a0062d")

		response, err := forwarderClientAdapter.SendRoomReSync(context.Background(), newForwarder(grpcMockAddress), &event)
		require.NoError(t, err)

		assert.EqualValues(t, 200, response.Code)
		assert.Equal(t, "Event received with success", response.Message)
	})

	t.Run("failed when trying to send event", func(t *testing.T) {
		err := test.AddStubRequestToMockedGrpcServer(
			httpInputMockAddress,
			"../../../test/data/events_mock/events-forwarder-grpc-send-room-resync-failure.json",
		)
		require.NoError(t, err)

		basicArrangeForwarderClient(t)
		event := newRoomStatusEvent("f7415c97-5c28-418b-b19b-87380e2d0113")

		response, err := forwarderClientAdapter.SendRoomReSync(context.Background(), newForwarder(grpcMockAddress), &event)
		require.NoError(t, err)

		assert.EqualValues(t, 500, response.Code)
		assert.Equal(t, "Internal server error from matchmaker", response.Message)
	})
}

func TestSendPlayerEvent(t *testing.T) {
	t.Run("success case should return response", func(t *testing.T) {
		err := test.AddStubRequestToMockedGrpcServer(
			httpInputMockAddress,
			"../../../test/data/events_mock/events-forwarder-grpc-send-player-event-success.json",
		)
		require.NoError(t, err)

		basicArrangeForwarderClient(t)
		event := newPlayerEvent("c50acc91-4d88-46fa-aa56-48d63c5b5311")

		response, err := forwarderClientAdapter.SendPlayerEvent(context.Background(), newForwarder(grpcMockAddress), &event)
		require.NoError(t, err)

		assert.EqualValues(t, 200, response.Code)
		assert.Equal(t, "Event received with success", response.Message)
	})

	t.Run("failed when trying to send event", func(t *testing.T) {
		err := test.AddStubRequestToMockedGrpcServer(
			httpInputMockAddress,
			"../../../test/data/events_mock/events-forwarder-grpc-send-player-event-failure.json",
		)
		require.NoError(t, err)

		basicArrangeForwarderClient(t)
		event := newPlayerEvent("446bb3d0-0334-4468-a4e7-8068a97caa53")

		response, err := forwarderClientAdapter.SendPlayerEvent(context.Background(), newForwarder(grpcMockAddress), &event)
		require.NoError(t, err)

		assert.EqualValues(t, 500, response.Code)
		assert.Equal(t, "Internal server error from matchmaker", response.Message)
	})
}

func basicArrangeForwarderClient(t *testing.T) {
	forwarderClientAdapter = NewForwarderClient()
}

func newRoomEvent(mockIdentifier string) pb.RoomEvent {
	return pb.RoomEvent{
		Room: &pb.Room{
			Game:     "game-name",
			RoomId:   "123",
			Host:     "game.svc.io",
			Port:     9090,
			Metadata: map[string]string{"mockIdentifier": mockIdentifier},
		},
		EventType: "roomReady",
		Metadata:  map[string]string{"roomType": "red", "ping": "true"},
	}
}

func newForwarder(address string) forwarder.Forwarder {
	return forwarder.Forwarder{
		Name:        "matchmaking",
		Enabled:     true,
		ForwardType: forwarder.TypeGrpc,
		Address:     address,
		Options: &forwarder.ForwardOptions{
			Timeout:  time.Duration(1000),
			Metadata: map[string]interface{}{"roomType": "red", "ping": true, "roomEvent": "ready"},
		},
	}
}

func newRoomStatusEvent(mockIdentifier string) pb.RoomStatus {
	return pb.RoomStatus{
		Room: &pb.Room{
			Game:     "game-name",
			RoomId:   "123",
			Host:     "game.svc.io",
			Port:     9090,
			Metadata: map[string]string{"mockIdentifier": mockIdentifier},
		},
		StatusType: pb.RoomStatus_ready,
	}
}

func newPlayerEvent(playerID string) pb.PlayerEvent {
	return pb.PlayerEvent{
		PlayerId: playerID,
		Room: &pb.Room{
			RoomId:   "123",
			Metadata: map[string]string{"roomType": "red", "ping": "true"},
		},
		EventType: pb.PlayerEvent_PLAYER_JOINED,
		Metadata:  map[string]string{"roomType": "red", "ping": "true"},
	}
}
