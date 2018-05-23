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
	"os/user"

	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

func newLog(cmdName string) *logrus.Logger {
	ll := logrus.InfoLevel
	switch Verbose {
	case 0:
		ll = logrus.ErrorLevel
		break
	case 1:
		ll = logrus.WarnLevel
		break
	case 3:
		ll = logrus.DebugLevel
		break
	default:
		ll = logrus.InfoLevel
	}

	log := logrus.New()
	log.Level = ll

	return log
}

func homeDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	return usr.HomeDir, nil
}

func getServerUrl() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}

	configPath := fmt.Sprintf("%s/.maestro/config.yaml", home)
	bts, err := ioutil.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	config := make(map[string]interface{})
	err = yaml.Unmarshal(bts, &config)
	if err != nil {
		return "", err
	}

	return config["url"].(string), nil
}
