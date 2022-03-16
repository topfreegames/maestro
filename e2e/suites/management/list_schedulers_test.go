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

func TestListSchedulers(t *testing.T) {

	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {

		firstScheduler, err := createSchedulerAndWaitForIt(
			t,
			maestro,
			managementApiClient,
			kubeClient,
			"test-list-scheduler-game",
			[]string{"sh", "-c"},
		)

		_, err = createSchedulerAndWaitForIt(
			t,
			maestro,
			managementApiClient,
			kubeClient,
			"test-list-scheduler-game",
			[]string{"sh", "-c", "while true; do sleep 1; done"},
		)

		t.Run("Should Succeed - List schedulers", func(t *testing.T) {
			t.Parallel()

			listSchedulersRequest := &maestroApiV1.ListSchedulersRequest{}
			listSchedulersResponse := &maestroApiV1.ListSchedulersResponse{}

			err = managementApiClient.Do("GET", "/schedulers", listSchedulersRequest, listSchedulersResponse)

			require.NoError(t, err)
			require.NotEmpty(t, listSchedulersResponse.Schedulers)

			listSchedulersRequest = &maestroApiV1.ListSchedulersRequest{Name: firstScheduler.Name}
			listSchedulersResponse = &maestroApiV1.ListSchedulersResponse{}

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers?name=%s", firstScheduler.Name), listSchedulersRequest, listSchedulersResponse)

			require.NoError(t, err)
			require.Equal(t, listSchedulersResponse.Schedulers[0].Name, firstScheduler.Name)
			require.Len(t, listSchedulersResponse.Schedulers, 1)

			listSchedulersRequest = &maestroApiV1.ListSchedulersRequest{Game: firstScheduler.Game}
			listSchedulersResponse = &maestroApiV1.ListSchedulersResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers?game=%s", firstScheduler.Game), listSchedulersRequest, listSchedulersResponse)

			require.NoError(t, err)
			require.Equal(t, listSchedulersResponse.Schedulers[0].Game, firstScheduler.Game)
			require.Len(t, listSchedulersResponse.Schedulers, 2)

			listSchedulersRequest = &maestroApiV1.ListSchedulersRequest{Version: firstScheduler.Spec.Version}
			listSchedulersResponse = &maestroApiV1.ListSchedulersResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers?version=%s", firstScheduler.Spec.Version), listSchedulersRequest, listSchedulersResponse)

			require.NoError(t, err)
			require.Equal(t, listSchedulersResponse.Schedulers[0].Version, firstScheduler.Spec.Version)
			require.NotEmpty(t, listSchedulersResponse.Schedulers)
		})

		t.Run("Should Succeed - List schedulers returning empty array, when filter is not equals to a scheduler", func(t *testing.T) {
			t.Parallel()

			invalidSchedulerName := fmt.Sprintf("%s-invalid-scheduler", firstScheduler.Name)
			listSchedulersRequest := &maestroApiV1.ListSchedulersRequest{Name: invalidSchedulerName}
			listSchedulersResponse := &maestroApiV1.ListSchedulersResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers?name=%s", invalidSchedulerName), listSchedulersRequest, listSchedulersResponse)

			require.NoError(t, err)
			require.Empty(t, listSchedulersResponse.Schedulers)

			invalidSchedulerGame := fmt.Sprintf("%s-invalid-game", firstScheduler.Game)
			listSchedulersRequest = &maestroApiV1.ListSchedulersRequest{Game: invalidSchedulerGame}
			listSchedulersResponse = &maestroApiV1.ListSchedulersResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers?game=%s", invalidSchedulerGame), listSchedulersRequest, listSchedulersResponse)

			require.NoError(t, err)
			require.Empty(t, listSchedulersResponse.Schedulers)

			invalidSchedulerVersion := fmt.Sprintf("%s-invalid-version", firstScheduler.Spec.Version)
			listSchedulersRequest = &maestroApiV1.ListSchedulersRequest{Version: invalidSchedulerVersion}
			listSchedulersResponse = &maestroApiV1.ListSchedulersResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers?version=%s", invalidSchedulerVersion), listSchedulersRequest, listSchedulersResponse)

			require.NoError(t, err)
			require.Empty(t, listSchedulersResponse.Schedulers)
		})
	})

}
