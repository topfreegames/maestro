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

package maestro

import (
	"context"
	"fmt"
	"strings"

	tc "github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/topfreegames/maestro/e2e/framework/maestro/components"
	"github.com/topfreegames/maestro/e2e/framework/maestro/servermocks"
)

type MaestroInstance struct {
	path                 string
	compose              tc.ComposeStack
	Deps                 *dependencies
	ServerMocks          *servermocks.ServerMocks
	WorkerServer         *components.WorkerServer
	ManagementApiServer  *components.ManagementApiServer
	RoomsApiServer       *components.RoomsApiServer
	RuntimeWatcherServer *components.RuntimeWatcherServer
}

func ProvideMaestro() (*MaestroInstance, error) {
	var err error

	path, err := getMaestroPath()
	if err != nil {
		return nil, err
	}

	composeFilePaths := []string{fmt.Sprintf("%s/e2e/framework/maestro/docker-compose.yml", path)}
	identifier := strings.ToLower("e2e-test")

	compose, err := tc.NewDockerComposeWith(tc.WithStackFiles(composeFilePaths...), tc.StackIdentifier(identifier))
	if err != nil {
		return nil, fmt.Errorf("failed to create docker compose instance: %w", err)
	}

	ctx := context.Background()

	dependencies, err := provideDependencies(ctx, path, compose)
	if err != nil {
		return nil, fmt.Errorf("failed to start dependencies: %s", err)
	}

	serverMocks, err := servermocks.ProvideServerMocks(ctx, compose)
	if err != nil {
		return nil, fmt.Errorf("failed to start server mocks: %s", err)
	}

	roomsApiInstance, err := components.ProvideRoomsApi(ctx, compose)
	if err != nil {
		return nil, fmt.Errorf("failed to start rooms api: %s", err)
	}

	managementApiInstance, err := components.ProvideManagementApi(ctx, compose)
	if err != nil {
		return nil, fmt.Errorf("failed to start worker: %s", err)
	}

	workerInstance, err := components.ProvideWorker(ctx, compose)
	if err != nil {
		return nil, fmt.Errorf("failed to start worker: %s", err)
	}

	runtimeWatcherInstance, err := components.ProvideRuntimeWatcher(ctx, compose)
	if err != nil {
		return nil, fmt.Errorf("failed to start runtime watcher: %s", err)
	}

	return &MaestroInstance{
		"",
		compose,
		dependencies,
		serverMocks,
		workerInstance,
		managementApiInstance,
		roomsApiInstance,
		runtimeWatcherInstance,
	}, nil
}

func (mi *MaestroInstance) Teardown() {
	_ = mi.compose.Down(context.Background(), tc.RemoveOrphans(true), tc.RemoveVolumes(true))
}
