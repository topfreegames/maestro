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

	"github.com/topfreegames/maestro/internal/core/entities/operation"
)

type OperationLeaseStorage interface {
	// Grant takes a lease from an operation. NOTE: only one worker can have the operation lease, meaning that if a GrantLease called for an operation that already has it, it will return an error.
	GrantLease(ctx context.Context, schedulerName, operationID string, initialTTL time.Duration) error
	// RevokeLease frees the operation lease.
	RevokeLease(ctx context.Context, schedulerName, operationID string) error
	// RenewLease renews an active lease adding the specified TTL to it. NOTE: The lease must exist (by calling GrantLease) before renewing it.
	RenewLease(ctx context.Context, schedulerName, operationID string, ttl time.Duration) error
	// FetchLeaseTTL fetches the time the lease will expire given the operation.
	FetchLeaseTTL(ctx context.Context, schedulerName, operationID string) (time.Time, error)
	// ListExpiredLeases returns a list of leases that are expired based on the maxLease, which defines the max time for a lease to be considered expired.
	ListExpiredLeases(ctx context.Context, schedulerName string, maxLease time.Time) ([]operation.OperationLease, error)
}
