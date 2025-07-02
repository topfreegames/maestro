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

package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities"
	"github.com/topfreegames/maestro/internal/core/entities/allocation"
	"github.com/topfreegames/maestro/internal/core/filters"
	"github.com/topfreegames/maestro/internal/core/ports"
	portsErrors "github.com/topfreegames/maestro/internal/core/ports/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	_ ports.SchedulerStorage = (*kubernetesSchedulerStorage)(nil)
)

// CRD Group Version Resource
var schedulerGVR = schema.GroupVersionResource{
	Group:    "maestro.io",
	Version:  "v1",
	Resource: "roomschedulers",
}

type kubernetesSchedulerStorage struct {
	dynamicClient dynamic.Interface
	kubeClient    kubernetes.Interface
	namespace     string
	// Simple transaction support using mutex
	transactionMutex sync.Mutex
	transactions     map[ports.TransactionID]*transaction
}

type transaction struct {
	operations []func() error
}

// NewKubernetesSchedulerStorage creates a new Kubernetes CRD based scheduler storage
func NewKubernetesSchedulerStorage(dynamicClient dynamic.Interface, kubeClient kubernetes.Interface, namespace string) *kubernetesSchedulerStorage {
	if namespace == "" {
		namespace = "default"
	}
	return &kubernetesSchedulerStorage{
		dynamicClient: dynamicClient,
		kubeClient:    kubeClient,
		namespace:     namespace,
		transactions:  make(map[ports.TransactionID]*transaction),
	}
}

func (k *kubernetesSchedulerStorage) GetScheduler(ctx context.Context, name string) (*entities.Scheduler, error) {
	crd, err := k.dynamicClient.Resource(schedulerGVR).Namespace(k.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if isNotFoundError(err) {
			return nil, portsErrors.NewErrNotFound("scheduler %s not found", name)
		}
		return nil, portsErrors.NewErrUnexpected("error getting scheduler %s", name).WithError(err)
	}

	return k.crdToScheduler(crd)
}

func (k *kubernetesSchedulerStorage) GetSchedulerWithFilter(ctx context.Context, schedulerFilter *filters.SchedulerFilter) (*entities.Scheduler, error) {
	if schedulerFilter.Name == "" {
		return nil, portsErrors.NewErrInvalidArgument("Scheduler need at least a name")
	}

	scheduler, err := k.GetScheduler(ctx, schedulerFilter.Name)
	if err != nil {
		return nil, err
	}

	// Check version filter if specified
	if schedulerFilter.Version != "" && scheduler.Spec.Version != schedulerFilter.Version {
		// Check if it's the previous version
		crd, err := k.dynamicClient.Resource(schedulerGVR).Namespace(k.namespace).Get(ctx, schedulerFilter.Name, metav1.GetOptions{})
		if err != nil {
			return nil, portsErrors.NewErrNotFound("scheduler %s not found", schedulerFilter.Name)
		}

		previousVersion, found, err := unstructured.NestedMap(crd.Object, "spec", "previousVersion")
		if err != nil || !found {
			return nil, portsErrors.NewErrNotFound("scheduler %s version %s not found", schedulerFilter.Name, schedulerFilter.Version)
		}

		// Convert previous version to scheduler
		scheduler, err = k.mapToScheduler(previousVersion, schedulerFilter.Name)
		if err != nil {
			return nil, err
		}
		
		if scheduler.Spec.Version != schedulerFilter.Version {
			return nil, portsErrors.NewErrNotFound("scheduler %s version %s not found", schedulerFilter.Name, schedulerFilter.Version)
		}
	}

	return scheduler, nil
}

func (k *kubernetesSchedulerStorage) GetSchedulerVersions(ctx context.Context, name string) ([]*entities.SchedulerVersion, error) {
	crd, err := k.dynamicClient.Resource(schedulerGVR).Namespace(k.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if isNotFoundError(err) {
			return nil, portsErrors.NewErrNotFound("scheduler %s not found", name)
		}
		return nil, portsErrors.NewErrUnexpected("error getting scheduler versions").WithError(err)
	}

	versions := []*entities.SchedulerVersion{}

	// Get current version
	currentVersion, _, _ := unstructured.NestedString(crd.Object, "spec", "spec", "version")
	createdAt, _ := crd.GetCreationTimestamp().MarshalJSON()
	var createdTime time.Time
	json.Unmarshal(createdAt, &createdTime)

	versions = append(versions, &entities.SchedulerVersion{
		Version:   currentVersion,
		IsActive:  true,
		CreatedAt: createdTime,
	})

	// Get previous version if exists
	previousVersion, found, _ := unstructured.NestedMap(crd.Object, "spec", "previousVersion")
	if found {
		prevVersionStr, _ := previousVersion["version"].(string)
		if prevVersionStr != "" {
			// For previous version, we use the last update time as an approximation
			updatedAt, _ := getLastUpdateTime(crd)
			versions = append(versions, &entities.SchedulerVersion{
				Version:   prevVersionStr,
				IsActive:  false,
				CreatedAt: updatedAt.Add(-1 * time.Second), // Slightly before to maintain order
			})
		}
	}

	// Sort by creation time descending
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].CreatedAt.After(versions[j].CreatedAt)
	})

	return versions, nil
}

func (k *kubernetesSchedulerStorage) GetSchedulers(ctx context.Context, names []string) ([]*entities.Scheduler, error) {
	schedulers := make([]*entities.Scheduler, 0, len(names))
	
	for _, name := range names {
		scheduler, err := k.GetScheduler(ctx, name)
		if err != nil {
			// Skip not found errors to match postgres behavior
			if errors.Is(err, portsErrors.ErrNotFound) {
				continue
			}
			return nil, err
		}
		schedulers = append(schedulers, scheduler)
	}
	
	return schedulers, nil
}

func (k *kubernetesSchedulerStorage) GetSchedulersWithFilter(ctx context.Context, schedulerFilter *filters.SchedulerFilter) ([]*entities.Scheduler, error) {
	list, err := k.dynamicClient.Resource(schedulerGVR).Namespace(k.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, portsErrors.NewErrUnexpected("error listing schedulers").WithError(err)
	}

	schedulers := []*entities.Scheduler{}
	for _, item := range list.Items {
		scheduler, err := k.crdToScheduler(&item)
		if err != nil {
			continue
		}

		// Apply filters
		if schedulerFilter != nil {
			if schedulerFilter.Game != "" && scheduler.Game != schedulerFilter.Game {
				continue
			}
			if schedulerFilter.Name != "" && scheduler.Name != schedulerFilter.Name {
				continue
			}
			if schedulerFilter.Version != "" && scheduler.Spec.Version != schedulerFilter.Version {
				continue
			}
		}

		schedulers = append(schedulers, scheduler)
	}

	return schedulers, nil
}

func (k *kubernetesSchedulerStorage) GetAllSchedulers(ctx context.Context) ([]*entities.Scheduler, error) {
	return k.GetSchedulersWithFilter(ctx, nil)
}

func (k *kubernetesSchedulerStorage) CreateScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	crd := k.schedulerToCRD(scheduler)
	
	// Set initial timestamps in status
	now := time.Now()
	unstructured.SetNestedField(crd.Object, now.Format(time.RFC3339), "status", "createdAt")
	unstructured.SetNestedField(crd.Object, now.Format(time.RFC3339), "status", "stateLastChangedAt")
	
	_, err := k.dynamicClient.Resource(schedulerGVR).Namespace(k.namespace).Create(ctx, crd, metav1.CreateOptions{})
	if err != nil {
		if isAlreadyExistsError(err) {
			return portsErrors.NewErrAlreadyExists("error creating scheduler %s: name already exists", scheduler.Name)
		}
		return portsErrors.NewErrUnexpected("error creating scheduler %s", scheduler.Name).WithError(err)
	}
	
	return nil
}

func (k *kubernetesSchedulerStorage) UpdateScheduler(ctx context.Context, scheduler *entities.Scheduler) error {
	// Get existing CRD to preserve previous version
	existing, err := k.dynamicClient.Resource(schedulerGVR).Namespace(k.namespace).Get(ctx, scheduler.Name, metav1.GetOptions{})
	if err != nil {
		if isNotFoundError(err) {
			return portsErrors.NewErrNotFound("scheduler %s not found", scheduler.Name)
		}
		return portsErrors.NewErrUnexpected("error getting scheduler %s", scheduler.Name).WithError(err)
	}

	// Save current version as previous version (n-1)
	currentSpec, _, _ := unstructured.NestedMap(existing.Object, "spec")
	
	// Create new CRD with updated data
	crd := k.schedulerToCRD(scheduler)
	
	// Set the previous version
	unstructured.SetNestedMap(crd.Object, currentSpec, "spec", "previousVersion")
	
	// Preserve status fields
	status, _, _ := unstructured.NestedMap(existing.Object, "status")
	if status != nil {
		unstructured.SetNestedMap(crd.Object, status, "status")
	}
	
	// Update timestamps
	now := time.Now()
	unstructured.SetNestedField(crd.Object, now.Format(time.RFC3339), "status", "updatedAt")
	unstructured.SetNestedField(crd.Object, now.Format(time.RFC3339), "status", "stateLastChangedAt")
	
	// Preserve resource version for update
	crd.SetResourceVersion(existing.GetResourceVersion())
	
	_, err = k.dynamicClient.Resource(schedulerGVR).Namespace(k.namespace).Update(ctx, crd, metav1.UpdateOptions{})
	if err != nil {
		return portsErrors.NewErrUnexpected("error updating scheduler %s", scheduler.Name).WithError(err)
	}
	
	return nil
}

func (k *kubernetesSchedulerStorage) DeleteScheduler(ctx context.Context, transactionID ports.TransactionID, scheduler *entities.Scheduler) error {
	deleteFunc := func() error {
		err := k.dynamicClient.Resource(schedulerGVR).Namespace(k.namespace).Delete(ctx, scheduler.Name, metav1.DeleteOptions{})
		if err != nil && !isNotFoundError(err) {
			return portsErrors.NewErrUnexpected("error deleting scheduler %s", scheduler.Name).WithError(err)
		}
		return nil
	}

	if isInTransactionalContext(transactionID) {
		k.transactionMutex.Lock()
		defer k.transactionMutex.Unlock()
		
		tx, ok := k.transactions[transactionID]
		if !ok {
			return portsErrors.NewErrNotFound("transaction %s not found", transactionID)
		}
		tx.operations = append(tx.operations, deleteFunc)
		return nil
	}

	return deleteFunc()
}

func (k *kubernetesSchedulerStorage) CreateSchedulerVersion(ctx context.Context, transactionID ports.TransactionID, scheduler *entities.Scheduler) error {
	// In Kubernetes CRD storage, versions are handled during update
	// This is a no-op for create since the version is already in the scheduler
	return nil
}

func (k *kubernetesSchedulerStorage) RunWithTransaction(ctx context.Context, transactionFunc func(transactionId ports.TransactionID) error) error {
	k.transactionMutex.Lock()
	transactionID := ports.TransactionID(fmt.Sprintf("tx-%d", time.Now().UnixNano()))
	k.transactions[transactionID] = &transaction{
		operations: []func() error{},
	}
	k.transactionMutex.Unlock()

	// Execute the transaction function
	err := transactionFunc(transactionID)
	
	k.transactionMutex.Lock()
	defer k.transactionMutex.Unlock()
	
	tx, ok := k.transactions[transactionID]
	if !ok {
		return portsErrors.NewErrNotFound("transaction %s not found", transactionID)
	}
	
	// Clean up transaction
	defer delete(k.transactions, transactionID)
	
	if err != nil {
		// Rollback - in this simple implementation, we just discard operations
		return err
	}

	// Commit - execute all operations
	for _, op := range tx.operations {
		if err := op(); err != nil {
			return err
		}
	}
	
	return nil
}

// Helper functions

func (k *kubernetesSchedulerStorage) crdToScheduler(crd *unstructured.Unstructured) (*entities.Scheduler, error) {
	spec, found, err := unstructured.NestedMap(crd.Object, "spec")
	if err != nil || !found {
		return nil, portsErrors.NewErrEncoding("error getting scheduler spec").WithError(err)
	}

	return k.mapToScheduler(spec, crd.GetName())
}

func (k *kubernetesSchedulerStorage) mapToScheduler(spec map[string]interface{}, name string) (*entities.Scheduler, error) {
	// Marshal and unmarshal to convert to scheduler
	specBytes, err := json.Marshal(spec)
	if err != nil {
		return nil, portsErrors.NewErrEncoding("error marshaling scheduler spec").WithError(err)
	}

	var scheduler entities.Scheduler
	if err := json.Unmarshal(specBytes, &scheduler); err != nil {
		return nil, portsErrors.NewErrEncoding("error unmarshaling scheduler").WithError(err)
	}

	// Set name explicitly (it might not be in the spec)
	scheduler.Name = name

	// Set default values if not present
	if scheduler.MatchAllocation == nil {
		scheduler.MatchAllocation = &allocation.MatchAllocation{MaxMatches: 1}
	}

	return &scheduler, nil
}

func (k *kubernetesSchedulerStorage) schedulerToCRD(scheduler *entities.Scheduler) *unstructured.Unstructured {
	// Convert scheduler to map
	schedulerBytes, _ := json.Marshal(scheduler)
	var schedulerMap map[string]interface{}
	json.Unmarshal(schedulerBytes, &schedulerMap)

	// Create CRD structure
	crd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "maestro.io/v1",
			"kind":       "RoomScheduler",
			"metadata": map[string]interface{}{
				"name":      scheduler.Name,
				"namespace": k.namespace,
			},
			"spec":   schedulerMap,
			"status": map[string]interface{}{},
		},
	}

	return crd
}

func isNotFoundError(err error) bool {
	return err != nil && (contains(err.Error(), "not found") || contains(err.Error(), "NotFound"))
}

func isAlreadyExistsError(err error) bool {
	return err != nil && (contains(err.Error(), "already exists") || contains(err.Error(), "AlreadyExists"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr)))
}

func getLastUpdateTime(crd *unstructured.Unstructured) (time.Time, error) {
	updatedAtStr, found, _ := unstructured.NestedString(crd.Object, "status", "updatedAt")
	if found {
		return time.Parse(time.RFC3339, updatedAtStr)
	}
	// Fallback to creation time
	return crd.GetCreationTimestamp().Time, nil
}

func isInTransactionalContext(transactionID ports.TransactionID) bool {
	return transactionID != ""
}