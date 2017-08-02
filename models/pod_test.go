// maestro
// +build unit
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"fmt"

	goredis "github.com/go-redis/redis"
	"github.com/topfreegames/maestro/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pod", func() {
	var (
		clientset       *fake.Clientset
		command         []string
		env             []*models.EnvVar
		game            string
		image           string
		name            string
		namespace       string
		ports           []*models.Port
		requests        *models.Resources
		limits          *models.Resources
		shutdownTimeout int
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
		command = []string{
			"./room-binary",
			"-serverType",
			"6a8e136b-2dc1-417e-bbe8-0f0a2d2df431",
		}
		env = []*models.EnvVar{
			{
				Name:  "EXAMPLE_ENV_VAR",
				Value: "examplevalue",
			},
			{
				Name:  "ANOTHER_ENV_VAR",
				Value: "anothervalue",
			},
		}
		game = "pong"
		image = "pong/pong:v123"
		name = "pong-free-for-all-0"
		namespace = "pong-free-for-all"
		ports = []*models.Port{
			{
				ContainerPort: 5050,
				HostPort:      5000,
			},
			{
				ContainerPort: 8888,
				HostPort:      5000,
			},
		}
		limits = &models.Resources{
			CPU:    "2",
			Memory: "128974848",
		}
		requests = &models.Resources{
			CPU:    "1",
			Memory: "64487424",
		}
		shutdownTimeout = 180

		mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
		mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
			Return(goredis.NewStringResult("5000", nil))
		mockPipeline.EXPECT().SPop(models.FreePortsRedisKey()).
			Return(goredis.NewStringResult("5001", nil))
		mockPipeline.EXPECT().Exec()
	})

	Describe("NewPod", func() {
		It("should build correct pod struct", func() {
			pod, err := models.NewPod(
				game,
				image,
				name,
				namespace,
				limits,
				requests,
				shutdownTimeout,
				ports,
				command,
				env,
				mockClientset,
				mockRedisClient,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Game).To(Equal(game))
			Expect(pod.Image).To(Equal(image))
			Expect(pod.Name).To(Equal(name))
			Expect(pod.Namespace).To(Equal(namespace))
			Expect(pod.ResourcesLimitsCPU).To(Equal(limits.CPU))
			Expect(pod.ResourcesLimitsMemory).To(Equal(limits.Memory))
			Expect(pod.ResourcesRequestsCPU).To(Equal(requests.CPU))
			Expect(pod.ResourcesRequestsMemory).To(Equal(requests.Memory))
			Expect(pod.ShutdownTimeout).To(Equal(shutdownTimeout))
			Expect(pod.Ports).To(Equal(ports))
			Expect(pod.Command).To(Equal(command))
			Expect(pod.Env).To(Equal(env))
		})
	})

	Describe("Create", func() {
		It("should create a pod in kubernetes", func() {
			pod, err := models.NewPod(
				game,
				image,
				name,
				namespace,
				limits,
				requests,
				shutdownTimeout,
				ports,
				command,
				env,
				mockClientset,
				mockRedisClient,
			)
			Expect(err).NotTo(HaveOccurred())
			pod.SetToleration(game)
			podv1, err := pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(podv1.GetNamespace()).To(Equal(namespace))
			pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(1))
			Expect(pods.Items[0].GetName()).To(Equal(name))
			Expect(podv1.ObjectMeta.Name).To(Equal(name))
			Expect(podv1.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(podv1.ObjectMeta.Labels).To(HaveLen(1))
			Expect(podv1.ObjectMeta.Labels["app"]).To(Equal(name))
			Expect(*podv1.Spec.TerminationGracePeriodSeconds).To(BeEquivalentTo(shutdownTimeout))
			Expect(podv1.Spec.Tolerations).To(HaveLen(1))
			Expect(podv1.Spec.Tolerations[0].Key).To(Equal("dedicated"))
			Expect(podv1.Spec.Tolerations[0].Operator).To(Equal(v1.TolerationOpEqual))
			Expect(podv1.Spec.Tolerations[0].Value).To(Equal(game))
			Expect(podv1.Spec.Tolerations[0].Effect).To(Equal(v1.TaintEffectNoSchedule))

			Expect(podv1.Spec.Containers).To(HaveLen(1))
			Expect(podv1.Spec.Containers[0].Name).To(Equal(name))
			Expect(podv1.Spec.Containers[0].Image).To(Equal(image))
			Expect(podv1.Spec.Containers[0].Ports).To(HaveLen(len(ports)))
			for idx, port := range podv1.Spec.Containers[0].Ports {
				Expect(port.ContainerPort).To(BeEquivalentTo(ports[idx].ContainerPort))
				Expect(port.HostPort).To(BeEquivalentTo(ports[idx].HostPort))
			}
			quantity := podv1.Spec.Containers[0].Resources.Limits["memory"]
			Expect((&quantity).String()).To(Equal(limits.Memory))
			quantity = podv1.Spec.Containers[0].Resources.Limits["cpu"]
			Expect((&quantity).String()).To(Equal(limits.CPU))
			quantity = podv1.Spec.Containers[0].Resources.Requests["memory"]
			Expect((&quantity).String()).To(Equal(requests.Memory))
			quantity = podv1.Spec.Containers[0].Resources.Requests["cpu"]
			Expect((&quantity).String()).To(Equal(requests.CPU))
			Expect(podv1.Spec.Containers[0].Env).To(HaveLen(len(env)))
			for idx, envVar := range podv1.Spec.Containers[0].Env {
				Expect(envVar.Name).To(Equal(env[idx].Name))
				Expect(envVar.Value).To(Equal(env[idx].Value))
			}
			Expect(podv1.Spec.Containers[0].Command).To(HaveLen(3))
			Expect(podv1.Spec.Containers[0].Command).To(Equal(command))
		})

		It("should create pod without requests and limits", func() {
			pod, err := models.NewPod(
				game,
				image,
				name,
				namespace,
				nil,
				nil,
				shutdownTimeout,
				ports,
				command,
				env,
				mockClientset,
				mockRedisClient,
			)
			Expect(err).NotTo(HaveOccurred())
			podv1, err := pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(podv1.GetNamespace()).To(Equal(namespace))
			pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(1))
			Expect(pods.Items[0].GetName()).To(Equal(name))
			Expect(podv1.ObjectMeta.Name).To(Equal(name))
			Expect(podv1.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(podv1.ObjectMeta.Labels).To(HaveLen(1))
			Expect(podv1.ObjectMeta.Labels["app"]).To(Equal(name))
			Expect(*podv1.Spec.TerminationGracePeriodSeconds).To(BeEquivalentTo(shutdownTimeout))

			Expect(podv1.Spec.Containers).To(HaveLen(1))
			Expect(podv1.Spec.Containers[0].Name).To(Equal(name))
			Expect(podv1.Spec.Containers[0].Image).To(Equal(image))
			Expect(podv1.Spec.Containers[0].Ports).To(HaveLen(len(ports)))
			for idx, port := range podv1.Spec.Containers[0].Ports {
				Expect(port.ContainerPort).To(BeEquivalentTo(ports[idx].ContainerPort))
			}
			quantity := podv1.Spec.Containers[0].Resources.Limits["memory"]
			Expect((&quantity).String()).To(Equal("0"))
			quantity = podv1.Spec.Containers[0].Resources.Limits["cpu"]
			Expect((&quantity).String()).To(Equal("0"))
			quantity = podv1.Spec.Containers[0].Resources.Requests["memory"]
			Expect((&quantity).String()).To(Equal("0"))
			quantity = podv1.Spec.Containers[0].Resources.Requests["cpu"]
			Expect((&quantity).String()).To(Equal("0"))
			Expect(podv1.Spec.Containers[0].Env).To(HaveLen(len(env)))
			for idx, envVar := range podv1.Spec.Containers[0].Env {
				Expect(envVar.Name).To(Equal(env[idx].Name))
				Expect(envVar.Value).To(Equal(env[idx].Value))
			}
			Expect(podv1.Spec.Containers[0].Command).To(HaveLen(3))
			Expect(podv1.Spec.Containers[0].Command).To(Equal(command))
		})

		It("should create pod with node affinity", func() {
			pod, err := models.NewPod(
				game,
				image,
				name,
				namespace,
				limits,
				requests,
				shutdownTimeout,
				ports,
				command,
				env,
				mockClientset,
				mockRedisClient,
			)
			Expect(err).NotTo(HaveOccurred())
			pod.SetAffinity(game)
			podv1, err := pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(podv1.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal(game))
			Expect(podv1.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Operator).To(Equal(v1.NodeSelectorOpIn))
			Expect(podv1.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values).To(ConsistOf("true"))
		})

		It("should return error when creating existing pod", func() {
			pod, err := models.NewPod(
				game,
				image,
				name,
				namespace,
				limits,
				requests,
				shutdownTimeout,
				ports,
				command,
				env,
				mockClientset,
				mockRedisClient,
			)
			Expect(err).NotTo(HaveOccurred())
			_, err = pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			_, err = pod.Create(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf("Pod \"%s\" already exists", name)))
		})
	})

	Describe("Delete", func() {
		It("should delete a pod from kubernetes", func() {
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), 5000)
			mockPipeline.EXPECT().SAdd(models.FreePortsRedisKey(), 5001)
			mockPipeline.EXPECT().Exec()

			pod, err := models.NewPod(
				game,
				image,
				name,
				namespace,
				limits,
				requests,
				shutdownTimeout,
				ports,
				command,
				env,
				mockClientset,
				mockRedisClient,
			)
			Expect(err).NotTo(HaveOccurred())
			_, err = pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = pod.Delete(clientset, mockRedisClient)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error when deleting non existent pod", func() {
			pod, err := models.NewPod(
				game,
				image,
				name,
				namespace,
				limits,
				requests,
				shutdownTimeout,
				ports,
				command,
				env,
				mockClientset,
				mockRedisClient,
			)
			Expect(err).NotTo(HaveOccurred())
			err = pod.Delete(clientset, mockRedisClient)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Pod \"pong-free-for-all-0\" not found"))
		})
	})
})
