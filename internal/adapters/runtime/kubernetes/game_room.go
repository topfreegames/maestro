package kubernetes

import (
	"context"

	"github.com/topfreegames/maestro/internal/adapters/runtime"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *kubernetes) CreateGameRoomInstance(ctx context.Context, schedulerID string, gameRoomSpec game_room.Spec) (*game_room.Instance, error) {
	pod, err := convertGameRoomSpec(schedulerID, gameRoomSpec)
	if err != nil {
		return nil, runtime.NewErrGameRoomConversion(err)
	}

	pod, err = k.clientset.CoreV1().Pods(schedulerID).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		if kerrors.IsInvalid(err) {
			return nil, errors.NewErrInvalidArgument("invalid game room spec: %s", err)
		}

		return nil, errors.NewErrUnexpected("error create game room instance: %s", err)
	}

	return convertPod(pod), nil
}

func (k *kubernetes) DeleteGameRoomInstance(ctx context.Context, gameRoomInstance *game_room.Instance) error {
	err := k.clientset.CoreV1().Pods(gameRoomInstance.SchedulerID).Delete(ctx, gameRoomInstance.ID, metav1.DeleteOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return errors.NewErrNotFound("game room '%s' already exists", gameRoomInstance.ID)
		}

		return errors.NewErrUnexpected("error deleting game room instance: %s", err)
	}

	return nil
}
