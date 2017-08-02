// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/worker"
)

var hostPortRange string

// workerCmd represents the worker command
var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "starts maestro worker",
	Long:  `starts maestro worker`,
	Run: func(cmd *cobra.Command, args []string) {
		ll := logrus.InfoLevel
		switch Verbose {
		case 0:
			ll = logrus.InfoLevel
		case 1:
			ll = logrus.WarnLevel
		case 3:
			ll = logrus.DebugLevel
		}

		var log = logrus.New()
		if json {
			log.Formatter = new(logrus.JSONFormatter)
		}
		log.Level = ll

		cmdL := log.WithFields(logrus.Fields{
			"source":    "workerCmd",
			"operation": "Run",
			"bind":      bind,
			"port":      port,
		})

		cmdL.Info("starting maestro worker")

		// TODO: support new relic
		mr := models.NewMixedMetricsReporter()
		w, err := worker.NewWorker(config, log, mr, incluster, kubeconfig, nil, nil, nil)
		if err != nil {
			cmdL.Fatal(err)
		}

		startHostPortRange, endHostPortRange, err := getRange(hostPortRange)
		if err != nil {
			cmdL.Fatal("invalid port range", hostPortRange, err)
		}

		err = w.Start(startHostPortRange, endHostPortRange)
		if err != nil {
			cmdL.Fatal(err)
		}
	},
}

func getRange(hostPortRange string) (int, int, error) {
	ports := strings.Split(hostPortRange, "-")
	startHostPortRange, err := strconv.Atoi(ports[0])
	if err != nil {
		return 0, 0, err
	}
	endHostPortRange, err := strconv.Atoi(ports[1])
	if err != nil {
		return 0, 0, err
	}
	return startHostPortRange, endHostPortRange, nil
}

func init() {
	workerCmd.Flags().BoolVar(&incluster, "incluster", false, "incluster mode (for running on kubernetes)")
	workerCmd.Flags().StringVar(&context, "context", "", "kubeconfig context")
	home, err := homedir.Dir()
	if err != nil {
		panic(err)
	}
	workerCmd.Flags().StringVar(&kubeconfig, "kubeconfig", fmt.Sprintf("%s/.kube/config", home), "path to the kubeconfig file (not needed if using --incluster)")
	workerCmd.Flags().StringVar(&hostPortRange, "host-port-range", "40000-60000", "range of ports to use on host machine")
	RootCmd.AddCommand(workerCmd)
}
