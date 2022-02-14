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

package ports

import (
	"context"
	"time"

	"github.com/topfreegames/maestro/internal/core/operations"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

// Primary ports (input, driving ports)

type OperationManager interface {
	// CreateOperation creates a new operation for the given scheduler and includes it in the execution process.
	CreateOperation(ctx context.Context, schedulerName string, definition operations.Definition) (*operation.Operation, error)
	// GetOperation retrieves the operation and its definition.
	GetOperation(ctx context.Context, schedulerName, operationID string) (*operation.Operation, operations.Definition, error)
	// NextSchedulerOperation returns the next scheduler operation to be processed.
	NextSchedulerOperation(ctx context.Context, schedulerName string) (*operation.Operation, operations.Definition, error)
	// StartOperation used when an operation will start executing.
	StartOperation(ctx context.Context, op *operation.Operation, cancelFunction context.CancelFunc) error
	// FinishOperation used when an operation has finished executing, with error or not.
	FinishOperation(ctx context.Context, op *operation.Operation) error
	// ListSchedulerPendingOperations returns a list of operations with pending status for the given scheduler.
	ListSchedulerPendingOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error)
	// ListSchedulerActiveOperations returns a list of operations with active status for the given scheduler.
	ListSchedulerActiveOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error)
	// ListSchedulerFinishedOperations returns a list of operations with finished status for the given scheduler.
	ListSchedulerFinishedOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error)
	// EnqueueOperationCancellationRequest enqueues a cancellation request for the given operation.
	EnqueueOperationCancellationRequest(ctx context.Context, schedulerName, operationID string) error
	// WatchOperationCancellationRequests starts an async process to watch for cancellation requests for the given operation and cancels it.
	WatchOperationCancellationRequests(ctx context.Context) error
	// GrantLease grants a lease to the given operation.
	GrantLease(ctx context.Context, operation *operation.Operation) error
	// RevokeLease revokes the lease of the given operation.
	RevokeLease(ctx context.Context, operation *operation.Operation) error
	// StartLeaseRenewGoRoutine starts an async process to keep renewing the lease of the given operation.
	StartLeaseRenewGoRoutine(operationCtx context.Context, op *operation.Operation)
}

// Secondary ports (output, driven ports)

type OperationFlow interface {
	InsertOperationID(ctx context.Context, schedulerName, operationID string) error
	// NextOperationID fetches the next scheduler operation to be
	// processed and return its ID.
	NextOperationID(ctx context.Context, schedulerName string) (string, error)
	// ListSchedulerPendingOperationIDs list scheduler pending operation IDs.
	ListSchedulerPendingOperationIDs(ctx context.Context, schedulerName string) ([]string, error)
	// EnqueueOperationCancellationRequest enqueue a operation cancellation request
	EnqueueOperationCancellationRequest(ctx context.Context, request OperationCancellationRequest) error
	// WatchOperationCancellationRequests watches for operation cancellation requests
	WatchOperationCancellationRequests(ctx context.Context) chan OperationCancellationRequest
}

type OperationStorage interface {
	CreateOperation(ctx context.Context, operation *operation.Operation, definitionContent []byte) error
	// GetOperation returns the operation and the definition contents.
	GetOperation(ctx context.Context, schedulerName, operationID string) (*operation.Operation, []byte, error)
	// ListSchedulerActiveOperations list scheduler active operations.
	ListSchedulerActiveOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error)
	// ListSchedulerFinishedOperations list scheduler finished operations.
	ListSchedulerFinishedOperations(ctx context.Context, schedulerName string) ([]*operation.Operation, error)
	// UpdateOperationStatus only updates the operation status for the given
	// operation ID.
	UpdateOperationStatus(ctx context.Context, schedulerName, operationID string, status operation.Status) error
}

type OperationLeaseStorage interface {
	// GrantLease takes a lease from an operation. NOTE: only one worker can have the operation lease, meaning that if a GrantLease called for an operation that already has it, it will return an error.
	GrantLease(ctx context.Context, schedulerName, operationID string, initialTTL time.Duration) error
	// RevokeLease frees the operation lease.
	RevokeLease(ctx context.Context, schedulerName, operationID string) error
	// RenewLease renews an active lease adding the specified TTL to it. NOTE: The lease must exist (by calling GrantLease) before renewing it.
	RenewLease(ctx context.Context, schedulerName, operationID string, ttl time.Duration) error
	// FetchLeaseTTL fetches the time the lease will expire given the operation. Return error if the lease does not exist.
	FetchLeaseTTL(ctx context.Context, schedulerName, operationID string) (time.Time, error)
	// FetchOperationsLease fetches operation lease for each operationId in the function argument. Return error if some lease does not exist.
	FetchOperationsLease(ctx context.Context, schedulerName string, operationIDs ...string) ([]*operation.OperationLease, error)
	// ListExpiredLeases returns a list of leases that are expired based on the maxLease, which defines the max time for a lease to be considered expired.
	ListExpiredLeases(ctx context.Context, schedulerName string, maxLease time.Time) ([]operation.OperationLease, error)
}

// Inner types

type OperationCancellationRequest struct {
	SchedulerName string `json:"schedulerName"`
	OperationID   string `json:"operationID"`
}
