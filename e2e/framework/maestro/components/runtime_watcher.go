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
	"fmt"
	"net/http"
	"strings"
	"time"

	tc "github.com/testcontainers/testcontainers-go"

	"github.com/topfreegames/maestro/e2e/framework/maestro/helpers"
)

type RuntimeWatcherServer struct {
	compose tc.DockerCompose
}

func ProvideRuntimeWatcher(maestroPath string) (*RuntimeWatcherServer, error) {
	client := &http.Client{}

	composeFilePaths := []string{fmt.Sprintf("%s/e2e/framework/maestro/docker-compose.yml", maestroPath)}
	identifier := strings.ToLower("e2e-test")

	compose := tc.NewLocalDockerCompose(composeFilePaths, identifier)
	composeErr := compose.WithCommand([]string{"up", "-d", "--build", "runtime-watcher"}).Invoke()

	if composeErr.Error != nil {
		return nil, fmt.Errorf("failed to start runtime watcher API: %s", composeErr.Error)
	}

	err := helpers.TimedRetry(func() error {
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

	return &RuntimeWatcherServer{compose: compose}, nil
}

func (ws *RuntimeWatcherServer) Teardown() {
	ws.compose.WithCommand([]string{"rm", "-s", "-v", "-f", "runtime-watcher"}).Invoke()
}
