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
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	maestrov1 "github.com/topfreegames/maestro/pkg/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func TestCreateScheduler(t *testing.T) {
	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		t.Run("should succeed", func(t *testing.T) {
			schedulerName := framework.GenerateSchedulerName()
			createRequest := &maestrov1.CreateSchedulerRequest{
				Name:     schedulerName,
				Game:     "test",
				MaxSurge: "10%",
				Spec: &maestrov1.Spec{
					TerminationGracePeriod: 15,
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
				},
				PortRange: &maestroApiV1.PortRange{
					Start: 80,
					End:   8000,
				},
			}

			createResponse := &maestrov1.CreateSchedulerResponse{}
			err := managementApiClient.Do("POST", "/schedulers", createRequest, createResponse)
			require.NoError(t, err)

			// list operations
			require.Eventually(t, func() bool {
				listOperationsRequest := &maestrov1.ListOperationsRequest{}
				listOperationsResponse := &maestrov1.ListOperationsResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
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

		t.Run("should fail - operations finish with error (namespace already exists) - scheduler not found", func(t *testing.T) {
			schedulerName := framework.GenerateSchedulerName()

			// We create the scheduler on the runtime to guarantee that the operation will fail
			err := createSchedulerOnRuntime(kubeClient, schedulerName)
			assert.NoError(t, err)

			createRequest := &maestrov1.CreateSchedulerRequest{
				Name:     schedulerName,
				Game:     "test",
				MaxSurge: "10%",
				Spec: &maestrov1.Spec{
					TerminationGracePeriod: 15,
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
				},
				PortRange: &maestroApiV1.PortRange{
					Start: 80,
					End:   8000,
				},
			}

			createResponse := &maestrov1.CreateSchedulerResponse{}
			err = managementApiClient.Do("POST", "/schedulers", createRequest, createResponse)
			require.NoError(t, err)

			// list operations
			require.Eventually(t, func() bool {
				listOperationsRequest := &maestrov1.ListOperationsRequest{}
				listOperationsResponse := &maestrov1.ListOperationsResponse{}
				err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
				require.NoError(t, err)

				if len(listOperationsResponse.FinishedOperations) == 0 {
					return false
				}

				require.Equal(t, "create_scheduler", listOperationsResponse.FinishedOperations[0].DefinitionName)
				require.Equal(t, "error", listOperationsResponse.FinishedOperations[0].Status)
				return true
			}, 30*time.Second, time.Second)

			// Check that scheduler does not exist since operation as finished with error
			getSchedulerRequest := &maestrov1.GetSchedulerRequest{}
			getSchedulerResponse := &maestrov1.GetSchedulerResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", schedulerName), getSchedulerRequest, getSchedulerResponse)
			require.Error(t, err)
		})
	})
}

func createSchedulerOnRuntime(kubeClient kubernetes.Interface, schedulerName string) error {
	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: schedulerName,
		},
	}

	_, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	return err
}