// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/go-redis/redis"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/topfreegames/maestro/errors"
	"github.com/topfreegames/maestro/reporters"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

var scheme = runtime.NewScheme()
var codecs = serializer.NewCodecFactory(scheme)

// TODO: setup livenessProbe
const podYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: {{.Name}}
  namespace: {{.Namespace}}
  labels:
    {{- if eq .Environment "development"}}
    app: {{.Name}}
    {{- else}}
    app: {{.Namespace}}
    {{- end}}
    heritage: maestro
    version: {{.Version}}
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
    imagePullPolicy: {{.ImagePullPolicy}}
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
        {{- else if .ValueFrom.FieldRef.FieldPath}}
        valueFrom:
          fieldRef:
            fieldPath: {{.ValueFrom.FieldRef.FieldPath}}
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
	IsTerminating   bool
	Containers      []*Container
	Version         string
	Status          v1.PodStatus
	Spec            v1.PodSpec
	NodeName        string
	Environment     string
}

// NewPod is the pod constructor
func NewPod(
	name string,
	envs []*EnvVar,
	configYaml *ConfigYAML,
	clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
	mr *MixedMetricsReporter,
) (*Pod, error) {
	pod := &Pod{
		Name:            name,
		Game:            configYaml.Game,
		Namespace:       configYaml.Name,
		ShutdownTimeout: configYaml.ShutdownTimeout,
	}

	container := &Container{
		Image:           configYaml.Image,
		ImagePullPolicy: configYaml.ImagePullPolicy,
		Name:            name,
		Env:             envs,
		Ports:           configYaml.Ports,
		Command:         configYaml.Cmd,
	}

	if configYaml.Limits != nil {
		container.Limits = &Resources{
			CPU:    configYaml.Limits.CPU,
			Memory: configYaml.Limits.Memory,
		}
	}
	if configYaml.Requests != nil {
		container.Requests = &Resources{
			CPU:    configYaml.Requests.CPU,
			Memory: configYaml.Requests.Memory,
		}
	}
	pod.Containers = []*Container{container}
	err := pod.configureHostPorts(configYaml, clientset, redisClient, mr)

	if err == nil {
		reporters.Report(reportersConstants.EventGruNew, map[string]interface{}{
			reportersConstants.TagGame:      configYaml.Game,
			reportersConstants.TagScheduler: configYaml.Name,
		})
	}

	return pod, err
}

// NewPodWithContainers returns a pod with multiple containers
func NewPodWithContainers(
	name string,
	containers []*Container,
	configYaml *ConfigYAML,
	clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
	mr *MixedMetricsReporter,
) (*Pod, error) {
	pod := &Pod{
		Game:            configYaml.Game,
		Name:            name,
		Namespace:       configYaml.Name,
		ShutdownTimeout: configYaml.ShutdownTimeout,
		Containers:      containers,
	}
	err := pod.configureHostPorts(configYaml, clientset, redisClient, mr)
	if err == nil {
		reporters.Report(reportersConstants.EventGruNew, map[string]interface{}{
			reportersConstants.TagGame:      configYaml.Game,
			reportersConstants.TagScheduler: configYaml.Name,
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

// SetVersion sets the pod version equals to the scheduler version
func (p *Pod) SetVersion(version string) {
	p.Version = version
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

	k8sPod := v1.Pod{}
	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader(buf.Bytes()), len(buf.Bytes())).Decode(&k8sPod)
	if err != nil {
		return nil, errors.NewKubernetesError("error unmarshaling pod", err)
	}

	pod, err := clientset.CoreV1().Pods(p.Namespace).Create(&k8sPod)
	if err != nil {
		return nil, errors.NewKubernetesError("create pod error", err)
	}

	return pod, nil
}

// Delete deletes a pod from kubernetes.
func (p *Pod) Delete(clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
	reason string,
	configYaml *ConfigYAML,
) error {
	err := clientset.CoreV1().Pods(p.Namespace).Delete(p.Name, &metav1.DeleteOptions{})
	if err != nil {
		return errors.NewKubernetesError("delete pod error", err)
	}

	if err == nil {
		reporters.Report(reportersConstants.EventGruDelete, map[string]interface{}{
			reportersConstants.TagGame:      p.Game,
			reportersConstants.TagScheduler: p.Namespace,
			reportersConstants.TagReason:    reason,
		})
	}

	return nil
}

func getContainerWithName(name string, pod *Pod) v1.Container {
	var container v1.Container

	for _, container = range pod.Spec.Containers {
		if container.Name == name {
			break
		}
	}

	return container
}

func (p *Pod) configureHostPorts(
	configYaml *ConfigYAML,
	clientset kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
	mr *MixedMetricsReporter,
) error {
	pod, err := GetPodFromRedis(redisClient, mr, p.Name, p.Namespace)
	if err != nil {
		return errors.NewKubernetesError("could not access kubernetes", err)
	} else if err == nil && pod != nil {
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

	//pod not found, so give new ports if necessary
	podHasPorts := false

	for _, container := range p.Containers {
		if container.Ports != nil && len(container.Ports) > 0 {
			podHasPorts = true
			break
		}
	}

	if !podHasPorts {
		return nil
	}

	start, end, err := GetPortRange(configYaml, redisClient)
	if err != nil {
		return fmt.Errorf("error reading global port range from redis: %s", err.Error())
	}

	for _, container := range p.Containers {
		ports := GetRandomPorts(start, end, len(container.Ports))
		containerPorts := make([]*Port, len(container.Ports))
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

// MarshalToRedis stringfy pod object with the information to save on podMap redis key
func (p *Pod) MarshalToRedis() ([]byte, error) {

	return json.Marshal(map[string]interface{}{
		"name":          p.Name,
		"status":        p.Status,
		"version":       p.Version,
		"nodeName":      p.NodeName,
		"isTerminating": p.IsTerminating,
		"spec":          p.Spec,
	})
}

// UnmarshalFromRedis loads pod string from redis podMap key in a *Pod object
func (p *Pod) UnmarshalFromRedis(pod string) error {
	return json.Unmarshal([]byte(pod), p)
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

// IsPodReady returns true if pod is ready
func IsPodReady(pod *Pod) bool {
	status := &pod.Status
	if status == nil {
		return false
	}

	for _, condition := range status.Conditions {
		if condition.Type == v1.PodReady {
			return condition.Status == v1.ConditionTrue
		}
	}

	return false
}

func IsPodTerminating(pod *v1.Pod) bool {
	return pod.ObjectMeta.DeletionTimestamp != nil
}

// ValidatePodWaitingState returns nil if pod waiting reson is valid and error otherwise
// Errors checked:
// - ErrImageNeverPull
// - ErrImagePullBackOff
// - ErrInvalidImageName
func ValidatePodWaitingState(pod *Pod) error {

	for _, invalidState := range InvalidPodWaitingStates {
		status := &pod.Status
		if checkWaitingReason(status, invalidState) {
			return fmt.Errorf("one or more containers in pod are in %s", invalidState)
		}
	}

	return nil
}

func checkWaitingReason(status *v1.PodStatus, reason string) bool {
	if status == nil {
		return false
	}

	for _, containerStatus := range status.ContainerStatuses {
		state := containerStatus.State
		if state.Waiting != nil && state.Waiting.Reason == reason {
			return true
		}
	}

	return false
}

// PodPending returns true if pod is with status Pending.
// In this case, also returns reason for being pending and message.
func PodPending(pod *Pod) (isPending bool, reason, message string) {
	for _, condition := range pod.Status.Conditions {
		if condition.Status == v1.ConditionFalse {
			return true, condition.Reason, condition.Message
		}
	}

	return false, "", ""
}

// IsUnitTest returns true if pod was created using fake client-go
// and is not running in a kubernetes cluster.
func IsUnitTest(pod *Pod) bool {
	return len(pod.Status.Phase) == 0
}

// GetPodMapRedisKey gets the key for string that keeps the pod map from kube on redis
func GetPodMapRedisKey(schedulerName string) string {
	return fmt.Sprintf("scheduler:%s:podMap", schedulerName)
}

// GetPodMapFromRedis loads the pod map from redis
func GetPodMapFromRedis(
	redisClient redisinterfaces.RedisClient,
	mr *MixedMetricsReporter,
	schedulerName string,
) (podMap map[string]*Pod, err error) {
	pipe := redisClient.TxPipeline()
	cmd := pipe.HGetAll(GetPodMapRedisKey(schedulerName))
	err = mr.WithSegment(SegmentPipeExec, func() error {
		var err error
		_, err = pipe.Exec()
		return err
	})
	if err != nil {
		return nil, err
	}

	podMap = map[string]*Pod{}

	for podName, podStr := range cmd.Val() {
		pod := &Pod{}
		err = pod.UnmarshalFromRedis(podStr)
		if err != nil {
			return nil, err
		}

		podMap[podName] = pod
	}

	return podMap, err
}

// GetPodCountFromRedis returns the pod count from redis podMap key
func GetPodCountFromRedis(
	redisClient redisinterfaces.RedisClient,
	mr *MixedMetricsReporter,
	schedulerName string,
) (count int, err error) {
	pipe := redisClient.TxPipeline()
	cmd := pipe.HLen(GetPodMapRedisKey(schedulerName))
	err = mr.WithSegment(SegmentPipeExec, func() error {
		var err error
		_, err = pipe.Exec()
		return err
	})
	if err != nil {
		return 0, err
	}

	return int(cmd.Val()), err
}

// GetPodFromRedis returns a specific pod from redis
func GetPodFromRedis(
	redisClient redisinterfaces.RedisClient,
	mr *MixedMetricsReporter,
	podName, schedulerName string,
) (pod *Pod, err error) {
	podStr, err := redisClient.HGet(GetPodMapRedisKey(schedulerName), podName).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	pod = &Pod{}
	err = pod.UnmarshalFromRedis(podStr)

	return pod, err
}

// AddToPodMap adds a pod to redis podMap key
func AddToPodMap(
	redisClient redisinterfaces.RedisClient,
	mr *MixedMetricsReporter,
	pod *Pod,
	schedulerName string,
) error {
	// convert Pod to []byte
	podStr, err := pod.MarshalToRedis()
	if err != nil {
		return err
	}

	// Add pod to redis
	_, err = redisClient.HMSet(
		GetPodMapRedisKey(schedulerName),
		map[string]interface{}{
			pod.Name: podStr,
		},
	).Result()

	return err
}

// RemoveFromPodMap removes a pod from redis podMap key
func RemoveFromPodMap(
	redisClient redisinterfaces.RedisClient,
	mr *MixedMetricsReporter,
	podName, schedulerName string,
) error {
	// Remove pod from redis
	pipe := redisClient.TxPipeline()
	pipe.HDel(GetPodMapRedisKey(schedulerName), podName)
	_, err := pipe.Exec()
	return err
}
