package kubernetes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/topfreegames/maestro/internal/entities"
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
		containerResources entities.GameRoomContainerResources
		expectedKubernetes v1.ResourceList
		withError          bool
	}{
		"only with memory": {
			containerResources: entities.GameRoomContainerResources{
				Memory: "100Mi",
			},
			expectedKubernetes: v1.ResourceList{
				v1.ResourceMemory: *resource.NewQuantity(100*1024*1024, resource.BinarySI),
			},
		},
		"only with CPU": {
			containerResources: entities.GameRoomContainerResources{
				CPU: "100m",
			},
			expectedKubernetes: v1.ResourceList{
				v1.ResourceCPU: *resource.NewMilliQuantity(100, resource.DecimalSI),
			},
		},
		"with memory and CPU": {
			containerResources: entities.GameRoomContainerResources{
				Memory: "100Mi",
				CPU:    "100m",
			},
			expectedKubernetes: v1.ResourceList{
				v1.ResourceMemory: *resource.NewQuantity(100*1024*1024, resource.BinarySI),
				v1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
			},
		},
		"with invalid memory": {
			containerResources: entities.GameRoomContainerResources{
				Memory: "100abc",
			},
			withError: true,
		},
		"with invalid CPU": {
			containerResources: entities.GameRoomContainerResources{
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
		containerPort      entities.GameRoomContainerPort
		expectedKubernetes v1.ContainerPort
		withError          bool
	}{
		"tcp port": {
			containerPort: entities.GameRoomContainerPort{
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
			containerPort: entities.GameRoomContainerPort{
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
			containerPort: entities.GameRoomContainerPort{
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
		containerEnvironment entities.GameRoomContainerEnvironment
		expectedKubernetes   v1.EnvVar
	}{
		"name value environment": {
			containerEnvironment: entities.GameRoomContainerEnvironment{
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
		spec               entities.GameRoomSpec
		expectedKubernetes v1.Toleration
		empty              bool
	}{
		"with toleration": {
			spec: entities.GameRoomSpec{Toleration: "maestro-sample"},
			expectedKubernetes: v1.Toleration{
				Key:      tolerationKey,
				Operator: tolerationOperator,
				Effect:   tolerationEffect,
				Value:    "maestro-sample",
			},
		},
		"with no tolerations": {
			spec:  entities.GameRoomSpec{},
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
		spec             entities.GameRoomSpec
		expectedSelector v1.NodeSelectorRequirement
		empty            bool
	}{
		"with affinity": {
			spec: entities.GameRoomSpec{Affinity: "maestro-sample"},
			expectedSelector: v1.NodeSelectorRequirement{
				Key:      "maestro-sample",
				Operator: affinityOperator,
				Values:   []string{affinityValue},
			},
		},
		"with no affinity": {
			spec:  entities.GameRoomSpec{},
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
		spec                  entities.GameRoomSpec
		expectedPeriodSeconds int64
		empty                 bool
	}{
		"with duration": {
			spec:                  entities.GameRoomSpec{TerminationGracePeriod: 10 * time.Second},
			expectedPeriodSeconds: 10,
		},
		"with 0 duration": {
			spec:  entities.GameRoomSpec{TerminationGracePeriod: 0},
			empty: true,
		},
		"without duration": {
			spec:  entities.GameRoomSpec{},
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
		container         entities.GameRoomContainer
		expectedContainer v1.Container
		withError         bool
	}{
		"with empty container": {
			container:         entities.GameRoomContainer{},
			expectedContainer: v1.Container{},
		},
		"with simple container": {
			container:         entities.GameRoomContainer{Name: "simple", Image: "image"},
			expectedContainer: v1.Container{Name: "simple", Image: "image"},
		},
		"with options container": {
			container: entities.GameRoomContainer{
				Name:        "complete",
				Image:       "image",
				Command:     []string{"some", "command"},
				Environment: []entities.GameRoomContainerEnvironment{{Name: "env", Value: "value"}},
				Ports:       []entities.GameRoomContainerPort{{Port: 2222, Protocol: "tcp"}},
			},
			expectedContainer: v1.Container{
				Name:    "complete",
				Image:   "image",
				Command: []string{"some", "command"},
				Env:     []v1.EnvVar{{Name: "env", Value: "value"}},
				Ports:   []v1.ContainerPort{{ContainerPort: 2222, Protocol: "tcp"}},
			},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := convertContainer(test.container)
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
			require.Equal(t, len(test.expectedContainer.Env), len(res.Env))
		})
	}
}

func TestConvertGameSpec(t *testing.T) {
	cases := map[string]struct {
		gameRoom    *entities.GameRoom
		gameSpec    entities.GameRoomSpec
		expectedPod v1.Pod
		withError   bool
	}{
		"without containers": {
			gameRoom: &entities.GameRoom{
				Scheduler: entities.Scheduler{
					ID: "sample",
				},
			},
			gameSpec: entities.GameRoomSpec{
				Version: "version",
			},
			expectedPod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
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
			gameRoom: &entities.GameRoom{
				Scheduler: entities.Scheduler{
					ID: "sample",
				},
			},
			gameSpec: entities.GameRoomSpec{
				Version: "version",
				Containers: []entities.GameRoomContainer{
					entities.GameRoomContainer{},
					entities.GameRoomContainer{},
				},
			},
			expectedPod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "sample",
					Labels: map[string]string{
						maestroLabelKey:   maestroLabelValue,
						schedulerLabelKey: "sample",
						versionLabelKey:   "version",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						v1.Container{},
						v1.Container{},
					},
				},
			},
		},
		"with toleration": {
			gameRoom: &entities.GameRoom{
				Scheduler: entities.Scheduler{
					ID: "sample",
				},
			},
			gameSpec: entities.GameRoomSpec{
				Version:    "version",
				Toleration: "some-toleration",
			},
			expectedPod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "sample",
					Labels: map[string]string{
						maestroLabelKey:   maestroLabelValue,
						schedulerLabelKey: "sample",
						versionLabelKey:   "version",
					},
				},
				Spec: v1.PodSpec{
					Tolerations: []v1.Toleration{
						v1.Toleration{},
					},
				},
			},
		},
		"with affinity": {
			gameRoom: &entities.GameRoom{
				Scheduler: entities.Scheduler{
					ID: "sample",
				},
			},
			gameSpec: entities.GameRoomSpec{
				Version:  "version",
				Affinity: "sample-affinity",
			},
			expectedPod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
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
			gameRoom: &entities.GameRoom{
				Scheduler: entities.Scheduler{
					ID: "sample",
				},
			},
			gameSpec: entities.GameRoomSpec{
				Version:                "version",
				TerminationGracePeriod: 10 * time.Second,
			},
			expectedPod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
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
			res, err := convertGameRoomSpec(test.gameRoom.Scheduler, test.gameSpec)
			if test.withError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expectedPod.ObjectMeta.Labels, res.ObjectMeta.Labels)
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
