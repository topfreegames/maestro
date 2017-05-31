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
	jsonLib "encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Scheduler status",
	Long:  `Returns scheduler status like state, how many rooms are running and last time it scaled.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Error: inform scheduler name")
			os.Exit(1)
		}

		log := newLog("status")

		schedulerName := args[0]

		url, err := getServerUrl()
		if err != nil {
			log.WithError(err).Fatal("error reading maestro config")
		}

		url = fmt.Sprintf("%s/scheduler/%s", url, schedulerName)
		resp, err := http.Get(url)
		if err != nil {
			log.WithError(err).Fatal("error executing GET request")
		}

		bts, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.WithError(err).Fatal("error parsing response")
		}

		body := make(map[string]interface{})
		err = jsonLib.Unmarshal(bts, &body)
		if err != nil {
			log.WithError(err).Fatal("error unmarshaling response")
		}

		bts, err = jsonLib.MarshalIndent(body, "", "\t")
		fmt.Println(string(bts))
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
