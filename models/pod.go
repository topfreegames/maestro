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
  annotations:
    cluster-autoscaler.kubernetes.io/safe-to-evict: "true"
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
  {{range .Containers}}
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
      {{- with .Requests}}
      requests:
        {{- if .CPU}}
        cpu: {{.CPU}}
        {{- end}}
        {{- if .Memory}}
        memory: {{.Memory}}
        {{- end}}
      {{- end}}
      {{- with .Limits}}
      limits:
        {{- if .CPU}}
        cpu: {{.CPU}}
        {{- end}}
        {{- if .Memory}}
        memory: {{.Memory}}
        {{- end}}
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
  {{end}}
`

// Pod represents a pod
type Pod struct {
	Game            string
	Name            string
	Namespace       string
	ShutdownTimeout int
	NodeAffinity    string
	NodeToleration  string
	Containers      []*Container
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
		Game:            game,
		Name:            name,
		Namespace:       namespace,
		ShutdownTimeout: shutdownTimeout,
	}

	container := &Container{
		Image:   image,
		Name:    name,
		Env:     env,
		Ports:   ports,
		Command: command,
	}

	if limits != nil {
		container.Limits = &Resources{
			CPU:    limits.CPU,
			Memory: limits.Memory,
		}
	}
	if requests != nil {
		container.Requests = &Resources{
			CPU:    requests.CPU,
			Memory: requests.Memory,
		}
	}
	pod.Containers = []*Container{container}
	err := pod.configureHostPorts(clientset, redisClient)

	if err == nil {
		reporters.Report(reportersConstants.EventGruNew, map[string]string{
			reportersConstants.TagGame:      game,
			reportersConstants.TagScheduler: namespace,
		})
	}

	return pod, err
}

// NewDefaultPod creates a pod only with necessary information
// Should not be used to create a real pod
func NewDefaultPod(
	game, name, namespace string,
	clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
) (*Pod, error) {
	return NewPod(
		game, "", name, namespace,
		nil, nil, 0, []*Port{}, []string{}, []*EnvVar{}, clientset, redisClient)
}

// NewPodWithContainers returns a pod with multiple containers
func NewPodWithContainers(
	game, name, namespace string,
	shutdownTimeout int,
	containers []*Container,
	clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
) (*Pod, error) {
	pod := &Pod{
		Game:            game,
		Name:            name,
		Namespace:       namespace,
		ShutdownTimeout: shutdownTimeout,
		Containers:      containers,
	}
	err := pod.configureHostPorts(clientset, redisClient)
	if err == nil {
		reporters.Report(reportersConstants.EventGruNew, map[string]string{
			reportersConstants.TagGame:      game,
			reportersConstants.TagScheduler: namespace,
		})
	}

	return pod, err
}

//SetAffinity sets kubernetes Affinity on pod
func (p *Pod) SetAffinity(affinity string) {
	p.NodeAffinity = affinity
}

//SetToleration sets kubernetes Toleration on pod
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
	kubePod, err := clientset.CoreV1().Pods(p.Namespace).Get(p.Name, metav1.GetOptions{})
	if err != nil {
		return errors.NewKubernetesError("error getting pod to delete", err)
	}

	err = clientset.CoreV1().Pods(p.Namespace).Delete(p.Name, &metav1.DeleteOptions{})
	if err != nil {
		return errors.NewKubernetesError("delete pod error", err)
	}

	for _, container := range kubePod.Spec.Containers {
		RetrieveV1Ports(redisClient, container.Ports)
	}
	if err == nil {
		reporters.Report(reportersConstants.EventGruDelete, map[string]string{
			reportersConstants.TagGame:      p.Game,
			reportersConstants.TagScheduler: p.Namespace,
			reportersConstants.TagReason:    reason,
		})
	}

	return nil
}

func getContainerWithName(name string, pod *v1.Pod) v1.Container {
	var container v1.Container

	for _, container = range pod.Spec.Containers {
		if container.Name == name {
			break
		}
	}

	return container
}

func (p *Pod) configureHostPorts(clientset kubernetes.Interface, redisClient redisinterfaces.RedisClient) error {
	pod, err := clientset.CoreV1().Pods(p.Namespace).Get(p.Name, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return errors.NewKubernetesError("could not access kubernetes", err)
	} else if err == nil {
		//pod exists, so just retrieve ports
		for _, container := range p.Containers {
			podContainer := getContainerWithName(container.Name, pod)
			container.Ports = make([]*Port, len(podContainer.Ports))

			for i, port := range podContainer.Ports {
				container.Ports[i] = &Port{
					ContainerPort: int(port.ContainerPort),
					Name:          port.Name,
					HostPort:      int(port.HostPort),
					Protocol:      string(port.Protocol),
				}
			}
			break
		}
		return nil
	}

	//pod not found, so give new ports
	for _, container := range p.Containers {
		ports, err := GetFreePorts(redisClient, len(container.Ports))
		if err != nil {
			return err
		}
		containerPorts := make([]*Port, len(ports))
		for i, port := range ports {
			containerPorts[i] = &Port{
				ContainerPort: container.Ports[i].ContainerPort,
				Name:          container.Ports[i].Name,
				HostPort:      port,
				Protocol:      container.Ports[i].Protocol,
			}
		}
		container.Ports = containerPorts
	}
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
