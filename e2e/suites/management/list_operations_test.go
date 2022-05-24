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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	timeClock "github.com/topfreegames/maestro/internal/adapters/clock/time"
	operationadapters "github.com/topfreegames/maestro/internal/adapters/operation"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"
	"github.com/topfreegames/maestro/internal/core/operations/providers"
	"github.com/topfreegames/maestro/internal/core/operations/test_operation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
)

func TestListOperations(t *testing.T) {
	t.Parallel()

	duration, err := time.ParseDuration("24h")
	require.NoError(t, err)

	operationsTTLMap := map[operationadapters.Definition]time.Duration{
		healthcontroller.OperationName: duration,
	}

	statusPendingString, err := operation.StatusPending.String()
	require.NoError(t, err)
	statusInProgressString, err := operation.StatusInProgress.String()
	require.NoError(t, err)
	statusFinishedString, err := operation.StatusFinished.String()
	require.NoError(t, err)

	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		operationStorage := operationadapters.NewRedisOperationStorage(redisClient, timeClock.NewClock(), operationsTTLMap)
		operationFlow := operationadapters.NewRedisOperationFlow(redisClient)

		t.Run("when listing operations should return all of them with filled fields", func(t *testing.T) {
			t.Parallel()

			scheduler, err := createSchedulerAndWaitForIt(t,
				maestro,
				managementApiClient,
				kubeClient,
				"test",
				[]string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request PUT " +
					"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
					"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 1; done"})
			require.NoError(t, err)

			createTestOperation(context.Background(), t, operationStorage, operationFlow, scheduler.Name, 1)

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", scheduler.Name), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				testPresent := false
				for _, operation := range listOperationsResponse.FinishedOperations {
					if operation.DefinitionName == test_operation.OperationName && operation.Status == statusFinishedString {
						testPresent = true
						break
					}
				}

				if !testPresent {
					return false
				}

				return true
			}, 240*time.Second, time.Second)

			listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
			listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", scheduler.Name), listOperationsRequest, listOperationsResponse)
			require.NoError(t, err)

			for _, apiOperation := range listOperationsResponse.PendingOperations {
				assert.NotEmpty(t, apiOperation.Id)
				assert.Equal(t, apiOperation.Status, statusPendingString)

				_, operationExists := providers.ProvideDefinitionConstructors()[apiOperation.DefinitionName]
				assert.True(t, operationExists)

				assert.Equal(t, apiOperation.SchedulerName, scheduler.Name)
			}

			for _, apiOperation := range listOperationsResponse.ActiveOperations {
				assert.NotEmpty(t, apiOperation.Id)
				assert.Equal(t, apiOperation.Status, statusInProgressString)

				_, operationExists := providers.ProvideDefinitionConstructors()[apiOperation.DefinitionName]
				assert.True(t, operationExists)

				assert.Equal(t, apiOperation.SchedulerName, scheduler.Name)
			}

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
