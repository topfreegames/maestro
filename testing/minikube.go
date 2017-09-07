// maestro
// +build integration
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package testing

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strings"

	uuid "github.com/satori/go.uuid"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const minikubeHostFileName string = "MAESTRO_MINIKUBE_HOST"

func MinikubeConfig() (*rest.Config, error) {
	var err error
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(usr.HomeDir, ".kube", "config")

	config, err := clientcmd.BuildConfigFromFlags("", path)
	if err != nil {
		return nil, err
	}

	//TODO: generic way to do this
	config.TLSClientConfig.KeyFile = filepath.Join(usr.HomeDir, ".minikube", "apiserver.key")
	config.TLSClientConfig.CAFile = filepath.Join(usr.HomeDir, ".minikube", "ca.crt")
	config.TLSClientConfig.CertFile = filepath.Join(usr.HomeDir, ".minikube", "apiserver.crt")
	config.TLSClientConfig.ServerName = ""
	config.TLSClientConfig.CertData = nil
	config.TLSClientConfig.KeyData = nil
	config.TLSClientConfig.CAData = nil
	config.Username = ""
	config.Password = ""

	host, err := ioutil.ReadFile(filepath.Join("/tmp", minikubeHostFileName))
	if err != nil {
		return nil, err
	}
	config.Host = strings.TrimSpace(string(host))

	return config, nil
}

func NextJsonStr() (string, error) {
	jsonTempl := `
{
  "name": "{{.Name}}",
  "game": "game-name",
  "image": "nginx:alpine",
  "toleration": "game-name",
  "ports": [
    {
      "containerPort": 8080,
      "protocol": "TCP",
      "name": "tcp"
    }
  ],
  "limits": {
    "memory": "10Mi",
    "cpu": "10m"
  },
 "requests": {
    "memory": "10Mi",
    "cpu": "10m"
  },
  "occupiedTimeout": 300,
  "shutdownTimeout": 100,
  "autoscaling": {
    "min": 2,
    "up": {
      "delta": 1,
      "trigger": {
        "usage": 70,
        "time": 3,
        "threshold": 80
      },
      "cooldown": 1
    },
    "down": {
      "delta": 1,
      "trigger": {
        "usage": 50,
        "time": 3,
        "threshold": 80
      },
      "cooldown": 1
    }
  }
}`

	var jsonStr string
	index := struct {
		Name string
	}{}

	tmpl, err := template.New("json").Parse(jsonTempl)
	if err != nil {
		return "", err
	}

	index.Name = fmt.Sprintf("maestro-test-%s", uuid.NewV4())

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, index)
	if err != nil {
		return "", err
	}

	jsonStr = buf.String()
	return jsonStr, nil
}
