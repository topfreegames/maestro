// Copyright © 2017 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"plugin"

	"github.com/spf13/cobra"
	"github.com/topfreegames/maestro/eventforwarder"
)

var testRPCCmd = &cobra.Command{
	Use:   "test-rpc",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		p, err := plugin.Open("./bin/grpc.so")
		if err != nil {
			fmt.Printf("error loading grpc plugin: %s\n", err.Error())
			return
		}
		f, err := p.Lookup("NewForwarder")
		if err != nil {
			fmt.Printf("error loading plugin function")
		}
		ff, ok := f.(func() eventforwarder.EventForwarder)
		if !ok {
			fmt.Printf("could not get NewForwarder function")
			return
		}
		forwarder := ff()
		status, err := forwarder.Forward(config, "roomReady", map[string]interface{}{
			"game":     "rol",
			"roomId":   "someid",
			"roomType": "secso",
			"host":     "somehost",
			"port":     32,
		})
		fmt.Printf("%d\n", status)
	},
}

func init() {
	RootCmd.AddCommand(testRPCCmd)
}
