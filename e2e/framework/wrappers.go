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

package framework

import (
	"testing"

	"github.com/go-redis/redis"
	redisV8 "github.com/go-redis/redis/v8"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"k8s.io/client-go/kubernetes"
)

// TODO: remove the old redis client version
func WithClients(t *testing.T, testCase func(apiClient *APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, redisClientV8 *redisV8.Client, maestro *maestro.MaestroInstance)) {
	client := NewAPIClient(defaultMaestro.ManagementApiServer.Address)

	kubeClient, err := getKubeClient()
	if err != nil {
		t.Fatal(err)
	}
	redisClient, err := getRedisConnection(defaultMaestro.Deps.RedisAddress)
	if err != nil {
		t.Fatal(err)
	}
	redisClientV8, err := getRedisConnectionV8(defaultMaestro.Deps.RedisAddress)
	if err != nil {
		t.Fatal(err)
	}

	testCase(client, kubeClient, redisClient, redisClientV8, defaultMaestro)
}
