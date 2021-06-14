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
	ismock "github.com/topfreegames/maestro/internal/adapters/instance_storage/mock"
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
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	fakeClock := clockmock.NewFakeClock(now)
	roomManager := NewRoomManager(fakeClock, portAllocator, roomStorage, instanceStorage, runtime)
	scheduler := entities.Scheduler{
		Name: "game",
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
	runtime.EXPECT().CreateGameRoomInstance(context.Background(), scheduler.Name, game_room.Spec{
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

func TestRoomManager_DeleteRoom(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	roomStorage := rsmock.NewMockRoomStorage(mockCtrl)
	instanceStorage := ismock.NewMockGameRoomInstanceStorage(mockCtrl)
	runtime := runtimemock.NewMockRuntime(mockCtrl)
	roomManager := NewRoomManager(
		clockmock.NewFakeClock(time.Now()),
		pamock.NewMockPortAllocator(mockCtrl),
		roomStorage,
		instanceStorage,
		runtime,
	)

	nextStatus := game_room.GameStatusTerminating
	gameRoom := &game_room.GameRoom{ID: "test-room", SchedulerID: "test-scheduler", Status: game_room.GameStatusReady}
	instance := &game_room.Instance{ID: "test-instance"}
	instanceStorage.EXPECT().GetInstance(context.Background(), gameRoom.SchedulerID, gameRoom.ID).Return(instance, nil)
	roomStorage.EXPECT().SetRoomStatus(context.Background(), gameRoom.SchedulerID, gameRoom.ID, nextStatus).Return(nil)
	runtime.EXPECT().DeleteGameRoomInstance(context.Background(), instance).Return(nil)

	err := roomManager.DeleteRoom(context.Background(), gameRoom)
	require.NoError(t, err)
}
