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

package redis

import (
	"context"
	"github.com/topfreegames/maestro/internal/core/ports/errors"
	"time"

	"github.com/topfreegames/maestro/internal/core/entities/operation"

	"github.com/go-redis/redis/v8"
	"github.com/topfreegames/maestro/internal/core/ports"
)

var _ ports.OperationLeaseStorage = (*redisOperationLeaseStorage)(nil)

// redisOperationLeaseStorage adapter of the OperationLeaseStorage port. It will
// use a sorted set to store the operation lease
type redisOperationLeaseStorage struct {
	client *redis.Client
	clock  ports.Clock
}

func NewRedisOperationStorage(client *redis.Client, clock ports.Clock) *redisOperationLeaseStorage {
	return &redisOperationLeaseStorage{client, clock}
}

func (r *redisOperationLeaseStorage) GrantLease(ctx context.Context, schedulerName, operationID string, initialTTL time.Duration) error {
	return nil
}

func (r *redisOperationLeaseStorage) RevokeLease(ctx context.Context, schedulerName, operationID string) error {
	//TODO(caio.rodrigues): rebase with master after GrantLease merge request for functions "existsOperationLease" and "buildSchedulerOperationLeaseKey"
	existsLease, err := r.existsOperationLease(ctx, schedulerName, operationID)
	if err != nil {
		return err
	}

	if !existsLease {
		return errors.NewErrNotFound("Lease of scheduler \"%s\" and operationId \"%s\" trying to be revoked does not exist", schedulerName, operationID)
	}

	_, err = r.client.ZRem(context.Background(), r.buildSchedulerOperationLeaseKey(schedulerName), operationID).Result()
	if err != nil {
		return errors.NewErrUnexpected("Unexpected error on ZRem function")
	}
}

func (r *redisOperationLeaseStorage) RenewLease(ctx context.Context, schedulerName, operationID string, ttl time.Duration) error {
	return nil
}

func (r *redisOperationLeaseStorage) FetchLeaseTTL(ctx context.Context, schedulerName, operationID string) (time.Time, error) {
	return time.Time{}, nil
}

func (r *redisOperationLeaseStorage) ListExpiredLeases(ctx context.Context, schedulerName string, maxLease time.Time) ([]operation.OperationLease, error) {
	return nil, nil
}
