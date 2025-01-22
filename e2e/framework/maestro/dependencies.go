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
	"errors"
	"fmt"
	"strings"
	"time"

	tc "github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/topfreegames/maestro/e2e/framework/maestro/exec"
	"github.com/topfreegames/maestro/e2e/framework/maestro/helpers"
)

type dependencies struct {
	KubeconfigPath string
	RedisAddress   string
	KubeAddress    string
}

// TODO(gabrielcorado): spin up the dependencies using different ports from dev
// environment to avoid conflicts.
func provideDependencies(ctx context.Context, maestroPath string, compose tc.ComposeStack) (*dependencies, error) {
	err := compose.Down(ctx, tc.RemoveOrphans(true), tc.RemoveVolumes(true), tc.RemoveImagesAll)
	if err != nil {
		return nil, fmt.Errorf("failed to tear down running containers: %w", err)
	}

	services := []string{"postgres", "redis", "k3s_agent", "k3s_server"}
	err = compose.Up(context.Background(), tc.RunServices(services...), tc.RemoveOrphans(true), tc.Wait(true))
	if err != nil {
		return nil, fmt.Errorf("failed to start dependencies: %w", err)
	}

	migrateErr := helpers.TimedRetry(func() error {
		cmd, err := exec.ExecGoCmd(
			maestroPath,
			[]string{
				"MAESTRO_MIGRATION_PATH=file://internal/service/migrations",
			},
			"main.go", "migrate",
		)
		if err != nil {
			return err
		}

		output, err := cmd.ReadOutput()
		if err != nil {
			return err
		}
		if strings.Contains(string(output), "migration completed") || strings.Contains(string(output), "database schema already up to date") {
			return nil
		}

		return errors.New("migration not ready")
	}, time.Second, 2*time.Minute)

	if migrateErr != nil {
		return nil, fmt.Errorf("failed to migrate database: %s", migrateErr)
	}

	return &dependencies{
		KubeconfigPath: fmt.Sprintf("%s/kubeconfig/kubeconfig.yaml", maestroPath),
		KubeAddress:    "https://127.0.0.1:6443",
		RedisAddress:   "redis://127.0.0.1:6379/0",
	}, nil
}
