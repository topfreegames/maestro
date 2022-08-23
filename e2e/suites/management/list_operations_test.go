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

	"github.com/stretchr/testify/assert"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations/test_operation"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	timeClock "github.com/topfreegames/maestro/internal/adapters/clock/time"
	operationadapters "github.com/topfreegames/maestro/internal/adapters/operation"
	"github.com/topfreegames/maestro/internal/core/operations/healthcontroller"

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

	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		operationStorage := operationadapters.NewRedisOperationStorage(redisClient, timeClock.NewClock(), operationsTTLMap)
		operationFlow := operationadapters.NewRedisOperationFlow(redisClient)
		inProgressStatus, _ := operation.StatusInProgress.String()
		pendingStatus, _ := operation.StatusPending.String()
		finishedStatus, _ := operation.StatusFinished.String()

		scheduler, err := createSchedulerAndWaitForIt(t,
			maestro,
			managementApiClient,
			kubeClient,
			"test",
			[]string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request PUT " +
				"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
				"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 1; done"})
		require.NoError(t, err)
		numberOfPendingTestOps := 3
		numberOfFinishedTestOps := 10

		// Create some finished operations
		for i := 0; i < numberOfFinishedTestOps; i++ {
			createTestOperation(context.Background(), t, operationStorage, operationFlow, scheduler.Name, 1)
		}

		activeOp := createTestOperation(context.Background(), t, operationStorage, operationFlow, scheduler.Name, 2400)

		// Create some pending operations
		for i := 0; i < numberOfPendingTestOps; i++ {
			createTestOperation(context.Background(), t, operationStorage, operationFlow, scheduler.Name, 2400)
		}

		t.Run("when listing active operation it should return the operation that is currently being executed", func(t *testing.T) {

			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listActiveOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations?stage=active", scheduler.Name), listOperationsRequest, listActiveOperationsResponse)
				require.NoError(t, err)

				if len(listActiveOperationsResponse.Operations) == 1 && listActiveOperationsResponse.Operations[0].Id == activeOp.ID {
					assertPaginationInfo(t, listActiveOperationsResponse, uint32(1), uint32(1), uint32(1))
					assertEqualOperations(t, listActiveOperationsResponse.Operations[0], activeOp, inProgressStatus)
					return true
				}

				return false
			}, 1*time.Minute, 10*time.Second)

		})

		t.Run("when listing pending operations it should return all operations that are pending to be executed", func(t *testing.T) {
			listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
			listPendingOperationsResponse := &maestroApiV1.ListOperationsResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations?stage=pending", scheduler.Name), listOperationsRequest, listPendingOperationsResponse)
			require.NoError(t, err)
			assertPaginationInfo(t, listPendingOperationsResponse, uint32(numberOfPendingTestOps), uint32(1), uint32(numberOfPendingTestOps))
			testOpsResponseCount := 0
			for _, pendingOp := range listPendingOperationsResponse.Operations {
				if pendingOp.DefinitionName == test_operation.OperationName {
					testOpsResponseCount++
				}

				assert.Equal(t, pendingStatus, pendingOp.Status)
			}
			assert.Equal(t, numberOfPendingTestOps, testOpsResponseCount)
		})

		t.Run("when listing final operations it should return all operations that were already executed with pagination", func(t *testing.T) {
			listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
			listFinalOperationsResponse := &maestroApiV1.ListOperationsResponse{}
			testOpsResponseCount := 0

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations?stage=final", scheduler.Name), listOperationsRequest, listFinalOperationsResponse)
			require.NoError(t, err)
			// Get total for calculating per page value since the finished operations can contain some health controller operations
			finishedOpsTotal := listFinalOperationsResponse.Total
			perPage := (int(*finishedOpsTotal) / 2) + 1

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations?stage=final&page=1&perPage=%d", scheduler.Name, perPage), listOperationsRequest, listFinalOperationsResponse)
			require.NoError(t, err)
			assertPaginationInfo(t, listFinalOperationsResponse, uint32(10), uint32(1), uint32(5))
			assert.Len(t, listFinalOperationsResponse.Operations, perPage)

			for _, pendingOp := range listFinalOperationsResponse.Operations {
				if pendingOp.DefinitionName == test_operation.OperationName {
					testOpsResponseCount++
				}

				// Assert that every health controller operation in the final stage took some action
				if pendingOp.DefinitionName == healthcontroller.OperationName {
					assertHealthControllerTookAction(t, redisClient, scheduler, pendingOp)
				}

				assert.Equal(t, finishedStatus, pendingOp.Status)
			}

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations?stage=final&page=2&perPage=%d", scheduler.Name, perPage), listOperationsRequest, listFinalOperationsResponse)
			require.NoError(t, err)
			assertPaginationInfo(t, listFinalOperationsResponse, uint32(10), uint32(2), uint32(5))
			assert.Len(t, listFinalOperationsResponse.Operations, int(perPage)-1)
			for _, pendingOp := range listFinalOperationsResponse.Operations {
				if pendingOp.DefinitionName == test_operation.OperationName {
					testOpsResponseCount++
				}

				// Assert that every health controller operation in the final stage took some action
				if pendingOp.DefinitionName == healthcontroller.OperationName {
					assertHealthControllerTookAction(t, redisClient, scheduler, pendingOp)
				}

				assert.Equal(t, finishedStatus, pendingOp.Status)
			}

			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations?stage=final&page=3&perPage=%d", scheduler.Name, perPage), listOperationsRequest, listFinalOperationsResponse)
			require.NoError(t, err)
			assertPaginationInfo(t, listFinalOperationsResponse, uint32(10), uint32(3), uint32(5))
			assert.Empty(t, listFinalOperationsResponse.Operations)

			assert.Equal(t, numberOfFinishedTestOps, testOpsResponseCount)
		})
	})
}

func assertHealthControllerTookAction(t *testing.T, redisClient *redis.Client, scheduler *maestroApiV1.Scheduler, finishedOp *maestroApiV1.ListOperationItem) {
	healthControllerDef := healthcontroller.SchedulerHealthControllerDefinition{}
	definitionContents, err := redisClient.HGet(context.Background(), fmt.Sprintf("operations:%s:%s", scheduler.Name, finishedOp.Id), "definitionContents").Result()
	require.NoError(t, err)
	err = healthControllerDef.Unmarshal([]byte(definitionContents))
	require.NoError(t, err)
	assert.Equal(t, true, healthControllerDef.TookAction)
}

func assertPaginationInfo(t *testing.T, response *maestroApiV1.ListOperationsResponse, total uint32, page uint32, pageSize uint32) {
	assert.GreaterOrEqual(t, *response.Total, total)
	assert.GreaterOrEqual(t, *response.PageSize, pageSize)
	assert.Equal(t, page, *response.Page)
}

func assertEqualOperations(t *testing.T, op *maestroApiV1.ListOperationItem, op2 *operation.Operation, status string) {
	assert.Equal(t, op.Id, op2.ID)
	assert.Equal(t, op.DefinitionName, op2.DefinitionName)
	assert.Equal(t, op.SchedulerName, op2.SchedulerName)
	assert.Equal(t, op.Status, status)
}
