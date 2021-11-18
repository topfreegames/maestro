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

package management

import (
	"fmt"
	"testing"

	"github.com/go-redis/redis/v8"

	"github.com/stretchr/testify/require"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
)

func TestGetScheduler(t *testing.T) {
	framework.WithClients(t, func(apiClient *framework.APIClient, kubeclient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		t.Run("Should Succeed - Get scheduler without query parameter version", func(t *testing.T) {


			schedulerName, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				apiClient,
				kubeclient,
				[]string{"sh", "-c", "tail -f /dev/null"},
			)

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s", schedulerName), getSchedulerRequest, getSchedulerResponse)

			require.NoError(t, err)
			require.Equal(t, getSchedulerResponse.Scheduler.Name, schedulerName)
		})

		t.Run("Should Fail - Get scheduler 404 with non-existent query version", func(t *testing.T) {


			schedulerName, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				apiClient,
				kubeclient,
				[]string{"sh", "-c", "tail -f /dev/null"},
			)

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: schedulerName}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s?version=non-existent", schedulerName), getSchedulerRequest, getSchedulerResponse)

			require.Error(t, err, "failed with status 404, response body: {\"code\":5, \"message\":\"Not Found\", \"details\":[]}")
		})

		t.Run("Should succeed - Get scheduler with query version", func(t *testing.T) {


			schedulerName, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				apiClient,
				kubeclient,
				[]string{"sh", "-c", "tail -f /dev/null"},
			)

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: schedulerName}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s?version=non-existent", schedulerName), getSchedulerRequest, getSchedulerResponse)

			require.NoError(t, err)
			require.Equal(t, getSchedulerResponse.Scheduler.Name, schedulerName)
		})

	})

}
