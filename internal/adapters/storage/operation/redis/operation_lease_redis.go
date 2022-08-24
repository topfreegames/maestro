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
	"fmt"
	"time"

	"github.com/topfreegames/maestro/internal/adapters/metrics"

	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/ports/errors"

	"github.com/go-redis/redis/v8"
	"github.com/topfreegames/maestro/internal/core/ports"
)

var _ ports.OperationLeaseStorage = (*redisOperationLeaseStorage)(nil)

const operationLeaseStorageMetricLabel = "operation-lease-storage"

// redisOperationLeaseStorage adapter of the OperationLeaseStorage port. It will
// use a sorted set to store the operation lease
type redisOperationLeaseStorage struct {
	client *redis.Client
	clock  ports.Clock
}

func NewRedisOperationLeaseStorage(client *redis.Client, clock ports.Clock) *redisOperationLeaseStorage {
	return &redisOperationLeaseStorage{client, clock}
}

func (r *redisOperationLeaseStorage) GrantLease(ctx context.Context, schedulerName, operationID string, initialTTL time.Duration) (err error) {
	expireUnixTime := r.clock.Now().Add(initialTTL).Unix()

	alreadyExistsLease, err := r.existsOperationLease(ctx, schedulerName, operationID)
	if err != nil {
		return err
	}

	if alreadyExistsLease {
		return errors.NewErrAlreadyExists("Lease already exists for operation %s on scheduler %s", operationID, schedulerName)
	}

	metrics.RunWithMetrics(operationLeaseStorageMetricLabel, func() error {
		err = r.client.ZAdd(ctx, r.buildSchedulerOperationLeaseKey(schedulerName), &redis.Z{
			Member: operationID,
			Score:  float64(expireUnixTime),
		}).Err()
		return err
	})

	if err != nil {
		return err
	}

	return nil
}

func (r *redisOperationLeaseStorage) RevokeLease(ctx context.Context, schedulerName, operationID string) (err error) {
	existsLease, err := r.existsOperationLease(ctx, schedulerName, operationID)
	if err != nil {
		return err
	}

	if !existsLease {
		return errors.NewErrNotFound("Lease of scheduler \"%s\" and operationId \"%s\" does not exist", schedulerName, operationID)
	}
	metrics.RunWithMetrics(operationLeaseStorageMetricLabel, func() error {
		_, err = r.client.ZRem(ctx, r.buildSchedulerOperationLeaseKey(schedulerName), operationID).Result()
		return err
	})
	if err != nil {
		return errors.NewErrUnexpected("Unexpected error on ZRem function")
	}
	return nil
}

func (r *redisOperationLeaseStorage) RenewLease(ctx context.Context, schedulerName, operationID string, ttl time.Duration) (err error) {
	existsLease, err := r.existsOperationLease(ctx, schedulerName, operationID)
	if err != nil {
		return err
	}

	if !existsLease {
		return errors.NewErrNotFound("Lease of scheduler \"%s\" and operationId \"%s\" does not exist", schedulerName, operationID)
	}
	metrics.RunWithMetrics(operationLeaseStorageMetricLabel, func() error {
		err = r.client.ZIncrBy(ctx, r.buildSchedulerOperationLeaseKey(schedulerName), ttl.Seconds(), operationID).Err()
		return err
	})
	if err != nil {
		return errors.NewErrUnexpected("Unexpected error on incrementing sorted set member score")
	}

	return nil
}

func (r *redisOperationLeaseStorage) FetchLeaseTTL(ctx context.Context, schedulerName, operationID string) (ttl time.Time, err error) {
	var leaseTtl float64
	metrics.RunWithMetrics(operationLeaseStorageMetricLabel, func() error {
		leaseTtl, err = r.client.ZScore(ctx, r.buildSchedulerOperationLeaseKey(schedulerName), operationID).Result()
		return err
	})
	if err != nil {
		if err == redis.Nil {
			return time.Time{}, errors.NewErrNotFound("lease of scheduler \"%s\" and operationId \"%s\" does not exist", schedulerName, operationID)
		}
		return time.Time{}, errors.NewErrUnexpected("failed on fetching ttl for \"%s\" and operationID \"%s\"", schedulerName, operationID).WithError(err)
	}
	ttl = time.Unix(int64(leaseTtl), 0)
	return ttl, err
}

func (r *redisOperationLeaseStorage) FetchOperationsLease(ctx context.Context, schedulerName string, operationIDs ...string) (leases []*operation.OperationLease, err error) {
	var ttls []float64
	if len(operationIDs) == 0 {
		return leases, nil
	}

	metrics.RunWithMetrics(operationLeaseStorageMetricLabel, func() error {
		ttls, err = r.client.ZMScore(ctx, r.buildSchedulerOperationLeaseKey(schedulerName), operationIDs...).Result()
		return err

	})
	if err != nil {
		return []*operation.OperationLease{}, errors.NewErrUnexpected("failed on fetching ttl for \"%s\"", schedulerName).WithError(err)
	}

	for i, opID := range operationIDs {
		if ttls[i] == 0 {
			return []*operation.OperationLease{}, errors.NewErrUnexpected("lease for operation \"%s\" in scheduler \"%s\" does not exist", opID, schedulerName).WithError(err)
		}
		leases = append(leases, &operation.OperationLease{
			OperationID: opID,
			Ttl:         time.Unix(int64(ttls[i]), 0),
		})
	}
	return leases, nil
}

func (r *redisOperationLeaseStorage) ListExpiredLeases(ctx context.Context, schedulerName string, maxLease time.Time) (expiredLeases []operation.OperationLease, err error) {
	var ops []redis.Z
	metrics.RunWithMetrics(operationLeaseStorageMetricLabel, func() error {
		ops, err = r.client.ZRangeByScoreWithScores(ctx, r.buildSchedulerOperationLeaseKey(schedulerName), &redis.ZRangeBy{
			Min: "-inf",
			Max: fmt.Sprintf("%d", maxLease.Unix()),
		}).Result()
		return err
	})

	if err != nil {
		return nil, errors.NewErrUnexpected("failed on listing expired lease for \"%s\"", schedulerName).WithError(err)
	}

	expiredLeases = r.convertToOperationLeaseList(ops)
	return expiredLeases, nil
}

func (r *redisOperationLeaseStorage) convertToOperationLeaseList(ops []redis.Z) []operation.OperationLease {
	var operationsLease []operation.OperationLease
	for _, op := range ops {
		operationsLease = append(operationsLease, operation.OperationLease{
			OperationID: op.Member.(string),
			Ttl:         time.Unix(int64(op.Score), 0),
		})
	}
	return operationsLease
}

func (r *redisOperationLeaseStorage) buildSchedulerOperationLeaseKey(schedulerName string) string {
	return fmt.Sprintf("operations:%s:operationsLease", schedulerName)
}

func (r *redisOperationLeaseStorage) existsOperationLease(ctx context.Context, schedulerName, operationId string) (exists bool, err error) {
	metrics.RunWithMetrics(operationLeaseStorageMetricLabel, func() error {
		_, err = r.client.ZScore(ctx, r.buildSchedulerOperationLeaseKey(schedulerName), operationId).Result()
		return err
	})

	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, errors.NewErrUnexpected("failed on finding operationsLease for \"%s\" and operationID \"%s\"", schedulerName, operationId).WithError(err)
	}
	return true, nil
}
