// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"bytes"
	"text/template"

	"github.com/topfreegames/maestro/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
)

// TODO: setup livenessProbe
// TODO: setup tolerations
const podYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: {{.Name}}
  namespace: {{.Namespace}}
  labels:
    app: {{.Name}}
spec:
  terminationGracePeriodSeconds: {{.ShutdownTimeout}}
  tolerations:
  - key: "game"
    operator: "Equal"
    value: {{.Game}}
    effect: "NoSchedule"
  containers:
  - name: {{.Name}}
    image: {{.Image}}
    ports:
      {{range .Ports}}
      - containerPort: {{.ContainerPort}}
      {{end}}
    resources:
      requests:
        cpu: {{.ResourcesRequestsCPU}}
        memory: {{.ResourcesRequestsMemory}}
      limits:
        cpu: {{.ResourcesLimitsCPU}}
        memory: {{.ResourcesLimitsMemory}}
    env:
      {{range .Env}}
      - name: {{.Name}}
        value: "{{.Value}}"
      {{end}}
    command: {{.Command}}
`

// Pod represents a pod
type Pod struct {
	Command                 []string
	Env                     []*EnvVar
	Game                    string
	Image                   string
	Name                    string
	Namespace               string
	Ports                   []*Port
	ResourcesLimitsCPU      string
	ResourcesLimitsMemory   string
	ResourcesRequestsCPU    string
	ResourcesRequestsMemory string
	ShutdownTimeout         int
}

// NewPod is the pod constructor
func NewPod(
	game, image, name, namespace,
	resourcesLimitsCPU, resourcesLimitsMemory,
	resourcesRequestsCPU, resourcesRequestsMemory string,
	shutdownTimeout int,
	ports []*Port,
	command []string,
	env []*EnvVar) *Pod {
	return &Pod{
		Command:                 command,
		Env:                     env,
		Game:                    game,
		Image:                   image,
		Name:                    name,
		Namespace:               namespace,
		Ports:                   ports,
		ResourcesLimitsCPU:      resourcesLimitsCPU,
		ResourcesLimitsMemory:   resourcesLimitsMemory,
		ResourcesRequestsCPU:    resourcesRequestsCPU,
		ResourcesRequestsMemory: resourcesRequestsMemory,
		ShutdownTimeout:         shutdownTimeout,
	}
}

// Create creates a pod in Kubernetes
func (p *Pod) Create(clientset kubernetes.Interface) (*v1.Pod, error) {
	tmpl, err := template.New("create").Parse(podYaml)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, p)
	if err != nil {
		return nil, err
	}

	decoder := api.Codecs.UniversalDecoder()
	obj, _, err := decoder.Decode(buf.Bytes(), nil, nil)
	if err != nil {
		return nil, err
	}

	src := obj.(*api.Pod)
	dst := &v1.Pod{}

	err = api.Scheme.Convert(src, dst, 0)
	if err != nil {
		return nil, err
	}

	pod, err := clientset.CoreV1().Pods(p.Namespace).Create(dst)
	if err != nil {
		return nil, errors.NewKubernetesError("create pod error", err)
	}
	return pod, nil
}

// Delete deletes a pod from kubernetes
func (p *Pod) Delete(clientset kubernetes.Interface) error {
	err := clientset.CoreV1().Pods(p.Namespace).Delete(p.Name, &metav1.DeleteOptions{})
	if err != nil {
		return errors.NewKubernetesError("delete pod error", err)
	}

	return nil
}
