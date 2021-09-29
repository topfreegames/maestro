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

package test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/orlangure/gnomock"

	"github.com/go-redis/redis/v8"
	predis "github.com/orlangure/gnomock/preset/redis"
)

var dbNumber int32 = 0

func WithRedisContainer(exec func(redisAddress string)) {
	var err error
	redisContainer, err := gnomock.Start(predis.Preset())
	if err != nil {
		panic(fmt.Sprintf("error creating redis docker instance: %s\n", err))
	}

	exec(redisContainer.DefaultAddress())

	_ = gnomock.Stop(redisContainer)
}

func GetRedisConnection(t *testing.T, redisAddress string) *redis.Client {
	db := atomic.AddInt32(&dbNumber, 1)
	client := redis.NewClient(&redis.Options{
		Addr: redisAddress,
		DB:   int(db),
	})
	t.Cleanup(func() {
		client.FlushDB(context.Background())
	})
	return client
}
