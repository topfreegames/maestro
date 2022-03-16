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
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
)

func TestListOperations(t *testing.T) {
	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		t.Run("when listing operations should return all of them with filled fields", func(t *testing.T) {
			t.Parallel()

			scheduler, err := createSchedulerAndWaitForIt(t,
				maestro,
				managementApiClient,
				kubeClient,
				"test",
				[]string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request POST " +
					"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
					"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 1; done"})
			require.NoError(t, err)

			addRoomsRequest := &maestroApiV1.AddRoomsRequest{SchedulerName: scheduler.Name, Amount: 1}
			addRoomsResponse := &maestroApiV1.AddRoomsResponse{}
			err = managementApiClient.Do("POST", fmt.Sprintf("/schedulers/%s/add-rooms", scheduler.Name), addRoomsRequest, addRoomsResponse)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", scheduler.Name), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.FinishedOperations) < 2 {
					return false
				}

				require.Equal(t, "add_rooms", listOperationsResponse.FinishedOperations[0].DefinitionName)
				require.Equal(t, "finished", listOperationsResponse.FinishedOperations[0].Status)
				return true
			}, 240*time.Second, time.Second)

			listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
			listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", scheduler.Name), listOperationsRequest, listOperationsResponse)
			require.NoError(t, err)

			statusPendingString, err := operation.StatusPending.String()
			require.NoError(t, err)
			for _, apiOperation := range listOperationsResponse.PendingOperations {
				assert.NotEmpty(t, apiOperation.Id)
				assert.Equal(t, apiOperation.Status, statusPendingString)

				_, operationExists := providers.ProvideDefinitionConstructors()[apiOperation.DefinitionName]
				assert.True(t, operationExists)

				assert.Equal(t, apiOperation.SchedulerName, scheduler.Name)
			}

			statusInProgressString, err := operation.StatusInProgress.String()
			for _, apiOperation := range listOperationsResponse.ActiveOperations {
				assert.NotEmpty(t, apiOperation.Id)
				assert.Equal(t, apiOperation.Status, statusInProgressString)

				_, operationExists := providers.ProvideDefinitionConstructors()[apiOperation.DefinitionName]
				assert.True(t, operationExists)

				assert.Equal(t, apiOperation.SchedulerName, scheduler.Name)
			}

			statusFinishedString, err := operation.StatusFinished.String()
			for _, apiOperation := range listOperationsResponse.FinishedOperations {
				assert.NotEmpty(t, apiOperation.Id)
				assert.Equal(t, apiOperation.Status, statusFinishedString)

				_, operationExists := providers.ProvideDefinitionConstructors()[apiOperation.DefinitionName]
				assert.True(t, operationExists)

				assert.Equal(t, apiOperation.SchedulerName, scheduler.Name)
			}
		})
	})
}
