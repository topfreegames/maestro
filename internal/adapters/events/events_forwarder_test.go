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

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"

	"github.com/topfreegames/maestro/internal/core/ports/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/events"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	pb "github.com/topfreegames/protos/maestro/grpc/generated"
)

func Test_eventsForwarder_ForwardRoomEvent(t *testing.T) {
	type args struct {
		ctx             context.Context
		eventAttributes events.RoomEventAttributes
		forwarder       forwarder.Forwarder
	}
	type mockPreparation func(controller *gomock.Controller) *mock.MockForwarderClient
	arbitraryEventAttributes := newRoomEventAttributes(events.Arbitrary, nil)
	pingEventAttributes := newRoomEventAttributes(events.Ping, nil)
	statusEventAttributes := newRoomEventAttributes(events.Status, nil)
	staticForwarder := newStaticForwarder()

	tests := []struct {
		name            string
		mockPreparation mockPreparation
		args            args
		errWanted       error
		wantedCode      codes.Code
	}{
		{
			"with success when event type is Arbitrary",
			func(controller *gomock.Controller) *mock.MockForwarderClient {
				forwarderClientMock := mock.NewMockForwarderClient(controller)
				requiredEvent := pb.RoomEvent{
					Room: &pb.Room{
						Game:   arbitraryEventAttributes.Game,
						RoomId: arbitraryEventAttributes.RoomId,
						Host:   arbitraryEventAttributes.Host,
						Port:   arbitraryEventAttributes.Port,
						Metadata: map[string]string{
							"roomType":  "red",
							"ping":      "true",
							"roomEvent": "ready",
						},
					},
					EventType: "ready",
				}
				forwarderClientMock.EXPECT().SendRoomEvent(context.Background(), staticForwarder, &requiredEvent).Return(&pb.Response{Code: 200}, nil)
				return forwarderClientMock
			},
			args{
				context.Background(),
				arbitraryEventAttributes,
				staticForwarder,
			},
			nil,
			codes.OK,
		},
		{
			"failed when event type is Arbitrary and roomEvent is not provided",
			func(controller *gomock.Controller) *mock.MockForwarderClient {
				return mock.NewMockForwarderClient(controller)
			},
			args{
				context.Background(),
				newRoomEventAttributes(events.Arbitrary, map[string]interface{}{"roomType": "red", "ping": true}),
				staticForwarder,
			},
			errors.NewErrInvalidArgument("invalid or missing eventAttributes.Other['roomEvent'] field"),
			codes.InvalidArgument,
		},
		{
			"failed when event type is Arbitrary and forwarder client returns error",
			func(controller *gomock.Controller) *mock.MockForwarderClient {
				forwarderClientMock := mock.NewMockForwarderClient(controller)
				requiredEvent := pb.RoomEvent{
					Room: &pb.Room{
						Game:   arbitraryEventAttributes.Game,
						RoomId: arbitraryEventAttributes.RoomId,
						Host:   arbitraryEventAttributes.Host,
						Port:   arbitraryEventAttributes.Port,
						Metadata: map[string]string{
							"roomType":  "red",
							"ping":      "true",
							"roomEvent": "ready",
						},
					},
					EventType: "ready",
				}
				forwarderClientMock.EXPECT().SendRoomEvent(context.Background(), staticForwarder, &requiredEvent).Return(&pb.Response{Code: 2}, errors.NewErrNotFound("an error occurred"))
				return forwarderClientMock
			},
			args{
				context.Background(),
				arbitraryEventAttributes,
				staticForwarder,
			},
			errors.NewErrUnexpected("failed to forward event room at \"matchmaking\" with unknown grpc code: an error occurred"),
			codes.Unknown,
		},
		{
			"succeed when event type is Arbitrary and forwarder client returns status code different than 200",
			func(controller *gomock.Controller) *mock.MockForwarderClient {
				forwarderClientMock := mock.NewMockForwarderClient(controller)
				requiredEvent := pb.RoomEvent{
					Room: &pb.Room{
						Game:   arbitraryEventAttributes.Game,
						RoomId: arbitraryEventAttributes.RoomId,
						Host:   arbitraryEventAttributes.Host,
						Port:   arbitraryEventAttributes.Port,
						Metadata: map[string]string{
							"roomType":  "red",
							"ping":      "true",
							"roomEvent": "ready",
						},
					},
					EventType: "ready",
				}
				forwarderClientMock.EXPECT().SendRoomEvent(context.Background(), staticForwarder, &requiredEvent).Return(&pb.Response{Code: 404}, nil)
				return forwarderClientMock
			},
			args{
				context.Background(),
				newRoomEventAttributes(events.Arbitrary, nil),
				staticForwarder,
			},
			nil,
			codes.NotFound,
		},
		{
			"with success when event type is Ping",
			func(controller *gomock.Controller) *mock.MockForwarderClient {
				forwarderClientMock := mock.NewMockForwarderClient(controller)
				requiredEvent := pb.RoomStatus{
					Room: &pb.Room{
						Game:   pingEventAttributes.Game,
						RoomId: pingEventAttributes.RoomId,
						Host:   pingEventAttributes.Host,
						Port:   pingEventAttributes.Port,
						Metadata: map[string]string{
							"roomType":  "red",
							"ping":      "true",
							"roomEvent": "ready",
						},
						RoomType: "red",
					},
					StatusType: pb.RoomStatus_ready,
				}
				forwarderClientMock.EXPECT().SendRoomReSync(context.Background(), staticForwarder, &requiredEvent).Return(&pb.Response{Code: 200}, nil)
				return forwarderClientMock
			},
			args{
				context.Background(),
				pingEventAttributes,
				staticForwarder,
			},
			nil,
			codes.OK,
		},
		{
			"failed when event type is Ping and forwarder client returns error",
			func(controller *gomock.Controller) *mock.MockForwarderClient {
				forwarderClientMock := mock.NewMockForwarderClient(controller)
				requiredEvent := pb.RoomStatus{
					Room: &pb.Room{
						Game:   pingEventAttributes.Game,
						RoomId: pingEventAttributes.RoomId,
						Host:   pingEventAttributes.Host,
						Port:   pingEventAttributes.Port,
						Metadata: map[string]string{
							"roomType":  "red",
							"ping":      "true",
							"roomEvent": "ready",
						},
						RoomType: "red",
					},
					StatusType: pb.RoomStatus_ready,
				}
				forwarderClientMock.EXPECT().SendRoomReSync(context.Background(), staticForwarder, &requiredEvent).Return(&pb.Response{Code: 2}, errors.NewErrNotFound("an error occurred"))
				return forwarderClientMock
			},
			args{
				context.Background(),
				pingEventAttributes,
				staticForwarder,
			},
			errors.NewErrUnexpected("failed to forward event room at \"matchmaking\" with unknown grpc code: an error occurred"),
			codes.Unknown,
		},
		{
			"succeed when event type is Ping and forwarder client returns status code different than 200",
			func(controller *gomock.Controller) *mock.MockForwarderClient {
				forwarderClientMock := mock.NewMockForwarderClient(controller)
				requiredEvent := pb.RoomStatus{
					Room: &pb.Room{
						Game:   pingEventAttributes.Game,
						RoomId: pingEventAttributes.RoomId,
						Host:   pingEventAttributes.Host,
						Port:   pingEventAttributes.Port,
						Metadata: map[string]string{
							"roomType":  "red",
							"ping":      "true",
							"roomEvent": "ready",
						},
						RoomType: "red",
					},
					StatusType: pb.RoomStatus_ready,
				}
				forwarderClientMock.EXPECT().SendRoomReSync(context.Background(), staticForwarder, &requiredEvent).Return(&pb.Response{Code: 404}, nil)
				return forwarderClientMock
			},
			args{
				context.Background(),
				pingEventAttributes,
				staticForwarder,
			},
			nil,
			codes.NotFound,
		},
		{
			"with success when event type is Status",
			func(controller *gomock.Controller) *mock.MockForwarderClient {
				forwarderClientMock := mock.NewMockForwarderClient(controller)
				requiredEvent := pb.RoomStatus{
					Room: &pb.Room{
						Game:   statusEventAttributes.Game,
						RoomId: statusEventAttributes.RoomId,
						Host:   statusEventAttributes.Host,
						Port:   statusEventAttributes.Port,
						Metadata: map[string]string{
							"roomType":  "red",
							"ping":      "true",
							"roomEvent": "ready",
						},
						RoomType: "red",
					},
					StatusType: pb.RoomStatus_ready,
				}
				forwarderClientMock.EXPECT().SendRoomStatus(context.Background(), staticForwarder, &requiredEvent).Return(&pb.Response{Code: 200}, nil)
				return forwarderClientMock
			},
			args{
				context.Background(),
				statusEventAttributes,
				staticForwarder,
			},
			nil,
			codes.OK,
		},
		{
			"success when event type is Status and forwarder client returns status code different than 200",
			func(controller *gomock.Controller) *mock.MockForwarderClient {
				forwarderClientMock := mock.NewMockForwarderClient(controller)
				requiredEvent := pb.RoomStatus{
					Room: &pb.Room{
						Game:   statusEventAttributes.Game,
						RoomId: statusEventAttributes.RoomId,
						Host:   statusEventAttributes.Host,
						Port:   statusEventAttributes.Port,
						Metadata: map[string]string{
							"roomType":  "red",
							"ping":      "true",
							"roomEvent": "ready",
						},
						RoomType: "red",
					},
					StatusType: pb.RoomStatus_ready,
				}
				forwarderClientMock.EXPECT().SendRoomStatus(context.Background(), staticForwarder, &requiredEvent).Return(&pb.Response{Code: 404}, nil)
				return forwarderClientMock
			},
			args{
				context.Background(),
				statusEventAttributes,
				staticForwarder,
			},
			nil,
			codes.NotFound,
		},
		{
			"succeed when event type is Status and forwarder client returns status code different than 200",
			func(controller *gomock.Controller) *mock.MockForwarderClient {
				forwarderClientMock := mock.NewMockForwarderClient(controller)
				requiredEvent := pb.RoomStatus{
					Room: &pb.Room{
						Game:   statusEventAttributes.Game,
						RoomId: statusEventAttributes.RoomId,
						Host:   statusEventAttributes.Host,
						Port:   statusEventAttributes.Port,
						Metadata: map[string]string{
							"roomType":  "red",
							"ping":      "true",
							"roomEvent": "ready",
						},
						RoomType: "red",
					},
					StatusType: pb.RoomStatus_ready,
				}
				forwarderClientMock.EXPECT().SendRoomStatus(context.Background(), staticForwarder, &requiredEvent).Return(&pb.Response{Code: 500}, nil)
				return forwarderClientMock
			},
			args{
				context.Background(),
				statusEventAttributes,
				staticForwarder,
			},
			nil,
			codes.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			forwarderClientMock := tt.mockPreparation(mockCtrl)
			eventsForwarderAdapter := NewEventsForwarder(forwarderClientMock)

			code, err := eventsForwarderAdapter.ForwardRoomEvent(tt.args.ctx, tt.args.eventAttributes, tt.args.forwarder)

			require.Equal(t, tt.wantedCode, code)
			if tt.errWanted != nil {
				assert.EqualError(t, err, tt.errWanted.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestForwardPlayerEvent(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	t.Run("with success", func(t *testing.T) {
		// arrange
		forwarderClientMock, eventsForwarderAdapter := basicArrange(mockCtrl)
		forwarderClientMock.EXPECT().SendPlayerEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 200}, nil)

		// act
		code, err := eventsForwarderAdapter.ForwardPlayerEvent(context.Background(), newPlayerEventAttributes(), newStaticForwarder())

		// assert
		require.Equal(t, codes.OK, code)
		require.NoError(t, err)
		require.Nil(t, err)
	})

	t.Run("failed when grpcClient returns error", func(t *testing.T) {
		// arrange
		forwarderClientMock, eventsForwarderAdapter := basicArrange(mockCtrl)
		forwarderClientMock.EXPECT().SendPlayerEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 2}, errors.NewErrNotFound("an error occurred"))

		// act
		code, err := eventsForwarderAdapter.ForwardPlayerEvent(context.Background(), newPlayerEventAttributes(), newStaticForwarder())

		// assert
		require.Equal(t, codes.Unknown, code)
		require.Error(t, err)
		require.NotNil(t, err)
	})

	t.Run("succeed when grpcClient returns statusCode not equal 200", func(t *testing.T) {
		// arrange
		forwarderClientMock, eventsForwarderAdapter := basicArrange(mockCtrl)
		forwarderClientMock.EXPECT().SendPlayerEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pb.Response{Code: 404}, nil)

		// act
		code, err := eventsForwarderAdapter.ForwardPlayerEvent(context.Background(), newPlayerEventAttributes(), newStaticForwarder())

		require.Equal(t, codes.NotFound, code)
		require.NoError(t, err)
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
	pingType := events.RoomStatusReady
	other := map[string]interface{}{"roomType": "red", "ping": true, "roomEvent": "ready"}
	if eventMetadata != nil {
		other = eventMetadata
	}

	return events.RoomEventAttributes{
		Game:           "game-test",
		RoomId:         "123",
		Host:           "host.com",
		Port:           5050,
		EventType:      eventType,
		RoomStatusType: &pingType,
		Other:          other,
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
