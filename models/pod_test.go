// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"fmt"
	"strings"

	"github.com/topfreegames/maestro/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pod", func() {
	var (
		clientset               *fake.Clientset
		command                 []string
		env                     []*models.EnvVar
		game                    string
		image                   string
		name                    string
		namespace               string
		ports                   []*models.Port
		resourcesLimitsCPU      string
		resourcesLimitsMemory   string
		resourcesRequestsCPU    string
		resourcesRequestsMemory string
		shutdownTimeout         int
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
			},
			{
				ContainerPort: 8888,
			},
		}
		resourcesLimitsCPU = "2"
		resourcesLimitsMemory = "128974848"
		resourcesRequestsCPU = "1"
		resourcesRequestsMemory = "64487424"
		shutdownTimeout = 180
	})

	Describe("NewPod", func() {
		It("should build correct pod struct", func() {
			pod := models.NewPod(
				game,
				image,
				name,
				namespace,
				resourcesLimitsCPU,
				resourcesLimitsMemory,
				resourcesRequestsCPU,
				resourcesRequestsMemory,
				shutdownTimeout,
				ports,
				command,
				env,
			)
			Expect(pod.Game).To(Equal(game))
			Expect(pod.Image).To(Equal(image))
			Expect(pod.Name).To(Equal(name))
			Expect(pod.Namespace).To(Equal(namespace))
			Expect(pod.ResourcesLimitsCPU).To(Equal(resourcesLimitsCPU))
			Expect(pod.ResourcesLimitsMemory).To(Equal(resourcesLimitsMemory))
			Expect(pod.ResourcesRequestsCPU).To(Equal(resourcesRequestsCPU))
			Expect(pod.ResourcesRequestsMemory).To(Equal(resourcesRequestsMemory))
			Expect(pod.ShutdownTimeout).To(Equal(shutdownTimeout))
			Expect(pod.Ports).To(Equal(ports))
			Expect(pod.Command).To(Equal(command))
			Expect(pod.Env).To(Equal(env))
		})
	})

	Describe("Create", func() {
		It("should create a pod in kubernetes", func() {
			pod := models.NewPod(
				game,
				image,
				name,
				namespace,
				resourcesLimitsCPU,
				resourcesLimitsMemory,
				resourcesRequestsCPU,
				resourcesRequestsMemory,
				shutdownTimeout,
				ports,
				command,
				env,
			)
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
			Expect(podv1.Spec.Tolerations[0].Key).To(Equal("game"))
			Expect(podv1.Spec.Tolerations[0].Operator).To(Equal(v1.TolerationOpEqual))
			Expect(podv1.Spec.Tolerations[0].Value).To(Equal(game))
			Expect(podv1.Spec.Tolerations[0].Effect).To(Equal(v1.TaintEffectNoSchedule))

			Expect(podv1.Spec.Containers).To(HaveLen(1))
			Expect(podv1.Spec.Containers[0].Name).To(Equal(name))
			Expect(podv1.Spec.Containers[0].Image).To(Equal(image))
			Expect(podv1.Spec.Containers[0].Ports).To(HaveLen(len(ports)))
			for idx, port := range podv1.Spec.Containers[0].Ports {
				Expect(port.ContainerPort).To(BeEquivalentTo(ports[idx].ContainerPort))
			}
			quantity := podv1.Spec.Containers[0].Resources.Limits["memory"]
			Expect((&quantity).String()).To(Equal(resourcesLimitsMemory))
			quantity = podv1.Spec.Containers[0].Resources.Limits["cpu"]
			Expect((&quantity).String()).To(Equal(resourcesLimitsCPU))
			quantity = podv1.Spec.Containers[0].Resources.Requests["memory"]
			Expect((&quantity).String()).To(Equal(resourcesRequestsMemory))
			quantity = podv1.Spec.Containers[0].Resources.Requests["cpu"]
			Expect((&quantity).String()).To(Equal(resourcesRequestsCPU))
			Expect(podv1.Spec.Containers[0].Env).To(HaveLen(len(env)))
			for idx, envVar := range podv1.Spec.Containers[0].Env {
				Expect(envVar.Name).To(Equal(env[idx].Name))
				Expect(envVar.Value).To(Equal(env[idx].Value))
			}
			Expect(podv1.Spec.Containers[0].Command).To(HaveLen(1))
			Expect(podv1.Spec.Containers[0].Command[0]).To(Equal(strings.Join(command, " ")))
		})

		It("should return error when creating existing pod", func() {
			pod := models.NewPod(
				game,
				image,
				name,
				namespace,
				resourcesLimitsCPU,
				resourcesLimitsMemory,
				resourcesRequestsCPU,
				resourcesRequestsMemory,
				shutdownTimeout,
				ports,
				command,
				env,
			)
			_, err := pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			_, err = pod.Create(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf("Pod \"%s\" already exists", name)))
		})
	})

	Describe("Delete", func() {
		It("should delete a pod from kubernetes", func() {
			pod := models.NewPod(
				game,
				image,
				name,
				namespace,
				resourcesLimitsCPU,
				resourcesLimitsMemory,
				resourcesRequestsCPU,
				resourcesRequestsMemory,
				shutdownTimeout,
				ports,
				command,
				env,
			)
			_, err := pod.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = pod.Delete(clientset)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error when deleting non existent pod", func() {
			pod := models.NewPod(
				game,
				image,
				name,
				namespace,
				resourcesLimitsCPU,
				resourcesLimitsMemory,
				resourcesRequestsCPU,
				resourcesRequestsMemory,
				shutdownTimeout,
				ports,
				command,
				env,
			)
			err := pod.Delete(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Pod \"pong-free-for-all-0\" not found"))
		})
	})
})
