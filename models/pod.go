// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"bytes"
	"strings"
	"text/template"

	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/reporters"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
)

// TODO: setup livenessProbe
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
{{- if .NodeToleration }}
  tolerations:
  - key: "dedicated"
    operator: "Equal"
    value: {{.NodeToleration}}
    effect: "NoSchedule"
{{- end}}
{{- if .NodeAffinity }}
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: {{.NodeAffinity}}
            operator: In
            values: ["true"]
{{- end}}
  containers:
  - name: {{.Name}}
    image: {{.Image}}
    hostNetwork: "true"
    ports:
      {{range .Ports}}
      - containerPort: {{.ContainerPort}}
        hostPort: {{.HostPort}}
        protocol: {{.Protocol}}
        name: "{{.Name}}"
      {{end}}
    resources:
      requests:
        {{- if .ResourcesRequestsCPU}}
        cpu: {{.ResourcesRequestsCPU}}
        {{- end}}
        {{- if .ResourcesRequestsMemory}}
        memory: {{.ResourcesRequestsMemory}}
        {{- end}}
      limits:
        {{- if .ResourcesLimitsCPU}}
        cpu: {{.ResourcesLimitsCPU}}
        {{- end}}
        {{- if .ResourcesLimitsMemory}}
        memory: {{.ResourcesLimitsMemory}}
        {{- end}}
    env:
      {{range .Env}}
      - name: {{.Name}}
        {{- if .ValueFrom.SecretKeyRef.Name}}
        valueFrom:
          {{- with .ValueFrom}}
          secretKeyRef:
            name: {{.SecretKeyRef.Name}}
            key: {{.SecretKeyRef.Key}}
          {{- end}}
        {{- else}}
        value: "{{.Value}}"
        {{- end}}
      {{end}}
    {{- if .Command }}
    command:
      {{range .Command}}
      - {{.}}
      {{- end}}
    {{- end}}
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
	NodeAffinity            string
	NodeToleration          string
}

// NewPod is the pod constructor
func NewPod(
	game, image, name, namespace string,
	limits, requests *Resources,
	shutdownTimeout int,
	ports []*Port,
	command []string,
	env []*EnvVar,
	clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
) (*Pod, error) {
	pod := &Pod{
		Command:         command,
		Env:             env,
		Game:            game,
		Image:           image,
		Name:            name,
		Namespace:       namespace,
		Ports:           ports,
		ShutdownTimeout: shutdownTimeout,
	}
	if limits != nil {
		pod.ResourcesLimitsCPU = limits.CPU
		pod.ResourcesLimitsMemory = limits.Memory
	}
	if requests != nil {
		pod.ResourcesRequestsCPU = requests.CPU
		pod.ResourcesRequestsMemory = requests.Memory
	}
	err := pod.configureHostPorts(clientset, redisClient)

	if err == nil {
		reporters.GetInstance().Report(reportersConstants.EventGruNew, map[string]string{
			"game":      game,
			"scheduler": namespace,
		})
	}

	return pod, err
}

func (p *Pod) SetAffinity(affinity string) {
	p.NodeAffinity = affinity
}

func (p *Pod) SetToleration(toleration string) {
	p.NodeToleration = toleration
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
func (p *Pod) Delete(clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
	reason string) error {
	err := clientset.CoreV1().Pods(p.Namespace).Delete(p.Name, &metav1.DeleteOptions{})
	if err != nil {
		return errors.NewKubernetesError("delete pod error", err)
	}
	err = RetrievePorts(redisClient, p.Ports)
	if err != nil {
		//TODO: try again?
	}
	if err == nil {
		reporters.GetInstance().Report(reportersConstants.EventGruDelete, map[string]string{
			"game":      p.Game,
			"scheduler": p.Namespace,
			"reason":    reason,
		})
	}

	return nil
}

func (p *Pod) configureHostPorts(clientset kubernetes.Interface, redisClient redisinterfaces.RedisClient) error {
	pod, err := clientset.CoreV1().Pods(p.Namespace).Get(p.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return errors.NewKubernetesError("could not access kubernetes", err)
	} else if err == nil {
		//pod exists, so just retrieve ports
		p.Ports = make([]*Port, len(pod.Spec.Containers[0].Ports))
		for i, port := range pod.Spec.Containers[0].Ports {
			p.Ports[i] = &Port{
				ContainerPort: int(port.ContainerPort),
				Name:          port.Name,
				HostPort:      int(port.HostPort),
				Protocol:      string(port.Protocol),
			}
		}
		return nil
	}

	//pod not found, so give new ports
	ports, err := GetFreePorts(redisClient, len(p.Ports))
	if err != nil {
		return err
	}
	podPorts := make([]*Port, len(ports))
	for i, port := range ports {
		podPorts[i] = &Port{
			ContainerPort: p.Ports[i].ContainerPort,
			Name:          p.Ports[i].Name,
			HostPort:      port,
			Protocol:      p.Ports[i].Protocol,
		}
	}
	p.Ports = podPorts
	return nil
}

//PodExists returns true if a pod exists on namespace
// returns false if it doesn't
// returns false and a error if an error occurs
func PodExists(
	name, namespace string,
	clientset kubernetes.Interface,
) (bool, error) {
	_, err := clientset.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	if strings.Contains(err.Error(), "not found") {
		return false, nil
	}
	return false, err
}
