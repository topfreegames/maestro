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

//go:build integration
// +build integration

package kubernetes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/test"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSchedulerCreation(t *testing.T) {
	ctx := context.Background()
	client := test.GetKubernetesClientset(t, kubernetesContainer)
	kubernetesRuntime := New(client)

	t.Run("create single scheduler", func(t *testing.T) {
		scheduler := &entities.Scheduler{Name: "single-scheduler-test"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		_, err = client.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
	})

	t.Run("fail to create scheduler with the same name", func(t *testing.T) {
		scheduler := &entities.Scheduler{Name: "conflict-scheduler-test"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		err = kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.Error(t, err)
		require.ErrorIs(t, err, errors.ErrAlreadyExists)
	})
}

func TestSchedulerDeletion(t *testing.T) {
	ctx := context.Background()
	client := test.GetKubernetesClientset(t, kubernetesContainer)
	kubernetesRuntime := New(client)

	t.Run("delete scheduler", func(t *testing.T) {
		scheduler := &entities.Scheduler{Name: "delete-scheduler-test"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		err = kubernetesRuntime.DeleteScheduler(ctx, scheduler)
		require.NoError(t, err)

		ns, err := client.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, v1.NamespaceTerminating, ns.Status.Phase)
	})

	t.Run("fail to delete inexistent scheduler", func(t *testing.T) {
		scheduler := &entities.Scheduler{Name: "delete-inexistent-scheduler-test"}
		err := kubernetesRuntime.DeleteScheduler(ctx, scheduler)
		require.Error(t, err)
		require.ErrorIs(t, err, errors.ErrNotFound)
	})
}
