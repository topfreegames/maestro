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

	"testing"

	"github.com/stretchr/testify/require"
	pb "github.com/topfreegames/protos/maestro/grpc/generated"
)

var (
	forwarderClientAdapter *forwarderClient
)

func TestSendRoomEvent(t *testing.T) {
	t.Run("failed when trying to send event", func(t *testing.T) {
		basicArrangeForwarderClient(t)
		event := newRoomEvent()

		response, err := forwarderClientAdapter.SendRoomEvent(context.Background(), newForwarder(), &event)

		require.Nil(t, response)
		require.Error(t, err)
	})
}

func TestSendRoomReSync(t *testing.T) {
	t.Run("failed when trying to send event", func(t *testing.T) {
		basicArrangeForwarderClient(t)
		event := newRoomStatus()

		response, err := forwarderClientAdapter.SendRoomReSync(context.Background(), newForwarder(), &event)

		require.Nil(t, response)
		require.Error(t, err)
	})
}

func TestSendPlayerEvent(t *testing.T) {
	t.Run("failed when trying to send event", func(t *testing.T) {
		basicArrangeForwarderClient(t)
		event := newPlayerEvent()
		response, err := forwarderClientAdapter.SendPlayerEvent(context.Background(), newForwarder(), &event)

		require.Nil(t, response)
		require.Error(t, err)
	})
}

func TestGetGrpcClient(t *testing.T) {
	t.Run("success to get new configuration", func(t *testing.T) {
		basicArrangeForwarderClient(t)

		grpcClient, err := forwarderClientAdapter.getGrpcClient("matchmaker.svc.io")

		require.NotNil(t, grpcClient)
		require.NoError(t, err)
	})

	t.Run("success returning configuration from cache", func(t *testing.T) {
		basicArrangeForwarderClient(t)
		forwarderAddress := "matchmaker.svc.io"
		_, errArrange := forwarderClientAdapter.getGrpcClient(Address(forwarderAddress))
		require.NoError(t, errArrange)

		grpcClient, err := forwarderClientAdapter.getGrpcClient(Address(forwarderAddress))

		require.NotNil(t, grpcClient)
		require.NoError(t, err)
	})

	t.Run("failed when argument is invalid", func(t *testing.T) {
		basicArrangeForwarderClient(t)

		grpcClient, err := forwarderClientAdapter.getGrpcClient("")

		require.Nil(t, grpcClient)
		require.Error(t, err)
	})
}

func TestConfigureGrpcClient(t *testing.T) {

	t.Run("with success", func(t *testing.T) {
		basicArrangeForwarderClient(t)

		grpcClient, err := forwarderClientAdapter.configureGrpcClient("matchmaker.svc.io")

		require.NotNil(t, grpcClient)
		require.NoError(t, err)
	})

	t.Run("failed when argument is invalid", func(t *testing.T) {
		basicArrangeForwarderClient(t)

		grpcClient, err := forwarderClientAdapter.configureGrpcClient("")

		require.Nil(t, grpcClient)
		require.Error(t, err)
	})
}

func TestCacheDelete(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		basicArrangeForwarderClient(t)
		forwarderAddress := "matchmaker.svc.io"
		_, errArrange := forwarderClientAdapter.getGrpcClient(Address(forwarderAddress))
		require.NoError(t, errArrange)

		err := forwarderClientAdapter.CacheDelete(forwarderAddress)

		require.NoError(t, err)
	})

	t.Run("failed when forwarder not found", func(t *testing.T) {
		basicArrangeForwarderClient(t)

		err := forwarderClientAdapter.CacheDelete("matchmaker.svc.io")

		require.Error(t, err)
	})

	t.Run("failed when argument is invalid", func(t *testing.T) {
		basicArrangeForwarderClient(t)

		err := forwarderClientAdapter.CacheDelete("matchmaker.svc.io")

		require.Error(t, err)
	})
}

func basicArrangeForwarderClient(t *testing.T) {
	forwarderClientAdapter = NewForwarderClient()
}

func newRoomEvent() pb.RoomEvent {
	return pb.RoomEvent{
		Room: &pb.Room{
			Game:     "game-name",
			RoomId:   "123",
			Host:     "game.svc.io",
			Port:     9090,
			Metadata: map[string]string{"roomType": "red", "ping": "true"},
		},
		EventType: "grpc",
		Metadata:  map[string]string{"roomType": "red", "ping": "true"},
	}
}

func newRoomStatus() pb.RoomStatus {
	return pb.RoomStatus{
		Room: &pb.Room{
			Game:     "game-name",
			RoomId:   "123",
			Host:     "game.svc.io",
			Port:     9090,
			Metadata: map[string]string{"roomType": "red", "ping": "true"},
		},
		StatusType: pb.RoomStatus_ready,
	}
}

func newPlayerEvent() pb.PlayerEvent {
	return pb.PlayerEvent{
		PlayerId: "123",
		Room: &pb.Room{
			RoomId:   "123",
			Metadata: map[string]string{"roomType": "red", "ping": "true"},
		},
		EventType: pb.PlayerEvent_PLAYER_JOINED,
		Metadata:  map[string]string{"roomType": "red", "ping": "true"},
	}
}
