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

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *kubernetes) CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: scheduler.Name,
		},
	}

	_, err := k.clientSet.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		if kerrors.IsAlreadyExists(err) {
			return errors.NewErrAlreadyExists("scheduler '%s' already exists", scheduler.Name)
		}

		return errors.NewErrUnexpected("error creating scheduler: %s", err)
	}

	return nil
}

func (k *kubernetes) DeleteScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	err := k.clientSet.CoreV1().Namespaces().Delete(ctx, scheduler.Name, metav1.DeleteOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return errors.NewErrNotFound("scheduler '%s' not found", scheduler.Name)
		}

		return errors.NewErrUnexpected("error deleting scheduler: %s", err)
	}

	return nil
}
