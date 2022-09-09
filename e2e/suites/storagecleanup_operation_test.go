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

package suites

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework"
	"github.com/topfreegames/maestro/e2e/framework/maestro"
	timeClock "github.com/topfreegames/maestro/internal/adapters/clock/time"
	operation2 "github.com/topfreegames/maestro/internal/adapters/flow/redis/operation"
	operationredis "github.com/topfreegames/maestro/internal/adapters/storage/redis/operation"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	operationsproviders "github.com/topfreegames/maestro/internal/core/operations/providers"
	"github.com/topfreegames/maestro/internal/core/operations/storagecleanup"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
)

const storagecleanupTestGame = "storagecleanup-game"

func TestStorageCleanUpOperation(t *testing.T) {
	t.Parallel()

	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {

		t.Run("When storage cleanup operation runs, should clear the operation history", func(t *testing.T) {
			operationsTTLMap := map[operationredis.Definition]time.Duration{}
			operationStorage := operationredis.NewRedisOperationStorage(redisClient, timeClock.NewClock(), operationsTTLMap, operationsproviders.ProvideDefinitionConstructors())
			operationFlow := operation2.NewRedisOperationFlow(redisClient)

			scheduler, err := createSchedulerAndWaitForIt(t,
				maestro,
				managementApiClient,
				kubeClient,
				"test",
				[]string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request PUT " +
					"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
					"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 1; done"})
			require.NoError(t, err)

			operations, _, err := operationStorage.ListSchedulerFinishedOperations(context.Background(), scheduler.Name, 0, -1)
			require.NoError(t, err)

			// Delete operations
			pipe := redisClient.Pipeline()
			for _, op := range operations {
				pipe.Del(context.Background(), fmt.Sprintf("operations:%s:%s", scheduler.Name, op.ID))
			}

			_, err = pipe.Exec(context.Background())
			require.NoError(t, err)

			// Execute storagecleaup operation
			storageCleanUpOperation := operation.New(scheduler.Name, storagecleanup.OperationName, (&storagecleanup.Definition{}).Marshal())

			err = operationStorage.CreateOperation(context.Background(), storageCleanUpOperation)
			require.NoError(t, err)

			err = operationFlow.InsertOperationID(context.Background(), scheduler.Name, storageCleanUpOperation.ID)
			require.NoError(t, err)

			// Ensure storagecleanup ran
			require.Eventually(t, func() bool {
				getOperationRequest := &maestroApiV1.GetOperationRequest{}
				getOperationResponse := &maestroApiV1.GetOperationResponse{}
				err := managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations/%s", scheduler.Name, storageCleanUpOperation.ID), getOperationRequest, getOperationResponse)
				require.NoError(t, err)

				if getOperationResponse.Operation.Status != "finished" {
					return false
				}

				return true
			}, 4*time.Minute, 10*time.Millisecond, "failed to create scheduler")

			newOperations, _, err := operationStorage.ListSchedulerFinishedOperations(context.Background(), scheduler.Name, 0, -1)
			require.NoError(t, err)

			for _, op := range newOperations {
				for _, inexistentsOperations := range operations {
					if op.ID == inexistentsOperations.ID {
						assert.FailNow(t, "operation keep existing after storagecleanup execution")
					}
				}
			}

			newOperationsIDs, err := redisClient.ZRangeByScore(context.Background(), fmt.Sprintf("operations:%s:lists:history", scheduler.Name), &redis.ZRangeBy{
				Min: "-inf",
				Max: "+inf",
			}).Result()
			require.NoError(t, err)

			for _, opIDs := range newOperationsIDs {
				for _, inexistentsOperations := range operations {
					if opIDs == inexistentsOperations.ID {
						assert.FailNow(t, "operation ID keep existing after storagecleanup execution")
					}
				}
			}
		})
	})
}
