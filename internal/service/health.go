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

package service

import (
	"context"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/go-redis/redis/v8"
	"github.com/topfreegames/maestro/internal/config"
)

type HealthDependencies struct {
	PostgresDB  *pg.DB
	RedisClient *redis.Client
}

func InitHealthDependencies(c config.Config) *HealthDependencies {
	deps := &HealthDependencies{}

	if pgDB, err := initPostgresForHealth(c); err == nil {
		deps.PostgresDB = pgDB
	}

	if redisClient, err := initRedisForHealth(c); err == nil {
		deps.RedisClient = redisClient
	}

	return deps
}

func initPostgresForHealth(c config.Config) (*pg.DB, error) {
	opts, err := connectToPostgres(c, GetSchedulerStoragePostgresURL(c))
	if err != nil {
		return nil, err
	}
	return pg.Connect(opts), nil
}

func initRedisForHealth(c config.Config) (*redis.Client, error) {
	redisURL := c.GetString(schedulerCacheRedisURLPath)
	if redisURL == "" {
		redisURL = c.GetString(roomStorageRedisURLPath)
	}
	if redisURL == "" {
		redisURL = c.GetString(operationStorageRedisURLPath)
	}

	return createRedisClient(c, redisURL, func(opts *redis.Options) {
		opts.MaxRetries = 3
	})
}

func checkPostgres(ctx context.Context, db *pg.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, "SELECT 1")
	return err
}

func checkRedis(ctx context.Context, client *redis.Client) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return client.Ping(ctx).Err()
}
