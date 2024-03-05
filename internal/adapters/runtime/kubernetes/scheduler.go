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
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	v1Policy "k8s.io/api/policy/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const DisruptionSafetyPercentage float32 = 1.05 // 5% added to roomAmount value for extra safety on disruptions

func (k *kubernetes) createPDBFromScheduler(ctx context.Context, scheduler *entities.Scheduler) (*v1Policy.PodDisruptionBudget, error) {
	if scheduler == nil {
		return nil, errors.NewErrInvalidArgument("scheduler pointer can not be nil")
	}
	pdbSpec := &v1Policy.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name: scheduler.Name,
		},
		Spec: v1Policy.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: int32(0),
			},
		},
	}

	if scheduler.Autoscaling != nil {
		pdbSpec.Spec.MinAvailable = &intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: int32(scheduler.Autoscaling.Min),
		}
	}

	pdb, err := k.clientSet.PolicyV1().PodDisruptionBudgets(scheduler.Name).Create(ctx, pdbSpec, metav1.CreateOptions{})
	if err != nil {
		k.logger.Warn("error creating pdb", zap.String("scheduler", scheduler.Name), zap.Error(err))
		return nil, err
	}

	return pdb, nil
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

	k.createPDBFromScheduler(ctx, scheduler)

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

func (k *kubernetes) MitigateDisruption(
	ctx context.Context,
	scheduler *entities.Scheduler,
	roomAmount int,
	thresholdPercentage float64,
) error {
	if scheduler == nil {
		return errors.NewErrInvalidArgument("empty pointer received for scheduler, can not mitigate disruptions")
	}

	// For kubernetes mitigating disruptions means updating the current PDB
	// minAvailable to the number of occupied rooms if above a threshold
	pdb, err := k.clientSet.PolicyV1().PodDisruptionBudgets(scheduler.Name).Get(ctx, scheduler.Name, metav1.GetOptions{})
	if !kerrors.IsNotFound(err) {
		// Non-recoverable errors
		return errors.NewErrUnexpected("non recoverable error when getting PDB for scheduler '%s': %s", scheduler.Name, err)
	}

	if pdb == nil || kerrors.IsNotFound(err) {
		pdb, err = k.createPDBFromScheduler(ctx, scheduler)
		if err != nil {
			return errors.NewErrUnexpected("error creating PDB for scheduler '%s': %s", scheduler.Name, err)
		}
	}

	currentPdbMinAvailable := pdb.Spec.MinAvailable.IntVal
	if currentPdbMinAvailable == int32(float32(roomAmount)*DisruptionSafetyPercentage) {
		return nil
	}

	pdb.Spec.MinAvailable = &intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: int32(float32(roomAmount) * DisruptionSafetyPercentage),
	}

	_, err = k.clientSet.PolicyV1().PodDisruptionBudgets(scheduler.Name).Update(ctx, pdb, metav1.UpdateOptions{})
	if err != nil {
		return errors.NewErrUnexpected("error updating PDB to mitigate disruptions for scheduler '%s': %s", scheduler.Name, err)
	}

	return nil
}
