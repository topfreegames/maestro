package kubernetes

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"

	"github.com/topfreegames/maestro/internal/adapters/runtime"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *kubernetes) CreateGameRoom(ctx context.Context, gameRoom *game_room.GameRoom, gameRoomSpec game_room.Spec) error {
	pod, err := convertGameRoomSpec(gameRoom.Scheduler, gameRoomSpec)
	if err != nil {
		return runtime.NewErrGameRoomConversion(err)
	}

	pod, err = k.clientset.CoreV1().Pods(gameRoom.Scheduler.ID).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		if errors.IsInvalid(err) {
			return runtime.NewErrInvalidGameRoomSpec(gameRoom.ID, err)
		}

		return runtime.NewErrUnknown(err)
	}

	gameRoom.ID = pod.ObjectMeta.Name
	return nil
}

func (k *kubernetes) DeleteGameRoom(ctx context.Context, gameRoom *game_room.GameRoom) error {
	err := k.clientset.CoreV1().Pods(gameRoom.Scheduler.ID).Delete(ctx, gameRoom.ID, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return runtime.NewErrGameRoomNotFound(gameRoom.ID)
		}

		return runtime.NewErrUnknown(err)
	}

	return nil
}
