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

package events_forwarder_test

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	efmock "github.com/topfreegames/maestro/internal/adapters/events_forwarder/mock"
	isMock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
	rsMock "github.com/topfreegames/maestro/internal/adapters/room_storage/mock"
	ssMock "github.com/topfreegames/maestro/internal/adapters/scheduler_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/events"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/services/events_forwarder"

	"github.com/golang/mock/gomock"
)

func TestEventsForwarderService_ProduceEvent(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	eventsForwarder := efmock.NewMockEventsForwarder(mockCtrl)
	schedulerStorage := ssMock.NewMockSchedulerStorage(mockCtrl)
	roomStorage := rsMock.NewMockRoomStorage(mockCtrl)
	instanceStorage := isMock.NewMockGameRoomInstanceStorage(mockCtrl)

	eventsForwarderService := events_forwarder.NewEventsForwarderService(eventsForwarder, schedulerStorage, roomStorage, instanceStorage)

	fwd := &forwarder.Forwarder{
		Name:        "fwd",
		Enabled:     true,
		ForwardType: forwarder.TypeGrpc,
		Address:     "address",
		Options: &forwarder.ForwardOptions{
			Timeout:  time.Second * 5,
			Metadata: nil,
		},
	}
	forwarders := []*forwarder.Forwarder{fwd}

	expectedScheduler := &entities.Scheduler{
		Name:            "scheduler",
		Game:            "game",
		State:           "",
		RollbackVersion: "",
		Spec:            game_room.Spec{},
		PortRange:       nil,
		CreatedAt:       time.Time{},
		MaxSurge:        "",
		Forwarders:      forwarders,
	}

	expectedGameRoom := &game_room.GameRoom{
		ID:          "room",
		SchedulerID: "scheduler",
		Status:      game_room.GameStatusPending,
		Metadata: map[string]interface{}{
			"playerId":  "player",
			"eventType": "playerLeft",
		},
	}

	expectedGameRoomInstance := &game_room.Instance{
		ID:          "instance",
		SchedulerID: "scheduler",
	}

	t.Run("when event type is PlayerEvent", func(t *testing.T) {
		event := &events.Event{
			Name:        events.PlayerEvent,
			SchedulerID: expectedScheduler.Name,
			RoomID:      expectedGameRoom.ID,
			Attributes:  nil,
		}

		schedulerStorage.EXPECT().GetScheduler(context.Background(), event.SchedulerID).Return(expectedScheduler, nil)
		roomStorage.EXPECT().GetRoom(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoom, nil)
		instanceStorage.EXPECT().GetInstance(context.Background(), event.SchedulerID, event.RoomID).Return(expectedGameRoomInstance, nil)

		err := eventsForwarderService.ProduceEvent(context.Background(), event)
		require.NoError(t, err)
	})

}
