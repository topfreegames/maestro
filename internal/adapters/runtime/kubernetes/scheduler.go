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
	"strconv"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/logs"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	v1Policy "k8s.io/api/policy/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
)

const (
	DefaultDisruptionSafetyPercentage float64 = 0.05
	MajorKubeVersionPDB               int     = 1
	MinorKubeVersionPDB               int     = 21
)

func (k *kubernetes) isPDBSupported() bool {
	// Check based on the kube version of the clientSet if PDBs are supported (1.21+)
	version, err := k.clientSet.Discovery().ServerVersion()
	if err != nil {
		k.logger.Warn("Could not get kube API version, can not check for PDB support", zap.Error(err))
		return false
	}
	major, err := strconv.Atoi(version.Major)
	if err != nil {
		k.logger.Warn(
			"Could not convert major kube API version to int, can not check for PDB support",
			zap.String("majorKubeAPIVersion", version.Major),
		)
		return false
	}
	if major < MajorKubeVersionPDB {
		k.logger.Warn(
			"Can not create PDB for this kube API version",
			zap.Int("majorKubeAPIVersion", major),
			zap.Int("majorPDBVersionRequired", MajorKubeVersionPDB),
		)
		return false
	}
	minor, err := strconv.Atoi(version.Minor)
	if err != nil {
		k.logger.Warn(
			"Could not convert minor kube API version to int, can not check for PDB support",
			zap.String("minorKubeAPIVersion", version.Minor),
		)
		return false
	}
	if minor < MinorKubeVersionPDB {
		k.logger.Warn(
			"Can not create PDB for this kube API version",
			zap.Int("minorKubeAPIVersion", minor),
			zap.Int("minorPDBVersionRequired", MinorKubeVersionPDB),
		)
		return false
	}
	return true
}

func (k *kubernetes) createPDBFromScheduler(ctx context.Context, scheduler *entities.Scheduler) (*v1Policy.PodDisruptionBudget, error) {
	if scheduler == nil {
		return nil, errors.NewErrInvalidArgument("scheduler pointer can not be nil")
	}
	pdbSpec := &v1Policy.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name: scheduler.Name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "maestro",
			},
		},
		Spec: v1Policy.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: int32(0),
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"maestro-scheduler": scheduler.Name,
				},
			},
		},
	}

	pdb, err := k.clientSet.PolicyV1().PodDisruptionBudgets(scheduler.Name).Create(ctx, pdbSpec, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		k.logger.Warn("error creating pdb", zap.String("scheduler", scheduler.Name), zap.Error(err))
		return nil, err
	}

	return pdb, nil
}

func (k *kubernetes) deletePDBFromScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	if scheduler == nil {
		return errors.NewErrInvalidArgument("scheduler pointer can not be nil")
	}
	if !k.isPDBSupported() {
		return errors.NewErrUnexpected("PDBs are not supported for this kube API version")
	}
	err := k.clientSet.PolicyV1().PodDisruptionBudgets(scheduler.Name).Delete(ctx, scheduler.Name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		k.logger.Warn("error deleting pdb", zap.String("scheduler", scheduler.Name), zap.Error(err))
		return err
	}
	return nil
}

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

	_, err = k.createPDBFromScheduler(ctx, scheduler)
	if err != nil {
		k.logger.Warn("PDB Creation during scheduler creation failed", zap.String("scheduler", scheduler.Name), zap.Error(err))
	}

	return nil
}

func (k *kubernetes) DeleteScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	err := k.deletePDBFromScheduler(ctx, scheduler)
	if err != nil {
		k.logger.Warn("PDB Deletion during scheduler deletion failed", zap.String("scheduler", scheduler.Name), zap.Error(err))
	}
	err = k.clientSet.CoreV1().Namespaces().Delete(ctx, scheduler.Name, metav1.DeleteOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return errors.NewErrNotFound("scheduler '%s' not found", scheduler.Name)
		}

		return errors.NewErrUnexpected("error deleting scheduler: %s", err)
	}

	return nil
}

func (k *kubernetes) MitigateDisruption(
	ctx context.Context,
	scheduler *entities.Scheduler,
	roomAmount int,
	safetyPercentage float64,
) error {
	if scheduler == nil {
		return errors.NewErrInvalidArgument("empty pointer received for scheduler, can not mitigate disruptions")
	}

	incSafetyPercentage := 1.0
	if safetyPercentage < DefaultDisruptionSafetyPercentage {
		k.logger.Warn(
			"invalid safety percentage, using default percentage",
			zap.Float64("safetyPercentage", safetyPercentage),
			zap.Float64("DefaultDisruptionSafetyPercentage", DefaultDisruptionSafetyPercentage),
		)
		safetyPercentage = DefaultDisruptionSafetyPercentage
	}
	incSafetyPercentage += safetyPercentage

	// Use RetryOnConflict to handle potential updates to the PDB object by other processes.
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// First, ensure the PDB exists, creating it if necessary.
		pdb, getErr := k.clientSet.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
		if getErr != nil {
			if kerrors.IsNotFound(getErr) {
				// PDB doesn't exist, try to create it.
				createdPdb, createErr := k.createPDBFromScheduler(ctx, scheduler)
				if createErr != nil {
					// If creation fails (and it's not because it now exists), return the error to stop retrying.
					return errors.NewErrUnexpected(
						"error creating PDB for scheduler '%s' during retry: %v",
						scheduler.Name,
						createErr,
					).WithError(createErr)
				}
				pdb = createdPdb // Use the newly created PDB for the rest of the logic in this attempt.
			} else {
				// Another error occurred while getting the PDB, return it to stop retrying.
				return errors.NewErrUnexpected(
					"non recoverable error when getting PDB for scheduler '%s' during retry: %v",
					scheduler.Name,
					getErr,
				).WithError(getErr)
			}
		}

		// At this point, pdb should be non-nil (either fetched or created).
		if pdb == nil { // Should not happen if logic above is correct
			return errors.NewErrUnexpected("PDB is nil for scheduler '%s' after get/create attempt in retry", scheduler.Name)
		}

		var currentPdbMinAvailable int32
		if pdb.Spec.MinAvailable != nil {
			currentPdbMinAvailable = pdb.Spec.MinAvailable.IntVal
		}

		desiredMinAvailable := int32(float64(roomAmount) * incSafetyPercentage)

		// If current PDB already matches the desired state regarding MinAvailable and MaxUnavailable being nil,
		// no update is needed.
		if currentPdbMinAvailable == desiredMinAvailable && pdb.Spec.MaxUnavailable == nil {
			k.logger.Info("PDB already in desired state", zap.String(logs.LogFieldSchedulerName, scheduler.Name))
			return nil // No update needed, success.
		}

		// Prepare the updated PDB spec.
		pdb.Spec = v1Policy.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: desiredMinAvailable,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"maestro-scheduler": scheduler.Name,
				},
			},
		}

		_, updateErr := k.clientSet.PolicyV1().PodDisruptionBudgets(scheduler.Name).Update(ctx, pdb, metav1.UpdateOptions{})
		// updateErr will be checked by RetryOnConflict. If it's a conflict, it retries.
		// If it's another error, RetryOnConflict will return it.
		return updateErr
	})

	if err != nil {
		return errors.NewErrUnexpected(
			"error updating PDB to mitigate disruptions for scheduler '%s': %v",
			scheduler.Name,
			err,
		).WithError(err)
	}

	return nil
}
