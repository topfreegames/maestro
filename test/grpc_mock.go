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
	"path/filepath"
	"time"

	"github.com/orlangure/gnomock"
)

// getProjectRoot traverses up from the current working directory to find the project root,
// identified by the presence of a "go.mod" file.
func getProjectRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	for {
		goModPath := filepath.Join(wd, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return wd, nil // Found go.mod, this is the project root
		}

		parent := filepath.Dir(wd)
		if parent == wd { // Reached the filesystem root
			break
		}
		wd = parent
	}

	return "", fmt.Errorf("failed to find project root (go.mod not found)")
}

// WithGrpcMockContainer instantiate a grpcMock to be used by integration tests.
func WithGrpcMockContainer(exec func(grpcMockAddress, httpInputMockAddress string)) {
	projectRoot, err := getProjectRoot()
	if err != nil {
		panic(fmt.Sprintf("error getting project root: %s\n", err))
	}

	gRPCMockContainer, startErr := gnomock.StartCustom(
		"tkpd/gripmock:v1.12.1", // Pin image version
		gnomock.NamedPorts{
			"grpc": gnomock.TCP(4770),
			"http": gnomock.TCP(4771),
		},
		gnomock.WithHostMounts(filepath.Join(projectRoot, "test/data"), "/protos"),
		gnomock.WithHostMounts(filepath.Join(projectRoot, "proto/api/v1"), "/protos_api_v1"),
		// Start gripmock in the background and keep the container alive with tail -f /dev/null
		// Load events.proto and all protos from proto/api/v1
		// Stubs are expected to be in /protos/events_mock based on the first mount
		gnomock.WithCommand("sh", "-c", "gripmock --imports /protos --imports /protos_api_v1 --stub=/protos/events_mock /protos/events.proto /protos_api_v1/*.proto & tail -f /dev/null"),
	)
	// Capture container and error for the defer block.
	containerForDefer := gRPCMockContainer
	errForDefer := startErr

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic recovered during gRPC mock container setup/teardown: %v\n", r)
		}

		if errForDefer != nil {
			fmt.Printf("gnomock.StartCustom returned an error: %v\n", errForDefer)
		} else if containerForDefer == nil {
			fmt.Println("Warning: gnomock.StartCustom returned no error, but container is nil.")
		}

		if containerForDefer != nil {
			if err := gnomock.Stop(containerForDefer); err != nil {
				fmt.Printf("Error stopping gripmock container: %v\n", err)
			}
		}
	}()

	if startErr != nil {
		// This panic will be caught by the recover in the defer block.
		panic(fmt.Sprintf("error creating gripmock grpc docker instance: %v", startErr))
	}
	if gRPCMockContainer == nil {
		// This case should ideally be covered by startErr != nil, but as a safeguard:
		panic("gnomock.StartCustom returned nil container without an error")
	}

	// Readiness probe for gripmock
	httpAddr := gRPCMockContainer.Address("http")
	gRPCPort := gRPCMockContainer.Port("grpc") // Get the gRPC port for logging
	fmt.Printf("Attempting to connect to gripmock HTTP at: %s (gRPC on port %d)\n", httpAddr, gRPCPort)

	const ( // Constants for readiness probe
		readinessTimeout  = 30 * time.Second
		readinessInterval = 500 * time.Millisecond
	)
	startTime := time.Now()
	ready := false
	for time.Since(startTime) < readinessTimeout {
		// We use the http address for the readiness check as it's simpler than a gRPC health check.
		// Gripmock's /add endpoint is what we'd use, but a simple GET to the base URL should suffice
		// to check if the HTTP server is up, which implies gRPC server is likely also starting/up.
		resp, err := http.Get(fmt.Sprintf("http://%s", httpAddr)) // Check base URL
		if err == nil {
			resp.Body.Close()
			// Gripmock might return 404 for GET /, but a successful connection is what we want.
			// Or it might return 200 if it has a default handler for /
			// Any successful HTTP response (err == nil) indicates the server is listening.
			fmt.Printf("Gripmock HTTP server is ready at %s (responded with status %d).\n", httpAddr, resp.StatusCode)
			ready = true
			break
		}
		fmt.Printf("Gripmock not ready yet at %s (error: %v), retrying in %v...\n", httpAddr, err, readinessInterval)
		time.Sleep(readinessInterval)
	}

	if !ready {
		panic(fmt.Sprintf("gripmock container failed to become ready within %v at address %s", readinessTimeout, httpAddr))
	}

	exec(gRPCMockContainer.Address("grpc"), gRPCMockContainer.Address("http"))
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
