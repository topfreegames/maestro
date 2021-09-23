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

package exec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
)

type Cmd struct {
	execCmd *exec.Cmd
	output  *bytes.Buffer
}

func (c *Cmd) Kill() {
	syscall.Kill(-c.execCmd.Process.Pid, syscall.SIGKILL)
}

func (c *Cmd) ReadOutput() ([]byte, error) {
	workerOutput, err := io.ReadAll(c.output)
	if err != nil {
		return nil, fmt.Errorf("failed to read command output: %s", err)
	}

	return workerOutput, nil
}

func ExecGoCmd(dir string, env []string, externalArgs ...string) (*Cmd, error) {
	args := []string{"run"}
	for _, arg := range externalArgs {
		args = append(args, arg)
	}

	c := &Cmd{
		execCmd: exec.Command("go", args...),
		output:  new(bytes.Buffer),
	}

	// TODO(gabrielcorado): receive it from somewhere.
	cmdEnv := []string{
		"MAESTRO_ADAPTERS_SCHEDULERSTORAGE_POSTGRES_URL=postgres://maestro:maestro@localhost:5432/maestro?sslmode=disable",
		"MAESTRO_OPERATIONFLOW_REDIS_URL=redis://localhost:6379/0",
		"MAESTRO_OPERATIONSTORAGE_REDIS_URL=redis://localhost:6379/0",
	}

	c.execCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.execCmd.Stdout = c.output
	c.execCmd.Stderr = c.output
	c.execCmd.Dir = dir
	c.execCmd.Env = append(
		os.Environ(),
		append(env, cmdEnv...)...,
	)

	err := c.execCmd.Start()
	if err != nil {
		c.Kill()
		return nil, fmt.Errorf("failed to start command: %s", err)
	}

	return c, nil
}

func ExecSysCmd(dir string, command string, args ...string) (*Cmd, error) {
	execCmd := exec.Command(command, args...)

	c := &Cmd{
		execCmd: execCmd,
		output:  new(bytes.Buffer),
	}

	c.execCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.execCmd.Stdout = c.output
	c.execCmd.Stderr = c.output
	c.execCmd.Dir = dir

	err := c.execCmd.Run()
	if err != nil {
		c.Kill()
		return nil, fmt.Errorf("failed to run command: %s", err)
	}

	return c, nil
}
