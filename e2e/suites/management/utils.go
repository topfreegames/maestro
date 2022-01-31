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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	_struct "github.com/golang/protobuf/ptypes/struct"

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
	gruCommand []string) (string, error) {
	schedulerName := framework.GenerateSchedulerName()
	createRequest := &maestroApiV1.CreateSchedulerRequest{
		Name:                   schedulerName,
		Game:                   "test",
		Version:                "v1.1",
		MaxSurge:               "10%",
		TerminationGracePeriod: 15,
		Containers: []*maestroApiV1.Container{
			{
				Name:            "example",
				Image:           "alpine",
				Command:         gruCommand,
				ImagePullPolicy: "Always",
				Environment: []*maestroApiV1.ContainerEnvironment{
					{
						Name:  "ROOMS_API_ADDRESS",
						Value: maestro.RoomsApiServer.ContainerInternalAddress,
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
		PortRange: &maestroApiV1.PortRange{
			Start: 80,
			End:   8000,
		},
	}

	createResponse := &maestroApiV1.CreateSchedulerResponse{}
	err := managementApiClient.Do("POST", "/schedulers", createRequest, createResponse)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
		listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
		err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
		require.NoError(t, err)

		if len(listOperationsResponse.FinishedOperations) == 0 {
			return false
		}

		require.Equal(t, "create_scheduler", listOperationsResponse.FinishedOperations[0].DefinitionName)
		require.Equal(t, "finished", listOperationsResponse.FinishedOperations[0].Status)
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
	return schedulerName, err
}

func createSchedulerWithForwardersAndWaitForIt(
	t *testing.T,
	maestro *maestro.MaestroInstance,
	managementApiClient *framework.APIClient,
	kubeClient kubernetes.Interface,
	forwarderAddress string,
	gruCommand []string) (string, error) {
	schedulerName := framework.GenerateSchedulerName()
	createRequest := &maestroApiV1.CreateSchedulerRequest{
		Name:                   schedulerName,
		Game:                   "test",
		Version:                "v1.1",
		TerminationGracePeriod: 15,
		Containers: []*maestroApiV1.Container{
			{
				Name:            "example",
				Image:           "alpine",
				Command:         gruCommand,
				ImagePullPolicy: "Always",
				Environment: []*maestroApiV1.ContainerEnvironment{
					{
						Name:  "ROOMS_API_ADDRESS",
						Value: maestro.RoomsApiServer.ContainerInternalAddress,
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
		PortRange: &maestroApiV1.PortRange{
			Start: 80,
			End:   8000,
		},
		MaxSurge: "10%",
		Forwarders: []*maestroApiV1.Forwarder{
			{
				Name:    "matchmaker-grpc",
				Enable:  true,
				Type:    "grpc",
				Address: forwarderAddress,
				Options: &maestroApiV1.ForwarderOptions{
					Timeout:  5000,
					Metadata: &_struct.Struct{},
				},
			},
		},
	}

	createResponse := &maestroApiV1.CreateSchedulerResponse{}
	err := managementApiClient.Do("POST", "/schedulers", createRequest, createResponse)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		listOperationsRequest := &maestroApiV1.ListOperationsRequest{}
		listOperationsResponse := &maestroApiV1.ListOperationsResponse{}
		err = managementApiClient.Do("GET", fmt.Sprintf("/schedulers/%s/operations", schedulerName), listOperationsRequest, listOperationsResponse)
		require.NoError(t, err)

		if len(listOperationsResponse.FinishedOperations) == 0 {
			return false
		}

		require.Equal(t, "create_scheduler", listOperationsResponse.FinishedOperations[0].DefinitionName)
		require.Equal(t, "finished", listOperationsResponse.FinishedOperations[0].Status)
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
	return schedulerName, err
}

func addStubRequestToMockedGrpcServer(stubFileName string) error {
	httpClient := &http.Client{}
	stubsPath := "../../framework/maestro/servermocks/"
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
