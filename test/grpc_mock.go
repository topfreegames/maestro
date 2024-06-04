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

package test

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/orlangure/gnomock"
)

// WithGrpcMockContainer instantiate a grpcMock to be used by integration tests.
func WithGrpcMockContainer(exec func(grpcMockAddress, httpInputMockAddress string)) {
	var err error
	path, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Error fetching current path: %s", err.Error()))
	}
	grpcMockContainer, err := gnomock.StartCustom(
		"tkpd/gripmock",
		gnomock.NamedPorts{
			"grpc": gnomock.TCP(4770),
			"http": gnomock.TCP(4771),
		},
		gnomock.WithCommand("/protos/events.proto"),
		gnomock.WithHostMounts(path+"/../../../test/data", "/protos"),
	)
	if err != nil {
		panic(fmt.Sprintf("error creating gripmock grpc docker instance: %s\n", err))
	}

	time.Sleep(10 * time.Second)

	exec(grpcMockContainer.Address("grpc"), grpcMockContainer.Address("http"))

	_ = gnomock.Stop(grpcMockContainer)
}

// AddStubRequestToMockedGrpcServer add a mock to GRPC server.
func AddStubRequestToMockedGrpcServer(httpInputMockAddress, stubFilePath string) error {
	httpClient := &http.Client{}
	stub, err := os.ReadFile(stubFilePath)
	if err != nil {
		return err
	}
	request, err := http.NewRequest("POST", fmt.Sprintf("http://%s/add", httpInputMockAddress), bytes.NewReader(stub))
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
