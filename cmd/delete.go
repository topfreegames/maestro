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
	"io/ioutil"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete scheduler on Maestro",
	Long: `Delete scheduler on Maestro. This will stop all rooms and remove
	configs from databases.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Error: inform scheduler name")
			os.Exit(1)
		}

		log := newLog("delete")

		schedulerName := args[0]
		log.Debugf("reading %s", schedulerName)

		url, err := getServerUrl()
		if err != nil {
			log.WithError(err).Fatal("error reading maestro config")
		}
		url = fmt.Sprintf("%s/scheduler/%s", url, schedulerName)

		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			log.WithError(err).Fatal("error reading maestro config")
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.WithError(err).Fatal("error reading maestro config")
		}
		defer resp.Body.Close()

		bts, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.WithError(err).Fatal("error reading response")
		}

		fmt.Println("Status:", resp.StatusCode)
		fmt.Println("Response:", string(bts))
	},
}

func init() {
	RootCmd.AddCommand(deleteCmd)
}
