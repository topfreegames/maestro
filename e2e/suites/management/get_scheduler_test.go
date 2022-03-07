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
	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		t.Run("Should Succeed - Get scheduler without query parameter version", func(t *testing.T) {
			t.Parallel()

			scheduler, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				managementApiClient,
				kubeClient,
				"test",
				[]string{"sh", "-c", "tail -f /dev/null"},
			)

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)

			require.NoError(t, err)
			require.Equal(t, getSchedulerResponse.Scheduler.Name, scheduler.Name)
		})

		t.Run("Should Fail - Get scheduler 404 with non-existent query version", func(t *testing.T) {
			t.Parallel()

			scheduler, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				managementApiClient,
				kubeClient,
				"test",
				[]string{"sh", "-c", "tail -f /dev/null"},
			)

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: scheduler.Name}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s?version=non-existent", scheduler.Name), getSchedulerRequest, getSchedulerResponse)

			require.Error(t, err, "failed with status 404, response body: {\"code\":5, \"message\":\"scheduler "+scheduler.Name+" not found\", \"details\":[]}")
		})

		t.Run("Should Fail - non-existent scheduler", func(t *testing.T) {
			t.Parallel()

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: "NonExistent"}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			err := managementApiClient.Do("GET", "/schedulers/NonExistent", getSchedulerRequest, getSchedulerResponse)

			require.Error(t, err, "failed with status 404, response body: {\"code\":5, \"message\":\"scheduler NonExistent not found\", \"details\":[]}")
		})

		t.Run("Should succeed - Get scheduler with query version", func(t *testing.T) {
			t.Parallel()

			scheduler, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				managementApiClient,
				kubeClient,
				"test",
				[]string{"sh", "-c", "tail -f /dev/null"},
			)

			getSchedulerRequest := &maestroApiV1.GetSchedulerRequest{SchedulerName: scheduler.Name}
			getSchedulerResponse := &maestroApiV1.GetSchedulerResponse{}

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s?version=v1.1", scheduler.Name), getSchedulerRequest, getSchedulerResponse)

			require.NoError(t, err)
			require.Equal(t, getSchedulerResponse.Scheduler.Name, scheduler.Name)
			require.Equal(t, getSchedulerResponse.Scheduler.GetSpec().GetVersion(), "v1.1")
		})

	})

}
