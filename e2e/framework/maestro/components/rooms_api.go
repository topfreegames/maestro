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

package components

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	tc "github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/topfreegames/maestro/e2e/framework/maestro/helpers"
)

type RoomsApiServer struct {
	Address                  string
	ContainerInternalAddress string
}

func ProvideRoomsApi(ctx context.Context, compose tc.ComposeStack) (*RoomsApiServer, error) {
	address := "http://localhost:8070"
	client := &http.Client{}

	services := compose.Services()
	services = append(services, "rooms-api")

	err := compose.Up(ctx, tc.RunServices(services...), tc.Recreate(api.RecreateNever), tc.RecreateDependencies(api.RecreateNever), tc.Wait(true))
	if err != nil {
		return nil, fmt.Errorf("failed to start rooms-api: %w", err)
	}

	err = helpers.TimedRetry(func() error {
		res, err := client.Get("http://localhost:8071/healthz")
		if err != nil {
			return err
		}

		if res != nil && res.StatusCode == http.StatusOK {
			return nil
		}

		return fmt.Errorf("not ready")
	}, time.Second, 120*time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to reach rooms API: %s", err)
	}

	container, err := compose.ServiceContainer(ctx, "rooms-api")
	if err != nil {
		return nil, fmt.Errorf("unable to get rooms api container: %s", err)
	}

	networks, err := container.Networks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get rooms-api container networks: %w", err)
	}

	cli, err := docker.NewClientWithOpts()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	networkInspect, err := cli.NetworkInspect(ctx, networks[0], network.InspectOptions{Verbose: true, Scope: ""})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect rooms-api network: %w", err)
	}

	output := networkInspect.IPAM.Config[0].Gateway
	roomsApiIP := strings.Trim(strings.TrimSuffix(output, "\n"), "'")
	internalAddress := fmt.Sprintf("%s:8070", roomsApiIP)

	return &RoomsApiServer{Address: address, ContainerInternalAddress: internalAddress}, nil
}
