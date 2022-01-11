package management

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
)

func TestOperationLease(t *testing.T) {
	framework.WithClients(t, func(apiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {

		t.Run("the lease gets renewed while worker continue to run", func(t *testing.T) {
			t.Parallel()

			schedulerName, err := createSchedulerAndWaitForIt(
				t,
				maestro,
				apiClient,
				kubeClient,
				[]string{"sh", "-c", "tail -f /dev/null"},
			)

			addRoomsRequest := &maestroApiV1.AddRoomsRequest{SchedulerName: schedulerName, Amount: 1}
			addRoomsResponse := &maestroApiV1.AddRoomsResponse{}
			err = apiClient.Do("POST", fmt.Sprintf("/schedulers/%s/add-rooms", schedulerName), addRoomsRequest, addRoomsResponse)
			var previousTtl = &time.Time{}
			require.Eventually(t, func() bool {
				listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
				listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
				err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				// Only exit the loop when the operation finishes
				if len(listOperationsResponse.FinishedOperations) >= 2 {
					return true
				}

				// Don't make assertions while the operation hasn't started
				if len(listOperationsResponse.ActiveOperations) < 1 {
					return false
				}

				require.Equal(t, "add_rooms", listOperationsResponse.ActiveOperations[0].DefinitionName)
				require.NotNil(t, "add_rooms", listOperationsResponse.ActiveOperations[0].Lease)
				require.NotNil(t, "add_rooms", listOperationsResponse.ActiveOperations[0].Lease.Ttl)
				renewedTtl, err := time.Parse(time.RFC3339, listOperationsResponse.ActiveOperations[0].Lease.Ttl)
				fmt.Println(renewedTtl.String())
				require.NoError(t, err)
				require.True(t, renewedTtl.After(time.Now()))
				require.True(t, previousTtl.Before(renewedTtl))

				previousTtl = &renewedTtl
				return false
			}, 240*time.Second, 5*time.Second)

		})

	})

}
