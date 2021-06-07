//+build unit

package room_manager

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/golang/mock/gomock"

	clockmock "github.com/topfreegames/maestro/internal/adapters/clock/mock"
	pamock "github.com/topfreegames/maestro/internal/adapters/port_allocator/mock"
	rsmock "github.com/topfreegames/maestro/internal/adapters/room_storage/mock"
	runtimemock "github.com/topfreegames/maestro/internal/adapters/runtime/mock"
)

func TestRoomManager_CreateRoom(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()
	portAllocator := pamock.NewMockPortAllocator(mockCtrl)
	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	fakeClock := clockmock.NewFakeClock(now)
	roomManager := NewRoomManager(fakeClock, portAllocator, roomStorage, runtime)
	scheduler := entities.Scheduler{
		ID: "game",
		Spec: game_room.Spec{
			Containers: []game_room.Container{
				{
					Name: "container1",
					Ports: []game_room.ContainerPort{
						{Protocol: "tcp"},
					},
				},
				{
					Name: "container2",
					Ports: []game_room.ContainerPort{
						{Protocol: "udp"},
					},
				},
			},
		},
		PortRange: nil,
	}

	portAllocator.EXPECT().Allocate(nil, 2).Return([]int32{5000, 6000}, nil)
	runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.ID, game_room.Spec{
		Containers: []game_room.Container{
			{
				Name: "container1",
				Ports: []game_room.ContainerPort{
					{
						Protocol: "tcp",
						HostPort: 5000,
					},
				},
			},
			{
				Name: "container2",
				Ports: []game_room.ContainerPort{
					{
						Protocol: "udp",
						HostPort: 6000,
					},
				},
			},
		},
	}).Return(&game_room.Instance{
		ID:          "game-1",
		SchedulerID: "game",
		Version:     "1",
	}, nil)
	roomStorage.EXPECT().CreateRoom(context.Background(), &game_room.GameRoom{
		ID:          "game-1",
		SchedulerID: "game",
		Status:      game_room.GameStatusPending,
		LastPingAt:  now,
	})

	room, instance, err := roomManager.CreateRoom(context.Background(), scheduler)
	require.NoError(t, err)
	require.Equal(t, &game_room.GameRoom{
		ID:          "game-1",
		SchedulerID: "game",
		Status:      game_room.GameStatusPending,
		LastPingAt:  now,
	}, room)
	require.Equal(t, &game_room.Instance{
		ID:          "game-1",
		SchedulerID: "game",
		Version:     "1",
	}, instance)
}
