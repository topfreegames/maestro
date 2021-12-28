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

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework"
	maestrov1 "github.com/topfreegames/maestro/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func TestCreateScheduler(t *testing.T) {
	framework.WithClients(t, func(apiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		schedulerName := framework.GenerateSchedulerName()
		createRequest := &maestrov1.CreateSchedulerRequest{
			Name:                   schedulerName,
			Game:                   "test",
			Version:                "v1.1",
			TerminationGracePeriod: 15,
			MaxSurge:               "10%",
			Containers: []*maestrov1.Container{
				{
					Name:            "example",
					Image:           "nginx",
					Command:         []string{"echo", "hello"},
					ImagePullPolicy: "Always",
					Requests: &maestrov1.ContainerResources{
						Memory: "1",
						Cpu:    "1",
					},
					Limits: &maestrov1.ContainerResources{
						Memory: "1",
						Cpu:    "1",
					},
					Ports: []*maestrov1.ContainerPort{
						{
							Name:     "default",
							Protocol: "tcp",
							Port:     80,
						},
					},
				},
			},
		}

		createResponse := &maestrov1.CreateSchedulerResponse{}
		err := apiClient.Do("POST", "/schedulers", createRequest, createResponse)
		require.NoError(t, err)

		// list operations
		require.Eventually(t, func() bool {
			listOperationsRequest := &maestrov1.ListOperationsRequest{}
			listOperationsResponse := &maestrov1.ListOperationsResponse{}
			err = apiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
			require.NoError(t, err)

			if len(listOperationsResponse.FinishedOperations) == 0 {
				return false
			}

			// TODO(gabrielcorado): we can use the operations constants here.
			require.Equal(t, "create_scheduler", listOperationsResponse.FinishedOperations[0].DefinitionName)
			require.Equal(t, "finished", listOperationsResponse.FinishedOperations[0].Status)
			return true
		}, 30*time.Second, time.Second)

		// Check on kubernetes that the namespace was created.
		_, err = kubeClient.CoreV1().Namespaces().Get(context.Background(), schedulerName, metav1.GetOptions{})
		require.NoError(t, err)
	})
}
