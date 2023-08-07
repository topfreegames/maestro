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
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/topfreegames/maestro/e2e/framework/maestro"

	"github.com/topfreegames/maestro/e2e/framework"
	maestrov1 "github.com/topfreegames/maestro/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
)

func TestPatchScheduler(t *testing.T) {
	t.Parallel()
	framework.WithClients(t, func(roomsApiClient *framework.APIClient, managementApiClient *framework.APIClient, kubeClient kubernetes.Interface, redisClient *redis.Client, maestro *maestro.MaestroInstance) {
		scheduler, err := createSchedulerAndWaitForIt(t,
			maestro,
			managementApiClient,
			kubeClient,
			"test",
			[]string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request PUT " +
				"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
				"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 20; done"})

		getSchedulerRequest := &maestrov1.GetSchedulerRequest{}
		getSchedulerResponse := &maestrov1.GetSchedulerResponse{}
		err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)
		require.NoError(t, err)
		require.Equal(t, "v1.0.0", getSchedulerResponse.Scheduler.Spec.Version)

		t.Run("should succeed patching single field of the scheduler", func(t *testing.T) {
			newMaxSurge := "22"
			patchSchedulerRequest := &maestrov1.PatchSchedulerRequest{MaxSurge: &newMaxSurge}
			patchSchedulerResponse := &maestrov1.PatchSchedulerResponse{}
			err = managementApiClient.Do("PATCH", fmt.Sprintf("/schedulers/%s", scheduler.Name), patchSchedulerRequest, patchSchedulerResponse)
			require.NoError(t, err)

			waitForOperationToFinishByOperationId(t, managementApiClient, scheduler.Name, patchSchedulerResponse.OperationId)

			getSchedulerRequest = &maestrov1.GetSchedulerRequest{}
			getSchedulerResponse = &maestrov1.GetSchedulerResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)
			require.NoError(t, err)
			require.Equal(t, "v1.1.0", getSchedulerResponse.Scheduler.Spec.Version)
			require.Equal(t, newMaxSurge, getSchedulerResponse.Scheduler.MaxSurge)
		})

		t.Run("should succeed patching multiple fields of the scheduler", func(t *testing.T) {
			newMaxSurge := "50%"
			newPortRange := maestrov1.PortRange{
				Start: 1000,
				End:   2000,
			}
			newTerminationGracePeriod := durationpb.New(time.Duration(99))
			newContainerImage := "alpine:3.14"
			newImagePullPolicy := "IfNotPresent"
			newContainers := []*maestrov1.OptionalContainer{
				{
					Image:           &newContainerImage,
					ImagePullPolicy: &newImagePullPolicy,
					Requests: &maestrov1.ContainerResources{
						Memory: "40Mi",
						Cpu:    "20m",
					},
					Limits: &maestrov1.ContainerResources{
						Memory: "40Mi",
						Cpu:    "20m",
					},
					Ports: []*maestrov1.ContainerPort{
						{
							Name:     "newportname",
							Protocol: "tcp",
							Port:     80,
						},
					},
				},
			}
			newSpec := maestrov1.OptionalSpec{
				TerminationGracePeriod: newTerminationGracePeriod,
				Containers:             newContainers,
			}
			newForwarders := []*maestrov1.Forwarder{
				{
					Name:    "newForwarder",
					Enable:  true,
					Type:    "gRPC",
					Address: "new_address",
					Options: &maestrov1.ForwarderOptions{
						Timeout: 100,
					},
				},
			}
			patchSchedulerRequest := &maestrov1.PatchSchedulerRequest{
				MaxSurge:   &newMaxSurge,
				PortRange:  &newPortRange,
				Spec:       &newSpec,
				Forwarders: newForwarders,
			}
			patchSchedulerResponse := &maestrov1.PatchSchedulerResponse{}
			err = managementApiClient.Do("PATCH", fmt.Sprintf("/schedulers/%s", scheduler.Name), patchSchedulerRequest, patchSchedulerResponse)
			require.NoError(t, err)

			waitForOperationToFinishByOperationId(t, managementApiClient, scheduler.Name, patchSchedulerResponse.OperationId)

			getSchedulerRequest = &maestrov1.GetSchedulerRequest{}
			getSchedulerResponse = &maestrov1.GetSchedulerResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)
			newForwarder := newForwarders[0]
			responseForwarder := *getSchedulerResponse.Scheduler.Forwarders[0]

			require.NoError(t, err)
			assert.Equal(t, "v2.0.0", getSchedulerResponse.Scheduler.Spec.Version)
			assert.Equal(t, newMaxSurge, getSchedulerResponse.Scheduler.MaxSurge)
			assert.Equal(t, newPortRange, *getSchedulerResponse.Scheduler.PortRange)
			assert.Equal(t, newForwarder.Name, responseForwarder.Name)
			assert.Equal(t, newForwarder.Address, responseForwarder.Address)
			assert.Equal(t, newForwarder.Options.Timeout, responseForwarder.Options.Timeout)
			assert.Equal(t, newForwarder.Type, responseForwarder.Type)
			assert.Equal(t, newForwarder.Enable, responseForwarder.Enable)
			assert.Equal(t, *newSpec.Containers[0].ImagePullPolicy, getSchedulerResponse.Scheduler.Spec.Containers[0].ImagePullPolicy)
			assert.Equal(t, *newSpec.Containers[0].Requests, *getSchedulerResponse.Scheduler.Spec.Containers[0].Requests)
			assert.Equal(t, *newSpec.Containers[0].Limits, *getSchedulerResponse.Scheduler.Spec.Containers[0].Limits)
			assert.Equal(t, newSpec.Containers[0].Ports, getSchedulerResponse.Scheduler.Spec.Containers[0].Ports)
			assert.Equal(t, *newSpec.TerminationGracePeriod, *getSchedulerResponse.Scheduler.Spec.TerminationGracePeriod)

		})

		t.Run("should fail patching empty fields of the scheduler", func(t *testing.T) {
			patchSchedulerRequest := &maestrov1.PatchSchedulerRequest{}
			patchSchedulerResponse := &maestrov1.PatchSchedulerResponse{}
			err = managementApiClient.Do("PATCH", fmt.Sprintf("/schedulers/%s", scheduler.Name), patchSchedulerRequest, patchSchedulerResponse)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed with status 409")
			getSchedulerRequest = &maestrov1.GetSchedulerRequest{}
			getSchedulerResponse = &maestrov1.GetSchedulerResponse{}
			err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s", scheduler.Name), getSchedulerRequest, getSchedulerResponse)
			require.NoError(t, err)
			// Nothing changed
			require.Equal(t, "v2.0.0", getSchedulerResponse.Scheduler.Spec.Version)
		})

	})
}
