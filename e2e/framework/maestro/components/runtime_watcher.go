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
	"time"

	"github.com/docker/compose/v2/pkg/api"
	tc "github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/topfreegames/maestro/e2e/framework/maestro/helpers"
)

type RuntimeWatcherServer struct{}

func ProvideRuntimeWatcher(ctx context.Context, compose tc.ComposeStack) (*RuntimeWatcherServer, error) {
	client := &http.Client{}

	services := compose.Services()
	services = append(services, "runtime-watcher")

	err := compose.Up(ctx, tc.RunServices(services...), tc.Recreate(api.RecreateNever), tc.RecreateDependencies(api.RecreateNever), tc.Wait(true))
	if err != nil {
		return nil, fmt.Errorf("failed to start runtime-watcher: %w", err)
	}

	err = helpers.TimedRetry(func() error {
		res, err := client.Get("http://localhost:8061/healthz")
		if err != nil {
			return err
		}

		if res != nil && res.StatusCode == http.StatusOK {
			return nil
		}

		return fmt.Errorf("not ready")
	}, time.Second, 120*time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to reach runtime watcher API: %s", err)
	}

	return &RuntimeWatcherServer{}, nil
}
