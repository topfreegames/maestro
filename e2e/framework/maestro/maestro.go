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
	"fmt"

	"github.com/topfreegames/maestro/e2e/framework/maestro/components"
)

type MaestroInstance struct {
	path                string
	Deps                *dependencies
	WorkerServer        *components.WorkerServer
	ManagementApiServer *components.ManagementApiServer
	RoomsApiServer      *components.RoomsApiServer
}

func ProvideMaestro() (*MaestroInstance, error) {
	var err error

	path, err := getMaestroPath()
	if err != nil {
		return nil, err
	}

	dependencies, err := provideDependencies(path)
	if err != nil {
		return nil, fmt.Errorf("failed to start dependencies: %s", err)
	}

	workerInstance, err := components.ProvideWorker(path)
	if err != nil {
		return nil, fmt.Errorf("failed to start worker: %s", err)
	}

	managementApiInstance, err := components.ProvideManagementApi(path)
	if err != nil {
		return nil, fmt.Errorf("failed to start worker: %s", err)
	}

	roomsApiInstance, err := components.ProvideRoomsApi(path)
	if err != nil {
		return nil, fmt.Errorf("failed to start rooms api: %s", err)
	}

	return &MaestroInstance{
		"",
		dependencies,
		workerInstance,
		managementApiInstance,
		roomsApiInstance,
	}, nil
}

func (mi *MaestroInstance) Teardown() {
	mi.ManagementApiServer.Teardown()
	mi.WorkerServer.Teardown()

	// TODO(gabrielcorado): add a flag to not stop depedencies during
	// development (this will make the e2e run way faster).
	mi.Deps.Teardown()
}
