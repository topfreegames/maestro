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
	"time"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"github.com/topfreegames/maestro/test"
	v1 "k8s.io/api/core/v1"
	v1Policy "k8s.io/api/policy/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestSchedulerCreation(t *testing.T) {
	ctx := context.Background()
	client := test.GetKubernetesClientSet(t, kubernetesContainer)
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
	client := test.GetKubernetesClientSet(t, kubernetesContainer)
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

func TestPDBCreationAndDeletion(t *testing.T) {
	ctx := context.Background()
	client := test.GetKubernetesClientSet(t, kubernetesContainer)
	kubernetesRuntime := New(client)

	t.Run("create pdb from scheduler without autoscaling", func(t *testing.T) {
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}

		scheduler := &entities.Scheduler{Name: "scheduler-pdb-test-no-autoscaling"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		if err != nil {
			require.ErrorIs(t, errors.ErrAlreadyExists, err)
		}

		defer func() {
			err := kubernetesRuntime.DeleteScheduler(ctx, scheduler)
			if err != nil {
				require.ErrorIs(t, errors.ErrNotFound, err)
			}
		}()

		pdb, err := client.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.NotNil(t, pdb.Spec)
		require.NotNil(t, pdb.Spec.Selector)
		require.Equal(t, pdb.Name, scheduler.Name)
		require.Equal(t, pdb.Spec.MinAvailable.IntVal, int32(0))
		require.Contains(t, pdb.Spec.Selector.MatchLabels, "maestro-scheduler")
		require.Contains(t, pdb.Spec.Selector.MatchLabels["maestro-scheduler"], scheduler.Name)
		require.Contains(t, pdb.Labels, "app.kubernetes.io/managed-by")
		require.Contains(t, pdb.Labels["app.kubernetes.io/managed-by"], "maestro")
	})

	t.Run("pdb should not use scheduler min as minAvailable", func(t *testing.T) {
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}

		scheduler := &entities.Scheduler{
			Name: "scheduler-pdb-test-with-autoscaling",
			Autoscaling: &autoscaling.Autoscaling{
				Enabled: true,
				Min:     2,
				Max:     3,
				Policy: autoscaling.Policy{
					Type: autoscaling.RoomOccupancy,
					Parameters: autoscaling.PolicyParameters{
						RoomOccupancy: &autoscaling.RoomOccupancyParams{
							ReadyTarget: 0.1,
						},
					},
				},
			},
		}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		if err != nil {
			require.ErrorIs(t, errors.ErrAlreadyExists, err)
		}

		defer func() {
			err := kubernetesRuntime.DeleteScheduler(ctx, scheduler)
			if err != nil {
				require.ErrorIs(t, errors.ErrNotFound, err)
			}
		}()

		pdb, err := client.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)
		require.Equal(t, pdb.Spec.MinAvailable.IntVal, int32(0))
	})

	t.Run("delete pdb on scheduler deletion", func(t *testing.T) {
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}

		scheduler := &entities.Scheduler{Name: "scheduler-pdb-test-delete"}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		if err != nil {
			require.ErrorIs(t, errors.ErrAlreadyExists, err)
		}

		pdb, err := client.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)
		require.Equal(t, pdb.Spec.MinAvailable.IntVal, int32(0))

		err = kubernetesRuntime.DeleteScheduler(ctx, scheduler)
		if err != nil {
			require.ErrorIs(t, errors.ErrNotFound, err)
		}

		_, err = client.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.True(t, kerrors.IsNotFound(err))
	})
}

func TestMitigateDisruption(t *testing.T) {
	ctx := context.Background()
	client := test.GetKubernetesClientSet(t, kubernetesContainer)
	kubernetesRuntime := New(client)

	t.Run("should not mitigate disruption if scheduler is nil", func(t *testing.T) {
		err := kubernetesRuntime.MitigateDisruption(ctx, nil, 0, 0.0)
		require.ErrorIs(t, errors.ErrInvalidArgument, err)
	})

	t.Run("should create PDB on mitigatation if not created before", func(t *testing.T) {
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}

		scheduler := &entities.Scheduler{Name: "scheduler-pdb-mitigation-create"}
		namespace := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: scheduler.Name,
			},
		}

		_, err := client.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
		require.NoError(t, err)

		err = kubernetesRuntime.MitigateDisruption(ctx, scheduler, 0, 0.0)
		require.NoError(t, err)

		pdb, err := client.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)
		require.Equal(t, pdb.Spec.MinAvailable.IntVal, int32(0))
	})

	t.Run("should update PDB on mitigation if not equal to current value", func(t *testing.T) {
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}

		scheduler := &entities.Scheduler{
			Name: "scheduler-pdb-mitigation-update",
		}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		if err != nil {
			require.ErrorIs(t, errors.ErrAlreadyExists, err)
		}

		defer func() {
			err := kubernetesRuntime.DeleteScheduler(ctx, scheduler)
			if err != nil {
				require.ErrorIs(t, errors.ErrNotFound, err)
			}
		}()

		pdb, err := client.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)
		require.Equal(t, pdb.Spec.MinAvailable.IntVal, int32(0))

		occupiedRooms := 100
		err = kubernetesRuntime.MitigateDisruption(ctx, scheduler, occupiedRooms, 0.0)
		require.NoError(t, err)

		pdb, err = client.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)

		incSafetyPercentage := 1.0 + DefaultDisruptionSafetyPercentage
		newRoomAmount := int32(float64(occupiedRooms) * incSafetyPercentage)
		require.Equal(t, pdb.Spec.MinAvailable.IntVal, newRoomAmount)
	})

	t.Run("should default safety percentage if invalid value", func(t *testing.T) {
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}

		scheduler := &entities.Scheduler{
			Name: "scheduler-pdb-mitigation-no-update",
			Autoscaling: &autoscaling.Autoscaling{
				Enabled: true,
				Min:     100,
				Max:     200,
				Policy: autoscaling.Policy{
					Type: autoscaling.RoomOccupancy,
					Parameters: autoscaling.PolicyParameters{
						RoomOccupancy: &autoscaling.RoomOccupancyParams{
							ReadyTarget: 0.1,
						},
					},
				},
			},
		}
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		if err != nil {
			require.ErrorIs(t, errors.ErrAlreadyExists, err)
		}

		defer func() {
			err := kubernetesRuntime.DeleteScheduler(ctx, scheduler)
			if err != nil {
				require.ErrorIs(t, errors.ErrNotFound, err)
			}
		}()

		pdb, err := client.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)
		require.Equal(t, pdb.Spec.MinAvailable.IntVal, int32(0))

		time.Sleep(time.Millisecond * 100)
		newValue := 100
		err = kubernetesRuntime.MitigateDisruption(ctx, scheduler, newValue, 0.0)
		require.NoError(t, err)

		time.Sleep(time.Millisecond * 100)
		pdb, err = client.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)

		incSafetyPercentage := 1.0 + DefaultDisruptionSafetyPercentage
		require.Equal(t, int32(float64(newValue)*incSafetyPercentage), pdb.Spec.MinAvailable.IntVal)
	})

	t.Run("should clear maxUnavailable and set minAvailable if existing PDB uses maxUnavailable", func(t *testing.T) {
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}
		scheduler := &entities.Scheduler{
			Name: "scheduler-pdb-max-unavailable",
		}
		pdbSpec := &v1Policy.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name: scheduler.Name,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "maestro",
				},
			},
			Spec: v1Policy.PodDisruptionBudgetSpec{
				MaxUnavailable: &intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: int32(10),
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"maestro-scheduler": scheduler.Name,
					},
				},
			},
		}
		namespace := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: scheduler.Name,
			},
		}

		_, err := client.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
		require.NoError(t, err)

		pdb, err := client.PolicyV1().PodDisruptionBudgets(scheduler.Name).Create(ctx, pdbSpec, metav1.CreateOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Spec.MaxUnavailable.IntVal, int32(10))

		occupiedRooms := 100
		err = kubernetesRuntime.MitigateDisruption(ctx, scheduler, occupiedRooms, 0.0)
		require.NoError(t, err)

		pdb, err = client.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)

		incSafetyPercentage := 1.0 + DefaultDisruptionSafetyPercentage
		newRoomAmount := int32(float64(occupiedRooms) * incSafetyPercentage)
		require.Equal(t, pdb.Spec.MinAvailable.IntVal, newRoomAmount)
		require.Nil(t, pdb.Spec.MaxUnavailable)

	})
}
