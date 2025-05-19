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
	"fmt"
	"os"
	"time"

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/test"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

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
		fmt.Printf("Using gRPC mock address: %s\n", grpcMockAddress)
		fmt.Printf("Using HTTP input mock address: %s\n", httpInputMockAddress)
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
		response, err := forwarderClientAdapter.SendRoomEvent(context.Background(), newForwarder(grpcMockAddress), &event, grpc.WaitForReady(true))
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
		response, err := forwarderClientAdapter.SendRoomEvent(context.Background(), newForwarder(grpcMockAddress), &event, grpc.WaitForReady(true))
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
		response, err := forwarderClientAdapter.SendRoomReSync(context.Background(), newForwarder(grpcMockAddress), &event, grpc.WaitForReady(true))
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
		response, err := forwarderClientAdapter.SendRoomReSync(context.Background(), newForwarder(grpcMockAddress), &event, grpc.WaitForReady(true))
		require.NoError(t, err)
		assert.EqualValues(t, 500, response.Code)
		assert.Equal(t, "Internal server error from matchmaker", response.Message)
	})
}

func TestForwarderClient_SendRoomStatus(t *testing.T) {
	type args struct {
		ctx       context.Context
		forwarder forwarder.Forwarder
		in        pb.RoomStatus
	}
	tests := []struct {
		name            string
		requestStubFile string
		args            args
		want            *pb.Response
		wantErr         assert.ErrorAssertionFunc
	}{
		{
			name:            "return response with code 200 when request succeeds",
			requestStubFile: "../../../test/data/events_mock/events-forwarder-grpc-send-room-status-success.json",
			args: args{
				ctx:       context.Background(),
				forwarder: newForwarder(grpcMockAddress),
				in:        newRoomStatusEvent("65e810a0-bb81-4633-93c9-826414a0062d"),
			},
			want: &pb.Response{
				Code:    200,
				Message: "Event received with success",
			},
			wantErr: assert.NoError,
		},
		{
			name:            "return response with code 500 when request fails",
			requestStubFile: "../../../test/data/events_mock/events-forwarder-grpc-send-room-status-failure.json",
			args: args{
				ctx:       context.Background(),
				forwarder: newForwarder(grpcMockAddress),
				in:        newRoomStatusEvent("f7415c97-5c28-418b-b19b-87380e2d0113"),
			},
			want: &pb.Response{
				Code:    500,
				Message: "Internal server error from matchmaker",
			},
			wantErr: assert.NoError,
		},
		{
			name:            "return error when connection establishment fails",
			requestStubFile: "",
			args: args{
				ctx:       context.Background(),
				forwarder: newForwarder("invalid-address:123"),
				in:        newRoomStatusEvent(""),
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.requestStubFile != "" {
				err := test.AddStubRequestToMockedGrpcServer(
					httpInputMockAddress,
					tt.requestStubFile,
				)
				require.NoError(t, err)
			}

			kac := keepalive.ClientParameters{}
			var dialOptions []grpc.DialOption
			if tt.name != "return error when connection establishment fails" {
				dialOptions = append(dialOptions, grpc.WithConnectParams(grpc.ConnectParams{MinConnectTimeout: 5 * time.Second}))
			}
			f := NewForwarderClient(kac, dialOptions...)

			got, err := f.SendRoomStatus(tt.args.ctx, tt.args.forwarder, &tt.args.in, grpc.WaitForReady(true))

			if !tt.wantErr(t, err, fmt.Sprintf("SendRoomStatus(%v, %v, %v)", tt.args.ctx, tt.args.forwarder, tt.args.in)) {
				return
			}
			if err == nil {
				assert.Equalf(t, tt.want, got, "SendRoomStatus(%v, %v, %v)", tt.args.ctx, tt.args.forwarder, tt.args.in)
			}
		})
	}
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
		response, err := forwarderClientAdapter.SendPlayerEvent(context.Background(), newForwarder(grpcMockAddress), &event, grpc.WaitForReady(true))
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
		response, err := forwarderClientAdapter.SendPlayerEvent(context.Background(), newForwarder(grpcMockAddress), &event, grpc.WaitForReady(true))
		require.NoError(t, err)
		assert.EqualValues(t, 500, response.Code)
		assert.Equal(t, "Internal server error from matchmaker", response.Message)
	})
}

func basicArrangeForwarderClient(t *testing.T) {
	kac := keepalive.ClientParameters{}
	forwarderClientAdapter = NewForwarderClient(kac, grpc.WithConnectParams(grpc.ConnectParams{MinConnectTimeout: 5 * time.Second}))
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
