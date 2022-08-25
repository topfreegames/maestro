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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	"github.com/topfreegames/maestro/internal/core/operations/test"
	"github.com/topfreegames/maestro/internal/core/ports"

	v1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/e2e/framework"
	"github.com/topfreegames/maestro/e2e/framework/maestro"
	maestroApiV1 "github.com/topfreegames/maestro/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createSchedulerAndWaitForIt(
	t *testing.T,
	maestro *maestro.MaestroInstance,
	managementApiClient *framework.APIClient,
	kubeClient kubernetes.Interface,
	game string,
	gruCommand []string) (*maestroApiV1.Scheduler, error) {
	roomsApiAddress := maestro.RoomsApiServer.ContainerInternalAddress
	schedulerName := framework.GenerateSchedulerName()
	createRequest := &maestroApiV1.CreateSchedulerRequest{
		Name:          schedulerName,
		Game:          game,
		MaxSurge:      "10%",
		RoomsReplicas: 0,
		Spec: &maestroApiV1.Spec{
			TerminationGracePeriod: 15,
			Containers: []*maestroApiV1.Container{
				{
					Name:            "example",
					Image:           "alpine:3.15.0",
					Command:         gruCommand,
					ImagePullPolicy: "IfNotPresent",
					Environment: []*maestroApiV1.ContainerEnvironment{
						{
							Name:  "ROOMS_API_ADDRESS",
							Value: &roomsApiAddress,
						},
						{
							Name: "HOST_IP",
							ValueFrom: &maestroApiV1.ContainerEnvironmentValueFrom{
								FieldRef: &maestroApiV1.ContainerEnvironmentValueFromFieldRef{FieldPath: "status.hostIP"},
							},
						},
						{
							Name: "SECRET_ENV_VAR",
							ValueFrom: &maestroApiV1.ContainerEnvironmentValueFrom{
								SecretKeyRef: &maestroApiV1.ContainerEnvironmentValueFromSecretKeyRef{Name: "namespace-secret", Key: "secret_key"},
							},
						},
					},
					Requests: &maestroApiV1.ContainerResources{
						Memory: "20Mi",
						Cpu:    "10m",
					},
					Limits: &maestroApiV1.ContainerResources{
						Memory: "20Mi",
						Cpu:    "10m",
					},
					Ports: []*maestroApiV1.ContainerPort{
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
			Start: 40000,
			End:   60000,
		},
	}

	createResponse := &maestroApiV1.CreateSchedulerResponse{}
	err := managementApiClient.Do("POST", "/schedulers", createRequest, createResponse)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
		listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
		err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations?stage=final", schedulerName), listOperationsRequest, listOperationsResponse)
		require.NoError(t, err)

		if len(listOperationsResponse.Operations) == 0 {
			return false
		}

		require.Equal(t, "create_scheduler", listOperationsResponse.Operations[0].DefinitionName)
		require.Equal(t, "finished", listOperationsResponse.Operations[0].Status)
		return true
	}, 30*time.Second, time.Second)

	// Check on kubernetes that the scheduler namespace was created.
	_, err = kubeClient.CoreV1().Namespaces().Get(context.Background(), schedulerName, metav1.GetOptions{})
	require.NoError(t, err)

	// wait for service account to be created
	// TODO: check if we need to wait the service account to be created on internal/adapters/runtime/kubernetes/scheduler.go
	// we were having errors when not waiting for this in this test, reported in this issue https://github.com/kubernetes/kubernetes/issues/66689
	require.Eventually(t, func() bool {
		svcAccs, err := kubeClient.CoreV1().ServiceAccounts(schedulerName).List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)

		return len(svcAccs.Items) > 0
	}, 5*time.Second, time.Second)

	// creating secret used by the pods
	_, err = createNamespaceSecrets(kubeClient, schedulerName, "namespace-secret", map[string]string{"secret_key": "secret_value"})
	require.NoError(t, err)

	return createResponse.Scheduler, err
}

func createNamespaceSecrets(kubeClient kubernetes.Interface, schedulerName string, secretName string, secretMap map[string]string) (*v1.Secret, error) {
	kubeSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
	}
	kubeSecret.Data = map[string][]byte{}
	for key, value := range secretMap {
		kubeSecret.Data[key] = []byte(value)
	}
	return kubeClient.CoreV1().Secrets(schedulerName).Create(context.Background(), kubeSecret, metav1.CreateOptions{})
}

func createSchedulerWithForwardersAndWaitForIt(
	t *testing.T,
	maestro *maestro.MaestroInstance,
	managementApiClient *framework.APIClient,
	kubeClient kubernetes.Interface,
	gruCommand []string,
	forwarders []*maestroApiV1.Forwarder,
) (*maestroApiV1.Scheduler, error) {
	roomsApiAddress := maestro.RoomsApiServer.ContainerInternalAddress
	schedulerName := framework.GenerateSchedulerName()
	createRequest := &maestroApiV1.CreateSchedulerRequest{
		Name: schedulerName,
		Game: "test",
		Spec: &maestroApiV1.Spec{
			TerminationGracePeriod: 15,
			Containers: []*maestroApiV1.Container{
				{
					Name:            "example",
					Image:           "alpine:3.15.0",
					Command:         gruCommand,
					ImagePullPolicy: "IfNotPresent",
					Environment: []*maestroApiV1.ContainerEnvironment{
						{
							Name:  "ROOMS_API_ADDRESS",
							Value: &roomsApiAddress,
						},
						{
							Name: "HOST_IP",
							ValueFrom: &maestroApiV1.ContainerEnvironmentValueFrom{
								FieldRef: &maestroApiV1.ContainerEnvironmentValueFromFieldRef{FieldPath: "status.hostIP"},
							},
						},
						{
							Name: "SECRET_ENV_VAR",
							ValueFrom: &maestroApiV1.ContainerEnvironmentValueFrom{
								SecretKeyRef: &maestroApiV1.ContainerEnvironmentValueFromSecretKeyRef{Name: "namespace-secret", Key: "secret_key"},
							},
						},
					},
					Requests: &maestroApiV1.ContainerResources{
						Memory: "20Mi",
						Cpu:    "10m",
					},
					Limits: &maestroApiV1.ContainerResources{
						Memory: "20Mi",
						Cpu:    "10m",
					},
					Ports: []*maestroApiV1.ContainerPort{
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
			Start: 40000,
			End:   60000,
		},
		RoomsReplicas: 2,
		MaxSurge:      "10%",
		Forwarders:    forwarders,
	}

	createResponse := &maestroApiV1.CreateSchedulerResponse{}
	err := managementApiClient.Do("POST", "/schedulers", createRequest, createResponse)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
		listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
		err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations?stage=final", schedulerName), listOperationsRequest, listOperationsResponse)
		require.NoError(t, err)

		if len(listOperationsResponse.Operations) == 0 {
			return false
		}

		require.Equal(t, "create_scheduler", listOperationsResponse.Operations[0].DefinitionName)
		require.Equal(t, "finished", listOperationsResponse.Operations[0].Status)
		return true
	}, 30*time.Second, time.Second)

	_, err = kubeClient.CoreV1().Namespaces().Get(context.Background(), schedulerName, metav1.GetOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		svcAccs, err := kubeClient.CoreV1().ServiceAccounts(schedulerName).List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)

		return len(svcAccs.Items) == 1
	}, 30*time.Second, time.Second)

	// creating secret used by the pods
	_, err = createNamespaceSecrets(kubeClient, schedulerName, "namespace-secret", map[string]string{"secret_key": "secret_value"})
	require.NoError(t, err)
	return createResponse.Scheduler, err
}

func createTestOperation(ctx context.Context, t *testing.T, operationStorage ports.OperationStorage, operationFlow ports.OperationFlow, schedulerName string, sleepSeconds int) *operation.Operation {
	definition := test.Definition{
		SleepSeconds: sleepSeconds,
	}

	op := &operation.Operation{
		ID:             uuid.NewString(),
		Status:         operation.StatusPending,
		DefinitionName: definition.Name(),
		SchedulerName:  schedulerName,
		CreatedAt:      time.Now(),
		Input:          definition.Marshal(),
	}

	err := operationStorage.CreateOperation(ctx, op)
	require.NoError(t, err)
	err = operationFlow.InsertOperationID(ctx, op.SchedulerName, op.ID)
	require.NoError(t, err)

	return op
}

func addStubRequestToMockedGrpcServer(stubFileName string) error {
	httpClient := &http.Client{}
	stubsPath := "../framework/maestro/servermocks/"
	stub, err := ioutil.ReadFile(stubsPath + stubFileName + ".json")
	if err != nil {
		return err
	}
	request, err := http.NewRequest("POST", "http://localhost:4771/add", bytes.NewReader(stub))
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status: %s", resp.Status)
	}
	return nil
}

func createSchedulerWithRoomsAndWaitForIt(t *testing.T, maestro *maestro.MaestroInstance, managementApiClient *framework.APIClient, game string, kubeClient kubernetes.Interface) (*maestroApiV1.Scheduler, error) {
	// Create scheduler
	scheduler, err := createSchedulerAndWaitForIt(
		t,
		maestro,
		managementApiClient,
		kubeClient,
		game,
		[]string{"/bin/sh", "-c", "apk add curl && " + "while true; do curl --request PUT " +
			"$ROOMS_API_ADDRESS/scheduler/$MAESTRO_SCHEDULER_NAME/rooms/$MAESTRO_ROOM_ID/ping " +
			"--data-raw '{\"status\": \"ready\",\"timestamp\": \"12312312313\"}' && sleep 20; done"},
	)
	require.NoError(t, err)

	// Change RoomsReplicas

	roomsReplicas := int32(2)
	patchSchedulerRequest := &maestroApiV1.PatchSchedulerRequest{RoomsReplicas: &roomsReplicas}
	patchSchedulerResponse := &maestroApiV1.PatchSchedulerResponse{}
	err = managementApiClient.Do("PATCH", fmt.Sprintf("/schedulers/%s", scheduler.Name), patchSchedulerRequest, patchSchedulerResponse)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		getOperationRequest := &maestroApiV1.GetOperationRequest{}
		getOperationResponse := &maestroApiV1.GetOperationResponse{}
		err := managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations/%s", scheduler.Name, patchSchedulerResponse.OperationId), getOperationRequest, getOperationResponse)
		require.NoError(t, err)

		if getOperationResponse.Operation.Status != "finished" {
			return false
		}

		return true
	}, 4*time.Minute, 10*time.Millisecond, "failed to create scheduler")

	require.Eventually(t, func() bool {
		pods, err := kubeClient.CoreV1().Pods(scheduler.Name).List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)

		if len(pods.Items) != 2 {
			return false
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase != v1.PodRunning {
				return false
			}
		}

		return true
	}, 2*time.Minute, 10*time.Millisecond, "failed to wait for initial pods to be created")

	scheduler.RoomsReplicas = 2

	return scheduler, err
}

func waitForOperationToFinish(t *testing.T, managementApiClient *framework.APIClient, schedulerName, operation string) {
	require.Eventually(t, func() bool {
		listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
		listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
		err := managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations?stage=final", schedulerName), listOperationsRequest, listOperationsResponse)
		require.NoError(t, err)

		if len(listOperationsResponse.Operations) >= 1 {
			for _, _operation := range listOperationsResponse.Operations {
				if _operation.DefinitionName == operation && _operation.Status == "finished" {
					return true
				}
			}
		}

		return false
	}, 4*time.Minute, 10*time.Millisecond)
}

func waitForOperationToFinishByOperationId(t *testing.T, managementApiClient *framework.APIClient, schedulerName, operationId string) {
	require.Eventually(t, func() bool {
		getOperationRequest := &maestroApiV1.GetOperationRequest{}
		getOperationResponse := &maestroApiV1.GetOperationResponse{}
		err := managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations/%s", schedulerName, operationId), getOperationRequest, getOperationResponse)
		require.NoError(t, err)

		op := getOperationResponse.GetOperation()
		statusFinish, _ := operation.StatusFinished.String()
		if op.GetStatus() == statusFinish {
			return true
		}

		return false
	}, 4*time.Minute, time.Second*5, "Timeout waiting operation to reach finished status")
}

func waitForOperationToFailById(t *testing.T, managementApiClient *framework.APIClient, schedulerName, operationId string) {
	require.Eventually(t, func() bool {
		getOperationRequest := &maestroApiV1.GetOperationRequest{}
		getOperationResponse := &maestroApiV1.GetOperationResponse{}
		err := managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations/%s", schedulerName, operationId), getOperationRequest, getOperationResponse)
		require.NoError(t, err)

		op := getOperationResponse.GetOperation()
		statusError, _ := operation.StatusError.String()
		if op.GetStatus() == statusError {
			return true
		}

		return false
	}, 4*time.Minute, time.Second*5, "Timout waiting operation to reach error state")
}
