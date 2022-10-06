// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//go:build unit
// +build unit

package kubernetes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/core/entities/game_room"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// small helper used to return a pointer to a int64
func int64Pointer(n int64) *int64 {
	return &n
}

func TestConvertContainerResources(t *testing.T) {
	cases := map[string]struct {
		containerResources game_room.ContainerResources
		expectedKubernetes v1.ResourceList
		withError          bool
	}{
		"only with memory": {
			containerResources: game_room.ContainerResources{
				Memory: "100Mi",
			},
			expectedKubernetes: v1.ResourceList{
				v1.ResourceMemory: *resource.NewQuantity(100*1024*1024, resource.BinarySI),
			},
		},
		"only with CPU": {
			containerResources: game_room.ContainerResources{
				CPU: "100m",
			},
			expectedKubernetes: v1.ResourceList{
				v1.ResourceCPU: *resource.NewMilliQuantity(100, resource.DecimalSI),
			},
		},
		"with memory and CPU": {
			containerResources: game_room.ContainerResources{
				Memory: "100Mi",
				CPU:    "100m",
			},
			expectedKubernetes: v1.ResourceList{
				v1.ResourceMemory: *resource.NewQuantity(100*1024*1024, resource.BinarySI),
				v1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
			},
		},
		"with invalid memory": {
			containerResources: game_room.ContainerResources{
				Memory: "100abc",
			},
			withError: true,
		},
		"with invalid CPU": {
			containerResources: game_room.ContainerResources{
				CPU: "100abc",
			},
			withError: true,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := convertContainerResources(test.containerResources)
			if test.withError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.True(t, res[v1.ResourceMemory].Equal(test.expectedKubernetes[v1.ResourceMemory]))
			require.True(t, res[v1.ResourceCPU].Equal(test.expectedKubernetes[v1.ResourceCPU]))
		})
	}
}

func TestConvertContainerPort(t *testing.T) {
	cases := map[string]struct {
		containerPort      game_room.ContainerPort
		expectedKubernetes v1.ContainerPort
		withError          bool
	}{
		"tcp port": {
			containerPort: game_room.ContainerPort{
				Name:     "testtcp",
				Protocol: "tcp",
				Port:     5555,
			},
			expectedKubernetes: v1.ContainerPort{
				Name:          "testtcp",
				Protocol:      v1.ProtocolTCP,
				ContainerPort: 5555,
			},
		},
		"udp port": {
			containerPort: game_room.ContainerPort{
				Name:     "testudp",
				Protocol: "udp",
				Port:     5555,
				HostPort: 25555,
			},
			expectedKubernetes: v1.ContainerPort{
				Name:          "testudp",
				Protocol:      v1.ProtocolUDP,
				ContainerPort: 5555,
				HostPort:      25555,
			},
		},
		"invalid protocol port": {
			containerPort: game_room.ContainerPort{
				Name:     "testsctp",
				Protocol: "sctp",
				Port:     5555,
				HostPort: 25555,
			},
			withError: true,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := convertContainerPort(test.containerPort)
			if test.withError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expectedKubernetes.Name, res.Name)
			require.Equal(t, test.expectedKubernetes.Protocol, res.Protocol)
			require.Equal(t, test.expectedKubernetes.ContainerPort, res.ContainerPort)
			require.Equal(t, test.expectedKubernetes.HostPort, res.HostPort)
		})
	}
}

func TestConvertContainerEnvironment(t *testing.T) {
	cases := map[string]struct {
		containerEnvironment game_room.ContainerEnvironment
		expectedKubernetes   v1.EnvVar
	}{
		"name value environment": {
			containerEnvironment: game_room.ContainerEnvironment{
				Name:  "SAMPLE",
				Value: "value",
			},
			expectedKubernetes: v1.EnvVar{
				Name:  "SAMPLE",
				Value: "value",
			},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			res := convertContainerEnvironment(test.containerEnvironment)
			require.Equal(t, test.expectedKubernetes.Name, res.Name)
			require.Equal(t, test.expectedKubernetes.Value, res.Value)
		})
	}
}

func TestConvertSpecTolerations(t *testing.T) {
	cases := map[string]struct {
		spec               game_room.Spec
		expectedKubernetes v1.Toleration
		empty              bool
	}{
		"with toleration": {
			spec: game_room.Spec{Toleration: "maestro-sample"},
			expectedKubernetes: v1.Toleration{
				Key:      tolerationKey,
				Operator: tolerationOperator,
				Effect:   tolerationEffect,
				Value:    "maestro-sample",
			},
		},
		"with no tolerations": {
			spec:  game_room.Spec{},
			empty: true,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			res := convertSpecTolerations(test.spec)
			if test.empty {
				require.Empty(t, res)
				return
			}

			require.Len(t, res, 1)
			require.Equal(t, test.expectedKubernetes.Key, res[0].Key)
			require.Equal(t, test.expectedKubernetes.Operator, res[0].Operator)
			require.Equal(t, test.expectedKubernetes.Effect, res[0].Effect)
			require.Equal(t, test.expectedKubernetes.Value, res[0].Value)
		})
	}
}

func TestConvertSpecAffinity(t *testing.T) {
	cases := map[string]struct {
		spec             game_room.Spec
		expectedSelector v1.NodeSelectorRequirement
		empty            bool
	}{
		"with affinity": {
			spec: game_room.Spec{Affinity: "maestro-sample"},
			expectedSelector: v1.NodeSelectorRequirement{
				Key:      "maestro-sample",
				Operator: affinityOperator,
				Values:   []string{affinityValue},
			},
		},
		"with no affinity": {
			spec:  game_room.Spec{},
			empty: true,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			res := convertSpecAffinity(test.spec)
			if test.empty {
				require.Empty(t, res)
				return
			}

			require.NotNil(t, res)
			require.Equal(t, test.expectedSelector.Key, res.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key)
			require.Equal(t, test.expectedSelector.Operator, res.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Operator)
			require.Equal(t, test.expectedSelector.Values, res.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values)
		})
	}
}

func TestConvertTerminationGracePeriod(t *testing.T) {
	cases := map[string]struct {
		spec                  game_room.Spec
		expectedPeriodSeconds int64
		empty                 bool
	}{
		"with duration": {
			spec:                  game_room.Spec{TerminationGracePeriod: 10 * time.Second},
			expectedPeriodSeconds: 10,
		},
		"with 0 duration": {
			spec:  game_room.Spec{TerminationGracePeriod: 0},
			empty: true,
		},
		"without duration": {
			spec:  game_room.Spec{},
			empty: true,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			res := convertTerminationGracePeriod(test.spec)
			if test.empty {
				require.Nil(t, res)
				return
			}

			require.NotNil(t, res)
			require.Equal(t, test.expectedPeriodSeconds, *res)
		})
	}
}

func TestConvertContainer(t *testing.T) {
	cases := map[string]struct {
		container         game_room.Container
		expectedContainer v1.Container
		withError         bool
	}{
		"with empty container": {
			container: game_room.Container{},
			expectedContainer: v1.Container{
				Env: []v1.EnvVar{
					{Name: "MAESTRO_SCHEDULER_NAME", Value: "scheduler-name"},
					{Name: "MAESTRO_ROOM_ID", Value: "scheduler-name-1234"},
				},
			},
		},
		"with simple container": {
			container: game_room.Container{Name: "simple", Image: "image"},
			expectedContainer: v1.Container{
				Name:  "simple",
				Image: "image",
				Env: []v1.EnvVar{
					{Name: "MAESTRO_SCHEDULER_NAME", Value: "scheduler-name"},
					{Name: "MAESTRO_ROOM_ID", Value: "scheduler-name-1234"},
				},
			},
		},
		"with options container": {
			container: game_room.Container{
				Name:    "complete",
				Image:   "image",
				Command: []string{"some", "command"},
				Environment: []game_room.ContainerEnvironment{
					{Name: "env", Value: "value"},
					{Name: "envFieldValue", ValueFrom: &game_room.ValueFrom{FieldRef: &game_room.FieldRef{FieldPath: "status.podIP"}}},
					{Name: "envSecretValue", ValueFrom: &game_room.ValueFrom{SecretKeyRef: &game_room.SecretKeyRef{Name: "namespace-secret", Key: "database_password"}}},
				},
				Ports: []game_room.ContainerPort{{Port: 2222, Protocol: "tcp"}},
			},
			expectedContainer: v1.Container{
				Name:    "complete",
				Image:   "image",
				Command: []string{"some", "command"},
				Env: []v1.EnvVar{
					{Name: "env", Value: "value"},
					{Name: "envFieldValue", ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "status.podIP"}}},
					{Name: "envSecretValue", ValueFrom: &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "namespace-secret"}, Key: "database_password"}}},
					{Name: "MAESTRO_SCHEDULER_NAME", Value: "scheduler-name"},
					{Name: "MAESTRO_ROOM_ID", Value: "scheduler-name-1234"},
				},
				Ports: []v1.ContainerPort{{ContainerPort: 2222, Protocol: "tcp"}},
			},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := convertContainer(test.container, "scheduler-name", "scheduler-name-1234")
			if test.withError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expectedContainer.Name, res.Name)
			require.Equal(t, test.expectedContainer.Image, res.Image)
			require.Equal(t, test.expectedContainer.Image, res.Image)
			require.ElementsMatch(t, test.expectedContainer.Command, res.Command)
			require.Equal(t, len(test.expectedContainer.Ports), len(res.Ports))
			require.Equal(t, test.expectedContainer.Env, res.Env)
		})
	}
}

func TestConvertGameSpec(t *testing.T) {
	cases := map[string]struct {
		schedulerID string
		roomName    string
		gameSpec    game_room.Spec
		expectedPod v1.Pod
		withError   bool
	}{
		"without containers": {
			schedulerID: "sample",
			roomName:    "roomName",
			gameSpec: game_room.Spec{
				Version: "version",
			},
			expectedPod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "roomName",
					Namespace: "sample",
					Labels: map[string]string{
						maestroLabelKey:   maestroLabelValue,
						schedulerLabelKey: "sample",
						versionLabelKey:   "version",
					},
				},
			},
		},
		"with containers": {
			schedulerID: "sample",
			roomName:    "roomName",
			gameSpec: game_room.Spec{
				Version: "version",
				Containers: []game_room.Container{
					{},
					{},
				},
			},
			expectedPod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "roomName",
					Namespace: "sample",
					Labels: map[string]string{
						maestroLabelKey:   maestroLabelValue,
						schedulerLabelKey: "sample",
						versionLabelKey:   "version",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{},
						{},
					},
				},
			},
		},
		"with toleration": {
			schedulerID: "sample",
			roomName:    "roomName",
			gameSpec: game_room.Spec{
				Version:    "version",
				Toleration: "some-toleration",
			},
			expectedPod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "roomName",
					Namespace: "sample",
					Labels: map[string]string{
						maestroLabelKey:   maestroLabelValue,
						schedulerLabelKey: "sample",
						versionLabelKey:   "version",
					},
				},
				Spec: v1.PodSpec{
					Tolerations: []v1.Toleration{
						{},
					},
				},
			},
		},
		"with affinity": {
			schedulerID: "sample",
			roomName:    "roomName",
			gameSpec: game_room.Spec{
				Version:  "version",
				Affinity: "sample-affinity",
			},
			expectedPod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "roomName",
					Namespace: "sample",
					Labels: map[string]string{
						maestroLabelKey:   maestroLabelValue,
						schedulerLabelKey: "sample",
						versionLabelKey:   "version",
					},
				},
				Spec: v1.PodSpec{
					Affinity: &v1.Affinity{},
				},
			},
		},
		"with termination grace period": {
			schedulerID: "sample",
			roomName:    "roomName",
			gameSpec: game_room.Spec{
				Version:                "version",
				TerminationGracePeriod: 10 * time.Second,
			},
			expectedPod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "roomName",
					Namespace: "sample",
					Labels: map[string]string{
						maestroLabelKey:   maestroLabelValue,
						schedulerLabelKey: "sample",
						versionLabelKey:   "version",
					},
				},
				Spec: v1.PodSpec{
					TerminationGracePeriodSeconds: int64Pointer(10),
				},
			},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := convertGameRoomSpec(test.schedulerID, test.roomName, test.gameSpec)
			if test.withError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expectedPod.ObjectMeta.Labels, res.ObjectMeta.Labels)
			require.Equal(t, test.expectedPod.ObjectMeta.Name, res.ObjectMeta.Name)
			require.Equal(t, test.expectedPod.ObjectMeta.Namespace, res.ObjectMeta.Namespace)
			require.Equal(t, len(test.expectedPod.Spec.Containers), len(res.Spec.Containers))
			require.Equal(t, len(test.expectedPod.Spec.Tolerations), len(res.Spec.Tolerations))

			if test.expectedPod.Spec.Affinity != nil {
				require.NotNil(t, res.Spec.Affinity)
			} else {
				require.Nil(t, res.Spec.Affinity)
			}

			if test.expectedPod.Spec.TerminationGracePeriodSeconds != nil {
				require.NotNil(t, res.Spec.TerminationGracePeriodSeconds)
				require.Equal(t, test.expectedPod.Spec.TerminationGracePeriodSeconds, res.Spec.TerminationGracePeriodSeconds)
			} else {
				require.Nil(t, res.Spec.TerminationGracePeriodSeconds)
			}
		})
	}
}

func TestConvertPodStatus(t *testing.T) {
	cases := map[string]struct {
		pod            *v1.Pod
		expectedStatus game_room.InstanceStatus
	}{
		"ready": {
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{Type: v1.PodReady, Status: v1.ConditionTrue},
					},
				},
			},
			expectedStatus: game_room.InstanceStatus{
				Type:        game_room.InstanceReady,
				Description: "",
			},
		},
		"pending no conditions present": {
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodPending,
				},
			},
			expectedStatus: game_room.InstanceStatus{
				Type:        game_room.InstancePending,
				Description: "",
			},
		},
		"pending scheduled": {
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodPending,
					Conditions: []v1.PodCondition{
						{Type: v1.PodScheduled, Status: v1.ConditionTrue},
					},
				},
			},
			expectedStatus: game_room.InstanceStatus{
				Type:        game_room.InstancePending,
				Description: "",
			},
		},
		"pod in crashloop": {
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					Conditions: []v1.PodCondition{
						{Type: v1.PodReady, Status: v1.ConditionFalse},
						{Type: v1.PodScheduled, Status: v1.ConditionTrue},
						{Type: v1.ContainersReady, Status: v1.ConditionFalse},
						{Type: v1.ContainersReady, Status: v1.ConditionFalse},
					},
					ContainerStatuses: []v1.ContainerStatus{
						{State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff", Message: "retrying"}}},
					},
				},
			},
			expectedStatus: game_room.InstanceStatus{
				Type:        game_room.InstanceError,
				Description: "CrashLoopBackOff: retrying",
			},
		},
		"pod with container error": {
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					Conditions: []v1.PodCondition{
						{Type: v1.PodReady, Status: v1.ConditionFalse},
						{Type: v1.PodScheduled, Status: v1.ConditionTrue},
						{Type: v1.ContainersReady, Status: v1.ConditionFalse},
						{Type: v1.ContainersReady, Status: v1.ConditionFalse},
					},
					ContainerStatuses: []v1.ContainerStatus{
						{State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "RunContainerError", Message: "failed to find executable"}}},
					},
				},
			},
			expectedStatus: game_room.InstanceStatus{
				Type:        game_room.InstanceError,
				Description: "RunContainerError: failed to find executable",
			},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			res := convertPodStatus(test.pod)
			require.Equal(t, test.expectedStatus.Type, res.Type)
			require.Equal(t, test.expectedStatus.Description, res.Description)
		})
	}
}

func TestConvertPod(t *testing.T) {
	cases := map[string]struct {
		pod              *v1.Pod
		nodeAddress      string
		expectedInstance game_room.Instance
		expectedError    bool
	}{
		"id": {
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-id",
				},
			},
			expectedInstance: game_room.Instance{
				ID: "pod-id",
			},
		},
		"scheduler": {
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "some-scheduler",
				},
			},
			expectedInstance: game_room.Instance{
				SchedulerID: "some-scheduler",
			},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := convertPod(test.pod, test.nodeAddress)
			if test.expectedError {
				require.Error(t, err)
				return
			}

			require.NotNil(t, res)
			require.Equal(t, test.expectedInstance.ID, res.ID)
			require.Equal(t, test.expectedInstance.SchedulerID, res.SchedulerID)
		})
	}
}

func TestConvertNodeAddress(t *testing.T) {
	cases := map[string]struct {
		node            *v1.Node
		expectedAddress string
		expectedErr     bool
	}{
		"node nil": {
			node:            nil,
			expectedAddress: "",
		},
		"node with external ip": {
			node:            &v1.Node{Status: v1.NodeStatus{Addresses: []v1.NodeAddress{{Address: "0.0.0.0", Type: v1.NodeExternalIP}}}},
			expectedAddress: "0.0.0.0",
		},
		"node with internal and external ip": {
			node: &v1.Node{
				Status: v1.NodeStatus{
					Addresses: []v1.NodeAddress{
						{Address: "1.1.1.1", Type: v1.NodeInternalIP},
						{Address: "0.0.0.0", Type: v1.NodeExternalIP},
					},
				},
			},
			expectedAddress: "0.0.0.0",
			expectedErr:     false,
		},
		"node with external DNS": {
			node: &v1.Node{
				Status: v1.NodeStatus{
					Addresses: []v1.NodeAddress{
						{Address: "internal.maestro.com", Type: v1.NodeInternalDNS},
						{Address: "external.maestro.com", Type: v1.NodeExternalDNS},
					},
				},
			},
			expectedAddress: "external.maestro.com",
			expectedErr:     false,
		},
		"node with external IP and DNS": {
			node: &v1.Node{
				Status: v1.NodeStatus{
					Addresses: []v1.NodeAddress{
						{Address: "0.0.0.0", Type: v1.NodeExternalIP},
						{Address: "external.maestro.com", Type: v1.NodeExternalDNS},
					},
				},
			},
			expectedAddress: "external.maestro.com",
			expectedErr:     false,
		},
		"node without external addresses": {
			node: &v1.Node{
				Status: v1.NodeStatus{
					Addresses: []v1.NodeAddress{
						{Address: "0.0.0.0", Type: v1.NodeInternalIP},
						{Address: "internal.maestro.com", Type: v1.NodeInternalDNS},
					},
				},
			},
			expectedAddress: "internal.maestro.com",
		},
		"node with malformed external IP address": {
			node: &v1.Node{
				Status: v1.NodeStatus{
					Addresses: []v1.NodeAddress{
						{Address: "abc", Type: v1.NodeExternalIP},
					},
				},
			},
			expectedErr: true,
		},
		"node without addresses": {
			node: &v1.Node{
				Status: v1.NodeStatus{
					Addresses: []v1.NodeAddress{},
				},
			},
			expectedErr: true,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			addr, err := convertNodeAddress(test.node)
			if test.expectedErr {
				require.Error(t, err)
				return
			}

			require.Equal(t, test.expectedAddress, addr)
		})
	}
}

func TestConvertPodAddress(t *testing.T) {
	cases := map[string]struct {
		pod           *v1.Pod
		expectedPorts []game_room.Port
	}{
		"single container": {
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Ports: []v1.ContainerPort{
								{
									Name:     "tcp",
									HostPort: 8080,
									Protocol: v1.ProtocolTCP,
								},
								{
									Name:     "second-tcp",
									HostPort: 8081,
									Protocol: v1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
			expectedPorts: []game_room.Port{
				{
					Name:     "tcp",
					Protocol: "TCP",
					Port:     8080,
				},
				{
					Name:     "second-tcp",
					Protocol: "TCP",
					Port:     8081,
				},
			},
		},
		"multiple containers": {
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Ports: []v1.ContainerPort{
								{
									Name:     "tcp",
									HostPort: 8080,
									Protocol: v1.ProtocolTCP,
								},
								{
									Name:     "second-tcp",
									HostPort: 8081,
									Protocol: v1.ProtocolTCP,
								},
							},
						},
						{
							Ports: []v1.ContainerPort{
								{
									Name:     "tcp",
									HostPort: 8082,
									Protocol: v1.ProtocolTCP,
								},
								{
									Name:     "udp",
									HostPort: 8083,
									Protocol: v1.ProtocolUDP,
								},
							},
						},
					},
				},
			},
			expectedPorts: []game_room.Port{
				{
					Name:     "tcp",
					Protocol: "TCP",
					Port:     8080,
				},
				{
					Name:     "second-tcp",
					Protocol: "TCP",
					Port:     8081,
				},
				{
					Name:     "tcp",
					Protocol: "TCP",
					Port:     8082,
				},
				{
					Name:     "udp",
					Protocol: "UDP",
					Port:     8083,
				},
			},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			ports := convertPodPorts(test.pod)
			require.Equal(t, test.expectedPorts, ports)
		})
	}
}
