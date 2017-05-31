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
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	jsonLib "encoding/json"

	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update scheduler on Maestro",
	Long: `Update scheduler on Maestro will update config on databases and, 
	if necessary, delete and create pods and services following new configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Error: inform scheduler config file path")
			os.Exit(1)
		}

		log := newLog("update")

		filePath := args[0]
		log.Debugf("reading %s", filePath)

		bts, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.WithError(err).Fatal("error reading scheduler config")
		}

		config := make(map[string]interface{})
		err = jsonLib.Unmarshal(bts, &config)
		if err != nil {
			log.WithError(err).Fatal("error unmarshaling scheduler config")
		}
		schedulerName, ok := config["name"].(string)
		if !ok {
			log.WithError(err).Fatal("scheduler name should be a string")
		}

		reader := bytes.NewReader(bts)
		url, err := getServerUrl()
		if err != nil {
			log.WithError(err).Fatal("error reading maestro config")
		}
		url = fmt.Sprintf("%s/scheduler/%s", url, schedulerName)

		req, err := http.NewRequest("PUT", url, reader)
		if err != nil {
			log.WithError(err).Fatal("error reading maestro config")
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.WithError(err).Fatal("error reading maestro config")
		}
		defer resp.Body.Close()

		bts, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.WithError(err).Fatal("error reading response")
		}

		fmt.Println("Status:", resp.StatusCode)
		fmt.Println("Response:", string(bts))
	},
}

func init() {
	RootCmd.AddCommand(updateCmd)
}
