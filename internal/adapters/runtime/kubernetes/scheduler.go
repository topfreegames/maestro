package kubernetes

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *kubernetes) CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: scheduler.ID,
		},
	}

	_, err := k.clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		if kerrors.IsAlreadyExists(err) {
			return errors.NewErrAlreadyExists("scheduler '%s' already exists", scheduler.ID)
		}

		return errors.NewErrUnexpected("error creating scheduler: %s", err)
	}

	return nil
}

func (k *kubernetes) DeleteScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	err := k.clientset.CoreV1().Namespaces().Delete(ctx, scheduler.ID, metav1.DeleteOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return errors.NewErrNotFound("scheduler '%s' not found", scheduler.ID)
		}

		return errors.NewErrUnexpected("error deleting scheduler: %s", err)
	}

	return nil
}
