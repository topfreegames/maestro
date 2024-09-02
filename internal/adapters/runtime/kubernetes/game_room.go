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

package kubernetes

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

func (k *kubernetes) CreateGameRoomInstance(ctx context.Context, scheduler *entities.Scheduler, gameRoomName string, gameRoomSpec game_room.Spec) (*game_room.Instance, error) {
	pod, err := convertGameRoomSpec(*scheduler, gameRoomName, gameRoomSpec)
	if err != nil {
		return nil, errors.NewErrInvalidArgument("invalid game room spec: %s", err)
	}

	pod, err = k.clientSet.CoreV1().Pods(scheduler.Name).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		if kerrors.IsInvalid(err) {
			return nil, errors.NewErrInvalidArgument("invalid game room spec: %s", err)
		}

		return nil, errors.NewErrUnexpected("error create game room instance: %s", err)
	}

	// when pod is created, we cannot have its node, so we're "forcing" node to
	// be nil here.
	instance, err := convertPod(pod, "")
	if err != nil {
		return nil, errors.NewErrUnexpected("failed to convert game room instance: %s", err)
	}

	return instance, nil
}

func (k *kubernetes) DeleteGameRoomInstance(ctx context.Context, gameRoomInstance *game_room.Instance, reason string) error {
	_ = k.createKubernetesEvent(ctx, gameRoomInstance.SchedulerID, gameRoomInstance.ID, reason, "GameRoomDeleted")

	err := k.clientSet.CoreV1().Pods(gameRoomInstance.SchedulerID).Delete(ctx, gameRoomInstance.ID, metav1.DeleteOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return errors.NewErrNotFound("game room '%s' not found", gameRoomInstance.ID)
		}

		return errors.NewErrUnexpected("error deleting game room instance: %s", err)
	}

	return nil
}

func (k *kubernetes) CreateGameRoomName(ctx context.Context, scheduler entities.Scheduler) (string, error) {
	base := scheduler.Name
	const (
		maxNameLength          = 63
		randomLength           = 5
		MaxGeneratedNameLength = maxNameLength - randomLength
	)
	if len(base) > MaxGeneratedNameLength {
		base = base[:MaxGeneratedNameLength]
	}
	return fmt.Sprintf("%s-%s", base, utilrand.String(randomLength)), nil
}

func (k *kubernetes) createKubernetesEvent(ctx context.Context, schedulerID string, gameRoomID string, reason string, message string) error {
	pod, err := k.clientSet.CoreV1().Pods(schedulerID).Get(ctx, gameRoomID, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return errors.NewErrNotFound("game room '%s' not found", gameRoomID)
		}

		return errors.NewErrUnexpected("error getting game room instance: %s", err)
	}

	k.eventRecorder.Event(pod, corev1.EventTypeNormal, reason, message)

	return nil
}
