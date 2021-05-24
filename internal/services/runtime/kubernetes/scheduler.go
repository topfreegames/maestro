package kubernetes

import (
	"context"
	"fmt"

	"github.com/topfreegames/maestro/internal/entities"
	"github.com/topfreegames/maestro/internal/services/runtime"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
		if errors.IsAlreadyExists(err) {
			return runtime.ErrSchedulerAlreadyExists
		}

		return fmt.Errorf("%w: %s", runtime.ErrUnknown, err)
	}

	return nil
}

func (k *kubernetes) DeleteScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	err := k.clientset.CoreV1().Namespaces().Delete(ctx, scheduler.ID, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return runtime.ErrSchedulerNotFound
		}

		return fmt.Errorf("%w: %s", runtime.ErrUnknown, err)
	}

	return nil
}
