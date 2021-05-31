//+build integration

package kubernetes

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube "k8s.io/client-go/kubernetes"

	"github.com/orlangure/gnomock"
	"github.com/orlangure/gnomock/preset/k3s"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/services/runtime"
)

func TestSchedulerCreation(t *testing.T) {
	c, err := gnomock.Start(
		k3s.Preset(k3s.WithVersion("v1.16.15")),
	)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, gnomock.Stop(c))
	}()

	kubeconfig, err := k3s.Config(c)
	require.NoError(t, err)

	ctx := context.Background()
	client, err := kube.NewForConfig(kubeconfig)
	require.NoError(t, err)

	kubernetesRuntime := New(client)
	t.Run("create single scheduler", func(t *testing.T) {
		scheduler := &entities.Scheduler{ID: "single-scheduler-test"}
		err = kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		_, err := client.CoreV1().Namespaces().Get(ctx, scheduler.ID, metav1.GetOptions{})
		require.NoError(t, err)
	})

	t.Run("fail to create scheduler with the same name", func(t *testing.T) {
		scheduler := &entities.Scheduler{ID: "conflict-scheduler-test"}
		err = kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		err = kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.Error(t, err)
		require.ErrorIs(t, err, runtime.ErrSchedulerAlreadyExists)
	})
}

func TestSchedulerDeletion(t *testing.T) {
	c, err := gnomock.Start(
		k3s.Preset(k3s.WithVersion("v1.16.15")),
	)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, gnomock.Stop(c))
	}()

	kubeconfig, err := k3s.Config(c)
	require.NoError(t, err)

	ctx := context.Background()
	client, err := kube.NewForConfig(kubeconfig)
	require.NoError(t, err)

	kubernetesRuntime := New(client)
	t.Run("delete scheduler", func(t *testing.T) {
		scheduler := &entities.Scheduler{ID: "scheduler-test"}
		err = kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		err = kubernetesRuntime.DeleteScheduler(ctx, scheduler)
		require.NoError(t, err)

		ns, err := client.CoreV1().Namespaces().Get(ctx, scheduler.ID, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, v1.NamespaceTerminating, ns.Status.Phase)
	})

	t.Run("fail to delete inexistent scheduler", func(t *testing.T) {
		scheduler := &entities.Scheduler{ID: "conflict-scheduler-test"}
		err = kubernetesRuntime.DeleteScheduler(ctx, scheduler)
		require.Error(t, err)
		require.ErrorIs(t, err, runtime.ErrSchedulerNotFound)
	})
}
