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

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/topfreegames/maestro/cmd/managementapi"
	"github.com/topfreegames/maestro/cmd/roomsapi"
	"github.com/topfreegames/maestro/cmd/runtimewatcher"
	"github.com/topfreegames/maestro/cmd/worker"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the provided maestro component service",
}

func init() {
	startCmd.AddCommand(worker.WorkerCmd)
	startCmd.AddCommand(runtimewatcher.RuntimeWatcherCmd)
	startCmd.AddCommand(roomsapi.RoomsAPICmd)
	startCmd.AddCommand(managementapi.ManagementApiCmd)
}
