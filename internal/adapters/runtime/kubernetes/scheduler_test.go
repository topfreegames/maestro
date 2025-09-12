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
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/autoscaling"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	v1 "k8s.io/api/core/v1"
	v1Policy "k8s.io/api/policy/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// randomStr generates a random string of length n.
func randomStr(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func TestSchedulerCreation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kubernetesRuntime := New(clientset, KubernetesConfig{})

	t.Run("create single scheduler", func(t *testing.T) {
		schedulerName := fmt.Sprintf("single-scheduler-test-%s", randomStr(6))
		scheduler := &entities.Scheduler{Name: schedulerName}
		t.Cleanup(func() {
			err := kubernetesRuntime.DeleteScheduler(ctx, scheduler)
			if err != nil && !kerrors.IsNotFound(err) { // Don't fail if already gone
				t.Logf("failed to cleanup scheduler %s: %v", scheduler.Name, err)
			}
			// Ensure the namespace is eventually deleted
			require.Eventually(t, func() bool {
				_, errGet := clientset.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
				return kerrors.IsNotFound(errGet)
			}, 30*time.Second, time.Second, "namespace %s was not deleted after cleanup", scheduler.Name)
		})

		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		ns, err := clientset.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, ns)
	})

	t.Run("fail to create scheduler with the same name", func(t *testing.T) {
		schedulerName := fmt.Sprintf("conflict-scheduler-test-%s", randomStr(6))
		scheduler := &entities.Scheduler{Name: schedulerName}
		t.Cleanup(func() {
			err := kubernetesRuntime.DeleteScheduler(ctx, scheduler)
			if err != nil && !kerrors.IsNotFound(err) {
				t.Logf("failed to cleanup scheduler %s: %v", scheduler.Name, err)
			}
			require.Eventually(t, func() bool {
				_, errGet := clientset.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
				return kerrors.IsNotFound(errGet)
			}, 30*time.Second, time.Second, "namespace %s was not deleted after cleanup", scheduler.Name)
		})

		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)

		err = kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.Error(t, err)
		require.ErrorIs(t, err, errors.ErrAlreadyExists)
	})
}

func TestSchedulerDeletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kubernetesRuntime := New(clientset, KubernetesConfig{})

	t.Run("delete scheduler", func(t *testing.T) {
		schedulerName := fmt.Sprintf("delete-scheduler-test-%s", randomStr(6))
		scheduler := &entities.Scheduler{Name: schedulerName}
		// No t.Cleanup here as we are testing the deletion itself.
		// We create it, then delete it, then check it's gone.

		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err, "failed to create scheduler for deletion test")

		// Ensure it's created before attempting deletion
		_, err = clientset.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err, "namespace for scheduler %s not found after creation", scheduler.Name)

		err = kubernetesRuntime.DeleteScheduler(ctx, scheduler)
		require.NoError(t, err)

		// Check it's terminating
		ns, err := clientset.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
		if err == nil { // It might be already gone, which is fine
			require.Equal(t, v1.NamespaceTerminating, ns.Status.Phase)
		} else {
			require.True(t, kerrors.IsNotFound(err), "expected namespace to be terminating or not found, but got: %v", err)
		}

		// Ensure the namespace is eventually deleted
		require.Eventually(t, func() bool {
			_, errGet := clientset.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
			return kerrors.IsNotFound(errGet)
		}, 30*time.Second, time.Second, "namespace %s was not deleted", scheduler.Name)
	})

	t.Run("fail to delete inexistent scheduler", func(t *testing.T) {
		schedulerName := fmt.Sprintf("delete-inexistent-scheduler-test-%s", randomStr(6))
		scheduler := &entities.Scheduler{Name: schedulerName}
		err := kubernetesRuntime.DeleteScheduler(ctx, scheduler)
		require.Error(t, err)
		require.ErrorIs(t, err, errors.ErrNotFound)
	})
}

func TestPDBCreationAndDeletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kubernetesRuntime := New(clientset, KubernetesConfig{})

	t.Run("create pdb from scheduler without autoscaling", func(t *testing.T) {
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}
		schedulerName := fmt.Sprintf("scheduler-pdb-no-autoscaling-%s", randomStr(6))
		scheduler := &entities.Scheduler{Name: schedulerName}

		t.Cleanup(func() {
			kubernetesRuntime.DeleteScheduler(ctx, scheduler) //nolint:errcheck // ignore error during cleanup
			// Ensure the namespace is eventually deleted
			require.Eventually(t, func() bool {
				_, errGet := clientset.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
				return kerrors.IsNotFound(errGet)
			}, 30*time.Second, time.Second, "namespace %s was not deleted after cleanup", scheduler.Name)
		})

		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err, "failed to create scheduler for PDB test (no autoscaling)")

		pdb, err := clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
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

		err = kubernetesRuntime.DeleteScheduler(ctx, scheduler)
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			_, errPDB := clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
			return kerrors.IsNotFound(errPDB)
		}, 30*time.Second, time.Second, "PDB %s was not deleted after scheduler deletion", scheduler.Name)
	})

	t.Run("pdb should not use scheduler min as minAvailable", func(t *testing.T) {
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}
		schedulerName := fmt.Sprintf("scheduler-pdb-autoscaling-%s", randomStr(6))
		scheduler := &entities.Scheduler{
			Name: schedulerName,
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
		t.Cleanup(func() {
			kubernetesRuntime.DeleteScheduler(ctx, scheduler) //nolint:errcheck
			require.Eventually(t, func() bool {
				_, errGet := clientset.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
				return kerrors.IsNotFound(errGet)
			}, 30*time.Second, time.Second, "namespace %s was not deleted after cleanup", scheduler.Name)
		})

		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err, "failed to create scheduler for PDB test (with autoscaling)")

		pdb, err := clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)
		require.Equal(t, pdb.Spec.MinAvailable.IntVal, int32(0))

		err = kubernetesRuntime.DeleteScheduler(ctx, scheduler)
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			_, errPDB := clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
			return kerrors.IsNotFound(errPDB)
		}, 30*time.Second, time.Second, "PDB %s was not deleted after scheduler deletion", scheduler.Name)
	})

	t.Run("delete pdb on scheduler deletion", func(t *testing.T) {
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}
		schedulerName := fmt.Sprintf("scheduler-pdb-delete-%s", randomStr(6))
		scheduler := &entities.Scheduler{Name: schedulerName}
		// No t.Cleanup here as we are testing the deletion itself.

		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err, "failed to create scheduler for PDB deletion test")

		pdb, err := clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)
		require.Equal(t, pdb.Spec.MinAvailable.IntVal, int32(0))

		err = kubernetesRuntime.DeleteScheduler(ctx, scheduler)
		require.NoError(t, err, "failed to delete scheduler for PDB test")

		// Check PDB is gone
		require.Eventually(t, func() bool {
			_, errPDB := clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
			return kerrors.IsNotFound(errPDB)
		}, 30*time.Second, time.Second, "PDB %s was not deleted after scheduler deletion", scheduler.Name)

		// Ensure the namespace is eventually deleted as well
		require.Eventually(t, func() bool {
			_, errNs := clientset.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
			return kerrors.IsNotFound(errNs)
		}, 30*time.Second, time.Second, "namespace %s was not deleted after PDB test", scheduler.Name)
	})
}

func TestMitigateDisruption(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kubernetesRuntime := New(clientset, KubernetesConfig{})

	t.Run("should not mitigate disruption if scheduler is nil", func(t *testing.T) {
		err := kubernetesRuntime.MitigateDisruption(ctx, nil, 0, 0.0)
		require.ErrorIs(t, errors.ErrInvalidArgument, err)
	})

	t.Run("should create PDB on mitigatation if not created before", func(t *testing.T) {
		t.Parallel() // Ensure parallel execution
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}

		schedulerName := fmt.Sprintf("pdb-mitigation-create-%s", randomStr(6))
		scheduler := &entities.Scheduler{Name: schedulerName}

		// This test creates a namespace manually, then calls MitigateDisruption which should create the PDB.
		namespace := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: scheduler.Name, // Use randomized name
			},
		}
		_, err := clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
		require.NoError(t, err)

		t.Cleanup(func() {
			// Attempt to delete the scheduler, which should handle namespace/PDB cleanup.
			// If namespace was manually created, ensure it's gone.
			kubernetesRuntime.DeleteScheduler(ctx, scheduler) //nolint:errcheck
			require.Eventually(t, func() bool {
				_, errGet := clientset.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
				return kerrors.IsNotFound(errGet)
			}, 30*time.Second, time.Second, "namespace %s was not deleted after cleanup", scheduler.Name)
		})

		err = kubernetesRuntime.MitigateDisruption(ctx, scheduler, 0, 0.0)
		require.NoError(t, err)

		pdb, err := clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)
		require.Equal(t, pdb.Spec.MinAvailable.IntVal, int32(0))
	})

	t.Run("should update PDB on mitigation if not equal to current value", func(t *testing.T) {
		t.Parallel()
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}

		schedulerName := fmt.Sprintf("pdb-mitigation-update-%s", randomStr(6))
		scheduler := &entities.Scheduler{Name: schedulerName}

		// This test creates the scheduler (which creates namespace & initial PDB), then updates PDB via MitigateDisruption.
		err := kubernetesRuntime.CreateScheduler(ctx, scheduler)
		require.NoError(t, err)
		t.Cleanup(func() {
			kubernetesRuntime.DeleteScheduler(ctx, scheduler) //nolint:errcheck
			require.Eventually(t, func() bool {
				_, errGet := clientset.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
				return kerrors.IsNotFound(errGet)
			}, 30*time.Second, time.Second, "namespace %s was not deleted after cleanup", scheduler.Name)
		})

		// Check initial PDB created by CreateScheduler
		pdb, err := clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)
		// Default PDB created by CreateScheduler should have MinAvailable = 0
		require.Equal(t, int32(0), pdb.Spec.MinAvailable.IntVal, "Initial PDB MinAvailable should be 0")

		totalRooms := 100
		err = kubernetesRuntime.MitigateDisruption(ctx, scheduler, totalRooms, 0.0) // This should update the PDB
		require.NoError(t, err)

		pdb, err = clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)

		newRoomAmount := int32(float64(totalRooms) * (1.0 - DefaultDisruptionSafetyPercentage))
		require.Equal(t, newRoomAmount, pdb.Spec.MinAvailable.IntVal)
	})

	t.Run("should default safety percentage if invalid value", func(t *testing.T) {
		t.Parallel()
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}

		schedulerName := fmt.Sprintf("pdb-mitigation-no-update-%s", randomStr(6))
		scheduler := &entities.Scheduler{
			Name: schedulerName,
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
		require.NoError(t, err)
		t.Cleanup(func() {
			kubernetesRuntime.DeleteScheduler(ctx, scheduler) //nolint:errcheck
			require.Eventually(t, func() bool {
				_, errGet := clientset.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
				return kerrors.IsNotFound(errGet)
			}, 30*time.Second, time.Second, "namespace %s was not deleted after cleanup", scheduler.Name)
		})

		// Wait for PDB to be created by CreateScheduler
		var pdb *v1Policy.PodDisruptionBudget
		require.Eventually(t, func() bool {
			pdb, err = clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
			return err == nil && pdb != nil
		}, 10*time.Second, 100*time.Millisecond, "PDB for scheduler %s not found after creation", scheduler.Name)

		require.Equal(t, pdb.Name, scheduler.Name)
		// Initial PDB from CreateScheduler uses MinAvailable 0 if not specified by autoscaling policy for disruption
		require.Equal(t, int32(0), pdb.Spec.MinAvailable.IntVal, "Initial PDB MinAvailable for non-disruption autoscaling policy should be 0")

		newValue := 100
		// MitigateDisruption with invalid safety percentage (-0.1), so it should use default.
		err = kubernetesRuntime.MitigateDisruption(ctx, scheduler, newValue, -0.1)
		require.NoError(t, err)

		pdb, err = clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)

		require.Equal(t, int32(float64(newValue)*(1.0-DefaultDisruptionSafetyPercentage)), pdb.Spec.MinAvailable.IntVal)
	})

	t.Run("should clear maxUnavailable and set minAvailable if existing PDB uses maxUnavailable", func(t *testing.T) {
		t.Parallel() // Ensure parallel execution
		if !kubernetesRuntime.isPDBSupported() {
			t.Log("Kubernetes version does not support PDB, skipping")
			t.SkipNow()
		}

		schedulerName := fmt.Sprintf("pdb-max-unavailable-%s", randomStr(6))
		scheduler := &entities.Scheduler{Name: schedulerName}

		// Manually create namespace and a PDB with MaxUnavailable
		namespace := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: scheduler.Name},
		}
		_, err := clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
		require.NoError(t, err)

		pdbSpec := &v1Policy.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:   scheduler.Name, // Use randomized name
				Labels: map[string]string{"app.kubernetes.io/managed-by": "maestro"},
			},
			Spec: v1Policy.PodDisruptionBudgetSpec{
				MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: int32(10)},
				Selector:       &metav1.LabelSelector{MatchLabels: map[string]string{"maestro-scheduler": scheduler.Name}},
			},
		}
		_, err = clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Create(ctx, pdbSpec, metav1.CreateOptions{})
		require.NoError(t, err)

		t.Cleanup(func() {
			// Attempt to delete the scheduler, which should handle namespace/PDB cleanup.
			kubernetesRuntime.DeleteScheduler(ctx, scheduler) //nolint:errcheck
			require.Eventually(t, func() bool {
				_, errGetNs := clientset.CoreV1().Namespaces().Get(ctx, scheduler.Name, metav1.GetOptions{})
				_, errGetPdb := clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
				return kerrors.IsNotFound(errGetNs) && kerrors.IsNotFound(errGetPdb)
			}, 30*time.Second, time.Second, "namespace/PDB for %s was not deleted after cleanup", scheduler.Name)
		})

		// Now, call MitigateDisruption
		totalRooms := 100
		err = kubernetesRuntime.MitigateDisruption(ctx, scheduler, totalRooms, 0.0)
		require.NoError(t, err)

		// Check that PDB was updated: MinAvailable should be set, MaxUnavailable should be cleared
		pdb, err := clientset.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, pdb)
		require.Equal(t, pdb.Name, scheduler.Name)

		newRoomAmount := int32(float64(totalRooms) * (1.0 - DefaultDisruptionSafetyPercentage))
		require.Equal(t, newRoomAmount, pdb.Spec.MinAvailable.IntVal, "MinAvailable not set correctly")
		require.Nil(t, pdb.Spec.MaxUnavailable, "MaxUnavailable was not cleared")
	})
}
